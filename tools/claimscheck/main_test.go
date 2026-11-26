package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// REQ: NFR-10
func TestCheck(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "README.md", "Fast. <!-- C1 -->\nSafe. <!-- C2 -->\nNew. <!-- C9 -->\n")
	write(t, dir, "docs/claims.md", "| ID | Claim | Tests |\n|---|---|---|\n"+
		"| C1 | fast | `go:TestFast` |\n| C2 | safe | `scenario:auth/a \"quoted\" name` `scenario:auth/missing` |\n| C3 | old | |\n")
	write(t, dir, "x/x_test.go", "package x\nfunc TestFast(t *testing.T) {}\n")
	write(t, dir, "e2e/scenarios/a_test.go", `add("auth", "a \"quoted\" name", func(t *testing.T) {})`)
	got, err := check(dir)
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{`scenario "auth/missing"`, "C3 has no test", "C9 has no row", "C3 isn't used"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in\n%s", want, all)
		}
	}
	if strings.Contains(all, "TestFast") || strings.Contains(all, "quoted") {
		t.Errorf("false positive:\n%s", all)
	}
}
