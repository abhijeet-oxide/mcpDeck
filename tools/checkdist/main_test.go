package main

import (
	"strings"
	"testing"
)

func goodDist() dist {
	d := dist{Version: "0.2.0", Checksums: map[string]bool{"install.sh": true, "install.ps1": true}}
	for _, p := range platforms {
		ext, bext := ".tar.gz", ""
		if p.os == "windows" {
			ext, bext = ".zip", ".exe"
		}
		d.Artifacts = append(d.Artifacts,
			artifact{Type: "Archive", Name: "mcpdeck_" + p.os + "_" + p.arch + ext, Size: 20 << 20},
			artifact{Type: "Binary", Name: "mcpdeck_" + p.os + "_" + p.arch + bext, Size: 50 << 20})
		d.Checksums["mcpdeck_"+p.os+"_"+p.arch+ext] = true
		d.Checksums["mcpdeck_"+p.os+"_"+p.arch+bext] = true
	}
	d.HostVersion = "mcpdeck 0.2.0 (abc1234, 2026-11-25T00:00:00Z) linux/amd64"
	return d
}

// REQ: NFR-1, NFR-5, NFR-7
func TestCheck(t *testing.T) {
	tests := []struct {
		name   string
		change func(*dist)
		want   string
	}{
		{"complete release", func(*dist) {}, ""},
		{"version not stamped", func(d *dist) { d.HostVersion = "mcpdeck dev (none, unknown) linux/amd64" }, "does not report version 0.2.0"},
		{"missing archive", func(d *dist) { d.Artifacts = d.Artifacts[2:] }, "missing mcpdeck_windows_amd64.zip"},
		{"missing checksum", func(d *dist) { delete(d.Checksums, "install.sh") }, "install.sh is not in checksums.txt"},
		{"binary too big", func(d *dist) { d.Artifacts[1].Size = 61 << 20 }, "over the 60 MB limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := goodDist()
			tt.change(&d)
			errs := check(d)
			got := strings.Join(errs, "\n")
			if tt.want == "" && len(errs) > 0 {
				t.Fatalf("want no problems, got:\n%s", got)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Fatalf("want a problem containing %q, got:\n%s", tt.want, got)
			}
		})
	}
}
