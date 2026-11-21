package outbound

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	C "github.com/metacubex/mihomo/constant"
)

type MaccOption struct {
	ShadowSocksROption
	LegacyProtocolParam string `proxy:"protocolparam,omitempty"`
	LegacyObfsParam     string `proxy:"obfsparam,omitempty"`
}

//go:embed macc-default.salt
var embeddedMaccSalt string

func NewMacc(option MaccOption) (*ShadowSocksR, error) {
	if option.ProtocolParam == "" {
		option.ProtocolParam = option.LegacyProtocolParam
	}
	if option.ObfsParam == "" {
		option.ObfsParam = option.LegacyObfsParam
	}
	salt, err := maccSalt()
	if err != nil {
		return nil, err
	}
	host, port, err := parseMaccServer(option.Server, option.ProtocolParam, salt)
	if err != nil {
		return nil, err
	}
	option.Server = host
	if option.Port == 0 {
		option.Port = port
	}
	if option.Port < 1 || option.Port > 65535 {
		return nil, errors.New("invalid macc port")
	}
	return newShadowSocksR(option.ShadowSocksROption, C.Macc)
}

func maccSalt() (string, error) {
	salt := os.Getenv("MIHOMO_MACC_SALT")
	if salt == "" {
		data, err := os.ReadFile(filepath.Join(C.Path.HomeDir(), "macc.salt"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", errors.New("cannot read macc.salt from the core home")
		}
		salt = string(bytes.TrimRight(data, "\r\n"))
	}
	if salt == "" {
		salt = strings.TrimRight(embeddedMaccSalt, "\r\n")
	}
	if salt == "" {
		return "", errors.New("macc requires MIHOMO_MACC_SALT or macc.salt in the core home")
	}
	return salt, nil
}

func parseMaccServer(server, protocolParam, salt string) (string, int, error) {
	prefix, rest, _ := strings.Cut(server, "-")
	domain, payload, found := strings.Cut(rest, "-")
	if !found {
		payload, domain, _ = strings.Cut(rest, ".")
	}
	if prefix == "" || payload == "" || domain == "" || strings.HasPrefix(domain, ".") {
		return "", 0, errors.New("invalid macc server format")
	}

	if encrypted, err := hex.DecodeString(payload); err == nil {
		if _, suffix, ok := strings.Cut(domain, "."); ok && suffix != "" {
			if host, port, ok := decodeMaccEndpoint(encrypted, suffix, protocolParam, salt, false); ok {
				return host, port, nil
			}
		}
		if host, port, ok := decodeMaccEndpoint(encrypted, domain, protocolParam, salt, false); ok {
			return host, port, nil
		}
	} else if encrypted, err := base64.StdEncoding.DecodeString(payload); err == nil {
		if host, port, ok := decodeMaccEndpoint(encrypted, domain, protocolParam, salt, true); ok {
			return host, port, nil
		}
	}
	return "", 0, errors.New("cannot decode macc server with the configured salt and protocol parameter")
}

func decodeMaccEndpoint(encrypted []byte, domain, protocolParam, salt string, cbc bool) (string, int, bool) {
	if len(encrypted) == 0 || len(encrypted)%aes.BlockSize != 0 || (cbc && len(encrypted) == aes.BlockSize) {
		return "", 0, false
	}
	key := sha256.Sum256([]byte(domain + protocolParam + salt))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", 0, false
	}

	var plain []byte
	if cbc {
		iv, ciphertext := encrypted[:aes.BlockSize], encrypted[aes.BlockSize:]
		plain = make([]byte, len(ciphertext))
		cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ciphertext)
	} else {
		// The hex format predates the IV-prefixed CBC format.
		plain = make([]byte, len(encrypted))
		for offset := 0; offset < len(encrypted); offset += aes.BlockSize {
			block.Decrypt(plain[offset:], encrypted[offset:])
		}
	}

	padding := int(plain[len(plain)-1])
	if padding < 1 || padding > aes.BlockSize || bytes.Count(plain[len(plain)-padding:], []byte{byte(padding)}) != padding {
		return "", 0, false
	}
	host, portText, err := net.SplitHostPort(string(plain[:len(plain)-padding]))
	if err != nil || host == "" || strings.ContainsAny(host, " \t\r\n\x00/") {
		return "", 0, false
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || port == 0 {
		return "", 0, false
	}
	return host, int(port), true
}
