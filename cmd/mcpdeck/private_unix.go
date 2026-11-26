//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// privateFiles makes new files readable only by this user.
func privateFiles() { syscall.Umask(0o077) }

// chmodPrivate tightens files an older version created (SEC-4).
func chmodPrivate(dir string) {
	_ = os.Chmod(dir, 0o700)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		n := e.Name()
		if e.Type().IsRegular() && (strings.Contains(n, ".db") || strings.HasSuffix(n, ".key") || strings.HasSuffix(n, ".json")) {
			_ = os.Chmod(filepath.Join(dir, n), 0o600)
		}
	}
}
