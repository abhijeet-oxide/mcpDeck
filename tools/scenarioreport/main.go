// Command scenarioreport turns the scenario suite's JSON report into a
// Markdown summary: pass rate by category and the failing scenarios. CI
// appends it to the job summary.
//
//	go test ./e2e/scenarios -count=1 -args -report scenarios.json
//	go run ./tools/scenarioreport scenarios.json
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

type report struct {
	Passed     int               `json:"passed"`
	Total      int               `json:"total"`
	ByCategory map[string][2]int `json:"byCategory"`
	Scenarios  []struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Passed   bool   `json:"passed"`
	} `json:"scenarios"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: scenarioreport <report.json>")
		os.Exit(2)
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var r report
	if err := json.Unmarshal(b, &r); err != nil {
		fmt.Fprintln(os.Stderr, "not a scenario report:", err)
		os.Exit(1)
	}
	if err := write(os.Stdout, r); err != nil {
		os.Exit(1)
	}
	if r.Passed != r.Total || r.Total == 0 {
		os.Exit(1)
	}
}

func write(w io.Writer, r report) error {
	fmt.Fprintf(w, "### Scenarios: %d of %d pass\n\n| Category | Pass | Total |\n|---|---:|---:|\n", r.Passed, r.Total)
	cats := make([]string, 0, len(r.ByCategory))
	for c := range r.ByCategory {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	for _, c := range cats {
		v := r.ByCategory[c]
		fmt.Fprintf(w, "| %s | %d | %d |\n", c, v[0], v[1])
	}
	var failed []string
	for _, s := range r.Scenarios {
		if !s.Passed {
			failed = append(failed, s.Name)
		}
	}
	if len(failed) > 0 {
		fmt.Fprintf(w, "\n**Failing:**\n\n")
		for _, f := range failed {
			fmt.Fprintf(w, "- %s\n", f)
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}
