// Command checkdist checks a GoReleaser dist/ folder before anything is
// published (NFR-1, NFR-5, NFR-7): every platform has its archive and bare
// binary under a version-less name, everything is in checksums.txt, binaries
// are under the size limit, and the host binary reports the release version.
//
//	go run ./tools/checkdist dist
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const maxBinary = 60 << 20 // NFR-1

var platforms = []struct{ os, arch string }{
	{"windows", "amd64"}, {"windows", "arm64"},
	{"darwin", "amd64"}, {"darwin", "arm64"},
	{"linux", "amd64"}, {"linux", "arm64"},
}

type artifact struct {
	Type string
	Name string
	Path string
	Size int64
}

type dist struct {
	Version     string
	Artifacts   []artifact
	Checksums   map[string]bool
	HostVersion string
}

func check(d dist) []string {
	var errs []string
	byName := map[string]artifact{}
	for _, a := range d.Artifacts {
		byName[a.Name] = a
	}
	want := []string{"install.sh", "install.ps1"}
	for _, p := range platforms {
		base := "mcpdeck_" + p.os + "_" + p.arch
		if p.os == "windows" {
			want = append(want, base+".zip", base+".exe")
		} else {
			want = append(want, base+".tar.gz", base)
		}
	}
	for _, n := range want {
		if strings.HasPrefix(n, "mcpdeck_") {
			a, ok := byName[n]
			if !ok {
				errs = append(errs, "missing "+n)
				continue
			}
			if a.Type == "Binary" && a.Size > maxBinary {
				errs = append(errs, fmt.Sprintf("%s is %d MB, over the 60 MB limit (NFR-1)", n, a.Size>>20))
			}
		}
		if !d.Checksums[n] {
			errs = append(errs, n+" is not in checksums.txt")
		}
	}
	if !strings.HasPrefix(d.HostVersion, "mcpdeck "+d.Version+" ") {
		errs = append(errs, fmt.Sprintf("host binary prints %q, which does not report version %s", d.HostVersion, d.Version))
	}
	return errs
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: checkdist <dist dir>")
		os.Exit(2)
	}
	d, err := load(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "checkdist:", err)
		os.Exit(1)
	}
	if errs := check(d); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "checkdist:", e)
		}
		os.Exit(1)
	}
	fmt.Printf("checkdist: %d artifacts OK, %s\n", len(d.Artifacts), d.HostVersion)
}

func load(dir string) (dist, error) {
	var d dist
	var meta struct{ Version string }
	if err := readJSON(filepath.Join(dir, "metadata.json"), &meta); err != nil {
		return d, err
	}
	d.Version = meta.Version

	var arts []struct {
		Type, Name, Path, Goos, Goarch string
	}
	if err := readJSON(filepath.Join(dir, "artifacts.json"), &arts); err != nil {
		return d, err
	}
	var host string
	for _, a := range arts {
		if a.Type != "Archive" && a.Type != "Binary" {
			continue
		}
		st, err := os.Stat(a.Path)
		if err != nil {
			return d, fmt.Errorf("artifact %s: %w", a.Name, err)
		}
		d.Artifacts = append(d.Artifacts, artifact{Type: a.Type, Name: a.Name, Path: a.Path, Size: st.Size()})
		if a.Type == "Binary" && a.Goos == runtime.GOOS && a.Goarch == runtime.GOARCH {
			host = a.Path
		}
	}

	f, err := os.Open(filepath.Join(dir, "checksums.txt"))
	if err != nil {
		return d, err
	}
	defer f.Close()
	d.Checksums = map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if fields := strings.Fields(sc.Text()); len(fields) == 2 && len(fields[0]) == 64 {
			d.Checksums[fields[1]] = true
		}
	}

	if host == "" {
		return d, fmt.Errorf("no binary for this machine (%s/%s) in %s", runtime.GOOS, runtime.GOARCH, dir)
	}
	out, err := exec.Command(host, "version").Output()
	if err != nil {
		return d, fmt.Errorf("%s version: %w", host, err)
	}
	d.HostVersion = strings.TrimSpace(string(out))
	return d, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
