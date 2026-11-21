package outbound

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	C "github.com/metacubex/mihomo/constant"
)

func TestMaccSaltSources(t *testing.T) {
	previousHome, previousEmbedded := C.Path.HomeDir(), embeddedMaccSalt
	C.SetHomeDir(t.TempDir())
	t.Cleanup(func() {
		C.SetHomeDir(previousHome)
		embeddedMaccSalt = previousEmbedded
	})
	t.Setenv("MIHOMO_MACC_SALT", "")
	embeddedMaccSalt = "built-in-test\n"
	check := func(want string) {
		t.Helper()
		got, err := maccSalt()
		if err != nil || got != want {
			t.Fatalf("maccSalt() = %q, %v; want %q", got, err, want)
		}
	}
	check("built-in-test")
	if err := os.WriteFile(filepath.Join(C.Path.HomeDir(), "macc.salt"), []byte("file-test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	check("file-test")
	t.Setenv("MIHOMO_MACC_SALT", "env-test")
	check("env-test")
}

func TestEmbeddedMaccMatchesReleaseSecret(t *testing.T) {
	want, ok := os.LookupEnv("MACC_RELEASE_SALT_CHECK")
	if !ok {
		t.Skip("release secret check is not configured")
	}
	if want == "" || strings.TrimRight(embeddedMaccSalt, "\r\n") != want {
		t.Fatal("embedded MACC material does not match the release input")
	}
}

func TestMaccUsesEmbeddedSalt(t *testing.T) {
	previousHome, previousEmbedded := C.Path.HomeDir(), embeddedMaccSalt
	C.SetHomeDir(t.TempDir())
	t.Cleanup(func() {
		C.SetHomeDir(previousHome)
		embeddedMaccSalt = previousEmbedded
	})
	t.Setenv("MIHOMO_MACC_SALT", "")
	embeddedMaccSalt = "test-salt"
	proxy, err := NewMacc(MaccOption{ShadowSocksROption: ShadowSocksROption{
		Name: "test", Server: "test-9d9abd690137c79d97ca800bf0cb565b.relay.example.test",
		Password: "test-password", Cipher: "rc4-md5", Protocol: "auth_aes128_md5",
		ProtocolParam: "12345:test-key", Obfs: "http_simple",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if proxy.Addr() != "127.0.0.1:10808" {
		t.Fatal("embedded salt did not decode the expected endpoint")
	}
}
