// Package vault encrypts secrets at rest (CRED-2). Each value is sealed with
// AES-256-GCM and a random nonce. The data key lives in the OS keychain, never
// in SQLite; only when no keychain is available is a key file used.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

// Keychain stores the data key. OSKeychain is the real one; tests use a fake.
type Keychain interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
}

// OSKeychain uses Windows Credential Manager, macOS Keychain or Secret Service.
type OSKeychain struct{}

func (OSKeychain) Get(service, user string) (string, error) { return keyring.Get(service, user) }
func (OSKeychain) Set(service, user, s string) error        { return keyring.Set(service, user, s) }

// NoKeychain always fails, forcing the key file (used by temporary dev data).
type NoKeychain struct{}

func (NoKeychain) Get(string, string) (string, error) { return "", errors.New("no keychain") }
func (NoKeychain) Set(string, string, string) error   { return errors.New("no keychain") }

// FakeKeychain is an in-memory keychain for tests.
type FakeKeychain struct{ M map[string]string }

func (f *FakeKeychain) Get(s, u string) (string, error) {
	v, ok := f.M[s+"/"+u]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return v, nil
}
func (f *FakeKeychain) Set(s, u, v string) error {
	if f.M == nil {
		f.M = map[string]string{}
	}
	f.M[s+"/"+u] = v
	return nil
}

const service = "mcpDeck"

// Vault seals and opens secret values.
type Vault struct {
	aead cipher.AEAD
	// Backend says where the key is: "os keychain" or "key file".
	Backend string
	KeyFile string
}

// Open gets or creates the data key for the data folder dir.
func Open(kc Keychain, dir string) (*Vault, error) {
	user := "data-key:" + filepath.Clean(dir)
	keyFile := filepath.Join(dir, "mcpdeck.key")
	var key []byte
	backend := "os keychain"
	if enc, err := kc.Get(service, user); err == nil {
		key, err = base64.StdEncoding.DecodeString(enc)
		if err != nil {
			return nil, fmt.Errorf("data key in keychain is damaged: %w", err)
		}
	} else if b, ferr := os.ReadFile(keyFile); ferr == nil {
		key, err = base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
		if err != nil {
			return nil, fmt.Errorf("key file %s is damaged: %w", keyFile, err)
		}
		backend = "key file"
	} else {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		enc := base64.StdEncoding.EncodeToString(key)
		if err := kc.Set(service, user, enc); err != nil {
			backend = "key file"
			if err := os.WriteFile(keyFile, []byte(enc), 0o600); err != nil {
				return nil, fmt.Errorf("no keychain and can't write key file: %w", err)
			}
		}
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	v := &Vault{aead: aead, Backend: backend}
	if backend == "key file" {
		v.KeyFile = keyFile
	}
	return v, nil
}

// Seal encrypts plain with a fresh nonce.
func (v *Vault) Seal(plain string) (string, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := v.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Unseal decrypts a value from Seal. Only internal/proxy calls this.
func (v *Vault) Unseal(sealed string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil || len(b) < v.aead.NonceSize() {
		return "", errors.New("secret is damaged")
	}
	pt, err := v.aead.Open(nil, b[:v.aead.NonceSize()], b[v.aead.NonceSize():], nil)
	if err != nil {
		return "", errors.New("secret can't be decrypted with this data key")
	}
	return string(pt), nil
}
