// Package secret encrypts small values (API keys, MCP auth headers) at rest
// with AES-256-GCM. The key is derived from KNITNOX_SECRET or a generated key file.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
)

type Box struct{ aead cipher.AEAD }

// New derives a 256-bit key from any passphrase.
func New(passphrase string) (*Box, error) {
	if passphrase == "" {
		return nil, errors.New("secret: empty passphrase")
	}
	key := sha256.Sum256([]byte(passphrase))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// LoadOrCreateKeyFile reads a passphrase from path, generating a random one if
// the file does not exist. Used when KNITNOX_SECRET is not set (development).
func LoadOrCreateKeyFile(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(b)), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	k := hex.EncodeToString(raw)
	if err := os.WriteFile(path, []byte(k+"\n"), 0o600); err != nil {
		return "", err
	}
	return k, nil
}

// Seal encrypts plaintext. Empty input yields nil (nothing stored).
func (b *Box) Seal(plain string) ([]byte, error) {
	if plain == "" {
		return nil, nil
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, []byte(plain), nil), nil
}

// Open decrypts a value produced by Seal. nil input yields "".
func (b *Box) Open(sealed []byte) (string, error) {
	if len(sealed) == 0 {
		return "", nil
	}
	ns := b.aead.NonceSize()
	if len(sealed) < ns {
		return "", errors.New("secret: ciphertext too short")
	}
	plain, err := b.aead.Open(nil, sealed[:ns], sealed[ns:], nil)
	if err != nil {
		return "", errors.New("secret: cannot decrypt (was KNITNOX_SECRET changed?)")
	}
	return string(plain), nil
}

// Mask renders a key for display: "sk-…a3f9".
func Mask(key string) string {
	if key == "" {
		return "(none)"
	}
	if len(key) <= 8 {
		return "••••"
	}
	return key[:3] + "…" + key[len(key)-4:]
}
