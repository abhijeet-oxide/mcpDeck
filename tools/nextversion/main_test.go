package main

import "testing"

// REQ: NFR-8
func TestNext(t *testing.T) {
	tests := []struct {
		name    string
		last    string
		commits []string
		want    string
		release bool
	}{
		{"first release", "", []string{"feat: core runtime"}, "v0.1.0", true},
		{"first release, docs only", "", []string{"docs: readme"}, "", false},
		{"fix bumps patch", "v0.1.0", []string{"fix(proxy): close body"}, "v0.1.1", true},
		{"perf bumps patch", "v0.1.0", []string{"perf: faster tokens"}, "v0.1.1", true},
		{"feat bumps minor", "v0.1.3", []string{"fix: a", "feat(ui): b"}, "v0.2.0", true},
		{"breaking with ! in 0.x bumps minor", "v0.4.2", []string{"feat(api)!: rename"}, "v0.5.0", true},
		{"breaking footer in 0.x bumps minor", "v0.4.2", []string{"fix: x\n\nBREAKING CHANGE: y"}, "v0.5.0", true},
		{"breaking after 1.0 bumps major", "v1.2.3", []string{"refactor!: drop flag"}, "v2.0.0", true},
		{"feat after 1.0 bumps minor", "v1.2.3", []string{"feat: x"}, "v1.3.0", true},
		{"no releasable commits", "v0.1.0", []string{"docs: x", "ci: y", "chore(deps): z", "test: w", "build: v", "refactor: u", "style: s"}, "", false},
		{"nothing since last tag", "v0.1.0", nil, "", false},
		{"merge commits and non-conventional are ignored", "v0.1.0", []string{"Merge pull request #3 from x", "update stuff"}, "", false},
		{"dependabot deps bump is a fix", "v0.1.0", []string{"fix(deps): bump x from 1 to 2"}, "v0.1.1", true},
		{"pre-release tag is the base", "v0.2.0-rc.1", []string{"fix: a"}, "v0.2.1", true},
		{"type is case-insensitive", "v0.1.0", []string{"Feat: a"}, "v0.2.0", true},
		{"feat without colon is not conventional", "v0.1.0", []string{"feature flags"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, release, err := Next(tt.last, tt.commits)
			if err != nil {
				t.Fatalf("Next: %v", err)
			}
			if got != tt.want || release != tt.release {
				t.Errorf("Next(%q, %q) = %q, %v; want %q, %v", tt.last, tt.commits, got, release, tt.want, tt.release)
			}
		})
	}
}

// REQ: NFR-8
func TestNextRejectsBadTag(t *testing.T) {
	if _, _, err := Next("release-1", []string{"fix: a"}); err == nil {
		t.Fatal("want an error for a tag that is not vMAJOR.MINOR.PATCH")
	}
}
