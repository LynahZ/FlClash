package core

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/metacubex/mihomo/adapter"
	N "github.com/metacubex/mihomo/common/net"
	"github.com/metacubex/mihomo/constant"
	ss "github.com/metacubex/mihomo/transport/shadowsocks/core"
)

const maccHexServer = "test-9d9abd690137c79d97ca800bf0cb565b.relay.example.test"

func TestLoadMaccProfile(t *testing.T) {
	home := t.TempDir()
	previous := constant.Path.HomeDir()
	constant.SetHomeDir(home)
	t.Cleanup(func() { constant.SetHomeDir(previous) })
	t.Setenv("MIHOMO_MACC_SALT", "")
	if err := os.WriteFile(filepath.Join(home, "macc.salt"), []byte("test-salt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeBaseProfile(t, `proxies:
  - name: Hex
    type: macc
    server: test-9d9abd690137c79d97ca800bf0cb565b.relay.example.test
    port: 0
    cipher: rc4-md5
    password: test-password
    protocol: auth_aes128_md5
    protocol-param: 12345:test-key
    protocolparam: ignored-in-favor-of-canonical-key
    obfs: http_simple
    obfs-param: example.test
    udp: true
proxy-providers:
  inline:
    type: inline
    payload:
      - name: CBC
        type: macc
        server: test-relay.example.test-MDEyMzQ1Njc4OWFiY2RlZp7XBwB6JpvsfRpji1LT4Go=
        port: 20808
        cipher: rc4-md5
        password: test-password
        protocol: auth_aes128_md5
        protocolparam: 12345:test-key
        obfs: http_simple
        obfsparam: example.test
rules:
  - MATCH,Hex
`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	hex := cfg.Proxies["Hex"]
	if hex == nil || hex.Type() != constant.Macc || hex.Addr() != "127.0.0.1:10808" || !hex.SupportUDP() {
		t.Fatalf("wrong MACC adapter for profile node: %v", hex)
	}
	inline := cfg.Providers["inline"].Proxies()
	if len(inline) != 1 || inline[0].Type() != constant.Macc || inline[0].Addr() != "127.0.0.1:20808" {
		t.Fatalf("wrong MACC adapter for provider node: %v", inline)
	}
}

func TestMaccSaltAndMalformedAddress(t *testing.T) {
	previous := constant.Path.HomeDir()
	constant.SetHomeDir(t.TempDir())
	t.Cleanup(func() { constant.SetHomeDir(previous) })
	t.Setenv("MIHOMO_MACC_SALT", "")

	node := map[string]any{
		"name": "Hex", "type": "macc", "port": 0,
		"server":   "test-4671ada0ed9b79e613e2fb6b98786457.relay.example.test",
		"password": "test-password", "cipher": "rc4-md5",
		"obfs": "plain", "protocol": "origin", "protocol-param": "12345:test-key",
	}
	if _, err := adapter.ParseProxy(node); err == nil || !strings.Contains(err.Error(), "MIHOMO_MACC_SALT") {
		t.Fatalf("missing salt error = %v", err)
	}
	t.Setenv("MIHOMO_MACC_SALT", "test-salt")
	proxy, err := adapter.ParseProxy(node)
	if err != nil || proxy.Addr() != "127.0.0.1:10808" {
		t.Fatalf("MACC suffix variant: %v, %v", proxy, err)
	}
	node["server"] = "test-0001.relay.example.test"
	if _, err := adapter.ParseProxy(node); err == nil || strings.Contains(err.Error(), node["server"].(string)) {
		t.Fatalf("expected a redacted error for malformed ciphertext: %v", err)
	}
}

func TestMaccUDPRoundTrip(t *testing.T) {
	t.Setenv("MIHOMO_MACC_SALT", "test-salt")
	socket, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	deadline := time.Now().Add(5 * time.Second)
	if err := socket.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	cipher, err := ss.PickCipher("rc4-md5", nil, "test-password")
	if err != nil {
		t.Fatal(err)
	}
	server := cipher.PacketConn(N.NewEnhancePacketConn(socket))
	proxy, err := adapter.ParseProxy(map[string]any{
		"name": "UDP", "type": "macc", "server": maccHexServer,
		"port": socket.LocalAddr().(*net.UDPAddr).Port, "udp": true,
		"cipher": "rc4-md5", "password": "test-password", "obfs": "http_simple",
		"protocol": "auth_aes128_md5", "protocol-param": "12345:test-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	target := netip.MustParseAddrPort("192.0.2.7:53")
	targetAddr := net.UDPAddrFromAddrPort(target)
	client, err := proxy.ListenPacketContext(t.Context(), &constant.Metadata{
		NetWork: constant.UDP, DstIP: target.Addr(), DstPort: target.Port(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	request := []byte("query")
	if n, err := client.WriteTo(request, targetAddr); err != nil || n != len(request) {
		t.Fatalf("write through MACC = %d, %v", n, err)
	}
	buffer := make([]byte, 1500)
	n, clientAddr, err := server.ReadFrom(buffer)
	if err != nil {
		t.Fatal(err)
	}
	header := []byte{1, 192, 0, 2, 7, 0, 53}
	expected := binary.LittleEndian.AppendUint32(append(bytes.Clone(header), request...), 12345)
	userKey := md5.Sum([]byte("test-key"))
	tag := hmac.New(md5.New, userKey[:])
	tag.Write(expected)
	expected = append(expected, tag.Sum(nil)[:4]...)
	if !bytes.Equal(buffer[:n], expected) {
		t.Fatalf("unexpected SSR datagram: %x", buffer[:n])
	}
	reply := append(bytes.Clone(header), "reply"...)
	tag = hmac.New(md5.New, cipher.(*ss.StreamCipher).Key)
	tag.Write(reply)
	if _, err := server.WriteTo(append(reply, tag.Sum(nil)[:4]...), clientAddr); err != nil {
		t.Fatal(err)
	}
	n, source, err := client.ReadFrom(buffer)
	if err != nil || source.String() != targetAddr.String() || string(buffer[:n]) != "reply" {
		t.Fatalf("read through MACC = %d, %v, %v", n, source, err)
	}
}
