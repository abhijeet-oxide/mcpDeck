package main

import (
	"strings"
	"testing"
)

// REQ: NFR-9
func TestWrite(t *testing.T) {
	var r report
	r.Passed, r.Total = 1, 2
	r.ByCategory = map[string][2]int{"auth": {1, 1}, "lint": {0, 1}}
	r.Scenarios = append(r.Scenarios, struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Passed   bool   `json:"passed"`
	}{"lint/x", "lint", false})
	var b strings.Builder
	if err := write(&b, r); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"1 of 2 pass", "| auth | 1 | 1 |", "- lint/x"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}
