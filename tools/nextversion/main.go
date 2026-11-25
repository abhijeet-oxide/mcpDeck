// Command nextversion prints the next release version from the conventional
// commits since the last v* tag (NFR-8). The release workflow runs it on main.
//
// Output, ready to append to $GITHUB_OUTPUT:
//
//	release=true
//	version=v0.2.0
//
// release=false means nothing since the last tag changes behaviour.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type bump int

const (
	none bump = iota
	patch
	minor
	major
)

// header is "type(scope)!: subject"; scope and ! are optional.
var header = regexp.MustCompile(`(?i)^([a-z]+)(\([^)]*\))?(!)?: \S`)

var semver = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z.-]+)?$`)

// Next returns the version after last for the given commit messages, and
// whether there is anything to release. last is "" before the first release.
func Next(last string, commits []string) (string, bool, error) {
	maj, min, pat := 0, 0, 0
	if last != "" {
		m := semver.FindStringSubmatch(last)
		if m == nil {
			return "", false, fmt.Errorf("tag %q is not vMAJOR.MINOR.PATCH", last)
		}
		maj, _ = strconv.Atoi(m[1])
		min, _ = strconv.Atoi(m[2])
		pat, _ = strconv.Atoi(m[3])
	}

	b := none
	for _, c := range commits {
		b = max(b, classify(c))
	}
	if b == none {
		return "", false, nil
	}
	if last == "" {
		return "v0.1.0", true, nil
	}
	if b == major && maj == 0 {
		b = minor // 0.x: breaking changes move the minor
	}
	switch b {
	case major:
		maj, min, pat = maj+1, 0, 0
	case minor:
		min, pat = min+1, 0
	default:
		pat++
	}
	return fmt.Sprintf("v%d.%d.%d", maj, min, pat), true, nil
}

func classify(msg string) bump {
	first, _, _ := strings.Cut(msg, "\n")
	m := header.FindStringSubmatch(strings.TrimSpace(first))
	if m == nil {
		return none
	}
	if m[3] == "!" || strings.Contains(msg, "\nBREAKING CHANGE:") || strings.Contains(msg, "\nBREAKING-CHANGE:") {
		return major
	}
	switch strings.ToLower(m[1]) {
	case "feat":
		return minor
	case "fix", "perf":
		return patch
	}
	return none
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "nextversion:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// Pre-release tags (v0.2.0-rc.1) are not a base: the stable line continues.
	last, err := git(ctx, "describe", "--tags", "--abbrev=0", "--match", "v*", "--exclude", "*-*")
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return err
		}
		last = "" // no tags yet
	}
	rng := "HEAD"
	if last != "" {
		rng = last + "..HEAD"
	}
	log, err := git(ctx, "log", "--no-merges", "--format=%B%x00", rng)
	if err != nil {
		return err
	}
	var commits []string
	for c := range strings.SplitSeq(log, "\x00") {
		if c = strings.TrimSpace(c); c != "" {
			commits = append(commits, c)
		}
	}
	v, release, err := Next(last, commits)
	if err != nil {
		return err
	}
	fmt.Printf("release=%t\nversion=%s\nprevious=%s\n", release, v, last)
	return nil
}

func git(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}
