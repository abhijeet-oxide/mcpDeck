// Command claimscheck keeps the README honest (NFR-10). Every factual claim
// in README.md carries a marker like <!-- C3 -->; docs/claims.md lists each
// claim with the tests that prove it. This fails when a marker has no row, a
// row has no test, a named test doesn't exist, or a row is unused.
//
//	go run ./tools/claimscheck
//
// Tests are named `scenario:<category>/<name>` (e2e/scenarios) or
// `go:<TestFunc>` (any _test.go file).
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	markerRe   = regexp.MustCompile(`<!-- (C\d+) -->`)
	rowRe      = regexp.MustCompile(`^\|\s*(C\d+)\s*\|(.*)\|(.*)\|\s*$`)
	evidenceRe = regexp.MustCompile("`(scenario|go):([^`]+)`")
	scenarioRe = regexp.MustCompile(`add\("([a-z-]+)",\s*"((?:[^"\\]|\\.)*)"`)
	goTestRe   = regexp.MustCompile(`(?m)^func (Test\w+)\(`)
)

func main() {
	problems, err := check(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, p := range problems {
		fmt.Println(p)
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
	fmt.Println("every README claim has a test")
}

func check(root string) ([]string, error) {
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		return nil, err
	}
	claims, err := os.ReadFile(filepath.Join(root, "docs", "claims.md"))
	if err != nil {
		return nil, err
	}
	scenarios, goTests := map[string]bool{}, map[string]bool{}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == ".git" || d.Name() == "dist") {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range goTestRe.FindAllStringSubmatch(string(b), -1) {
			goTests[m[1]] = true
		}
		if strings.Contains(filepath.ToSlash(p), "e2e/scenarios/") {
			for _, m := range scenarioRe.FindAllStringSubmatch(string(b), -1) {
				scenarios[m[1]+"/"+strings.ReplaceAll(m[2], `\"`, `"`)] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var problems []string
	rows := map[string]bool{}
	for i, line := range strings.Split(string(claims), "\n") {
		m := rowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id := m[1]
		rows[id] = true
		ev := evidenceRe.FindAllStringSubmatch(m[3], -1)
		if len(ev) == 0 {
			problems = append(problems, fmt.Sprintf("docs/claims.md:%d: %s has no test", i+1, id))
		}
		for _, e := range ev {
			switch e[1] {
			case "scenario":
				if !scenarios[e[2]] {
					problems = append(problems, fmt.Sprintf("docs/claims.md:%d: %s names scenario %q, which doesn't exist", i+1, id, e[2]))
				}
			case "go":
				if !goTests[e[2]] {
					problems = append(problems, fmt.Sprintf("docs/claims.md:%d: %s names Go test %s, which doesn't exist", i+1, id, e[2]))
				}
			}
		}
	}
	used := map[string]bool{}
	for _, m := range markerRe.FindAllStringSubmatch(string(readme), -1) {
		used[m[1]] = true
		if !rows[m[1]] {
			problems = append(problems, fmt.Sprintf("README.md: claim %s has no row in docs/claims.md", m[1]))
		}
	}
	var unused []string
	for id := range rows {
		if !used[id] {
			unused = append(unused, id)
		}
	}
	sort.Strings(unused)
	for _, id := range unused {
		problems = append(problems, fmt.Sprintf("docs/claims.md: %s isn't used in README.md; remove it or mark the claim", id))
	}
	return problems, nil
}
