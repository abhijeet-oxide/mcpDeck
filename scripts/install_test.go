package scripts

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const fakeVersion = "mcpdeck v9.9.9-test"

// REQ: NFR-7
func TestInstaller(t *testing.T) {
	tests := []struct {
		name    string
		corrupt bool
		wantErr string
	}{
		{name: "installs a verified binary"},
		{name: "checksum mismatch installs nothing", corrupt: true, wantErr: "checksum mismatch"},
	}
	bin := buildFake(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release := t.TempDir()
			archive := writeArchive(t, release, bin)
			sum := sha(t, filepath.Join(release, archive))
			if tt.corrupt {
				sum = strings.Repeat("0", 64)
			}
			writeFile(t, filepath.Join(release, "checksums.txt"), fmt.Sprintf("%s  %s\n%s  other.zip\n", sum, archive, strings.Repeat("1", 64)))
			srv := httptest.NewServer(http.FileServer(http.Dir(release)))
			defer srv.Close()

			dest := filepath.Join(t.TempDir(), "bin")
			out, err := runInstaller(t, srv.URL, dest)
			exe := filepath.Join(dest, exeName())

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(out, tt.wantErr) {
					t.Fatalf("want failure with %q, got err=%v\n%s", tt.wantErr, err, out)
				}
				if _, statErr := os.Stat(exe); statErr == nil {
					t.Fatalf("%s was installed despite the failure", exe)
				}
				return
			}
			if err != nil {
				t.Fatalf("installer failed: %v\n%s", err, out)
			}
			if !strings.Contains(out, fakeVersion) {
				t.Errorf("installer output does not show the installed version:\n%s", out)
			}
			got, err := exec.Command(exe, "version").Output()
			if err != nil || strings.TrimSpace(string(got)) != fakeVersion {
				t.Fatalf("installed binary: %q, %v", got, err)
			}
		})
	}
}

func runInstaller(t *testing.T, url, dest string) (string, error) {
	t.Helper()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		ps, err := exec.LookPath("powershell")
		if err != nil {
			t.Skip("powershell not found")
		}
		cmd = exec.Command(ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", "install.ps1")
	} else {
		sh, err := exec.LookPath("sh")
		if err != nil {
			t.Skip("sh not found")
		}
		cmd = exec.Command(sh, "install.sh")
	}
	cmd.Env = append(os.Environ(),
		"MCPDECK_DOWNLOAD_URL="+url,
		"MCPDECK_INSTALL_DIR="+dest,
		"MCPDECK_NO_MODIFY_PATH=1",
		"NO_PROXY=127.0.0.1,localhost", "no_proxy=127.0.0.1,localhost",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// buildFake compiles a stand-in for mcpdeck that prints a known version.
func buildFake(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module fake\n\ngo 1.21\n")
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "fmt"

func main() { fmt.Println("`+fakeVersion+`") }
`)
	out := filepath.Join(dir, exeName())
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOFLAGS=")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake binary: %v\n%s", err, b)
	}
	return out
}

// writeArchive lays the binary out the way GoReleaser does (NFR-7 names).
func writeArchive(t *testing.T, dir, bin string) string {
	t.Helper()
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	var name string
	if runtime.GOOS == "windows" {
		name = fmt.Sprintf("mcpdeck_windows_%s.zip", runtime.GOARCH)
		zw := zip.NewWriter(&buf)
		for _, f := range []struct {
			n string
			b []byte
		}{{"mcpdeck.exe", data}, {"LICENSE", []byte("license")}} {
			w, err := zw.Create(f.n)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write(f.b)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
	} else {
		name = fmt.Sprintf("mcpdeck_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		for _, f := range []struct {
			n    string
			b    []byte
			mode int64
		}{{"LICENSE", []byte("license"), 0o644}, {"mcpdeck", data, 0o755}} {
			if err := tw.WriteHeader(&tar.Header{Name: f.n, Mode: f.mode, Size: int64(len(f.b))}); err != nil {
				t.Fatal(err)
			}
			_, _ = tw.Write(f.b)
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(dir, name), buf.String())
	return name
}

func exeName() string {
	if runtime.GOOS == "windows" {
		return "mcpdeck.exe"
	}
	return "mcpdeck"
}

func sha(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
