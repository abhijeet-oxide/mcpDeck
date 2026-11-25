package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ: CRED-2
func TestVault_KeyInKeychainNotOnDisk(t *testing.T) {
	dir := t.TempDir()
	kc := &FakeKeychain{}
	v, err := Open(kc, dir)
	if err != nil {
		t.Fatal(err)
	}
	if v.Backend != "os keychain" {
		t.Fatalf("backend %q", v.Backend)
	}
	if _, err := os.Stat(filepath.Join(dir, "mcpdeck.key")); err == nil {
		t.Fatal("key file written although a keychain exists")
	}
	a, _ := v.Seal("canary-123")
	b, _ := v.Seal("canary-123")
	if a == b || strings.Contains(a, "canary") {
		t.Fatal("ciphertexts must differ per nonce and not contain the value")
	}
	// Reopen with the same keychain decrypts; another keychain can't.
	v2, _ := Open(kc, dir)
	if got, err := v2.Unseal(a); err != nil || got != "canary-123" {
		t.Fatalf("unseal: %q %v", got, err)
	}
	v3, _ := Open(&FakeKeychain{}, t.TempDir())
	if _, err := v3.Unseal(a); err == nil {
		t.Fatal("other key must not decrypt")
	}
}

// REQ: CRED-2
func TestVault_FallsBackToKeyFile(t *testing.T) {
	dir := t.TempDir()
	v, err := Open(NoKeychain{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if v.Backend != "key file" || v.KeyFile == "" {
		t.Fatalf("want key file backend, got %+v", v)
	}
	s, _ := v.Seal("x")
	v2, _ := Open(NoKeychain{}, dir)
	if got, _ := v2.Unseal(s); got != "x" {
		t.Fatal("key file not reused")
	}
}
