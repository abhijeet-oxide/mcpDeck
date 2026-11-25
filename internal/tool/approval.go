package tool

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// ApprovalInput is what the rules look at (AB-2).
type ApprovalInput struct {
	Version    store.ToolVersion
	Rules      store.Rules
	ByAgent    bool
	InScope    func(api string) bool
	Duplicates []string // names of library tools using the same calls (AB-5)
}

var addressing = regexp.MustCompile(`(?i)\b(ignore (all|previous|the)|always call|you must|disregard|system prompt|do not tell)\b|https?://`)

// DescriptionProblems checks agent-written text (AB-6).
func DescriptionProblems(v store.ToolVersion) []string {
	var out []string
	text := strings.Join([]string{v.Description, v.UseWhen, v.DontUseWhen}, " ")
	if n := len(v.Description); n < 15 || n > 500 {
		out = append(out, fmt.Sprintf("description is %d characters; keep it between 15 and 500", n))
	}
	if m := addressing.FindString(text); m != "" {
		out = append(out, fmt.Sprintf("description contains %q, which reads as instructions to agents or a link", m))
	}
	return out
}

// Evaluate runs the approval rules and returns the checks and the status:
// live or pending. Versions saved by a person go live (AB-2).
func Evaluate(in ApprovalInput) ([]store.Check, string) {
	v := in.Version
	var checks []store.Check
	add := func(rule, label string, ok bool, detail string) {
		checks = append(checks, store.Check{Rule: rule, Label: label, Passed: ok, Detail: detail})
	}
	add("only_reads", "Only reads (write tools always wait for a person)", v.Access == "read",
		map[bool]string{true: "", false: "uses a request that writes or isn't confirmed as read"}[v.Access == "read"])
	var out []string
	for _, a := range v.APIs {
		if in.InScope != nil && !in.InScope(a) {
			out = append(out, a)
		}
	}
	add("build_scope", "Uses only APIs in the build list", len(out) == 0, strings.Join(out, ", "))
	if in.Rules.RequireTests {
		ok := len(v.TestResults) > 0
		detail := ""
		if !ok {
			detail = "add at least one test (input + pm.test script)"
		}
		for _, t := range v.TestResults {
			if !t.Passed {
				ok = false
				detail = t.Name + ": " + t.Error
				break
			}
		}
		add("tests", "Has tests and they pass", ok, detail)
	}
	if in.Rules.DescriptionCheck {
		p := DescriptionProblems(v)
		add("description", "Description describes the tool", len(p) == 0, strings.Join(p, "; "))
	}
	if in.Rules.MaxOutputTokens > 0 {
		max := 0
		for _, t := range v.TestResults {
			if t.Tokens > max {
				max = t.Tokens
			}
		}
		add("output_size", fmt.Sprintf("Output under %d tokens on its tests", in.Rules.MaxOutputTokens), max <= in.Rules.MaxOutputTokens,
			map[bool]string{true: "", false: fmt.Sprintf("largest test output is %d tokens", max)}[max <= in.Rules.MaxOutputTokens])
	}
	if in.Rules.NoDuplicates {
		add("duplicate", "Not a duplicate of a library tool", len(in.Duplicates) == 0,
			map[bool]string{true: "", false: "uses the same requests as " + strings.Join(in.Duplicates, ", ")}[len(in.Duplicates) == 0])
	}
	if !in.ByAgent {
		return checks, "live"
	}
	if !in.Rules.AutoApprove {
		return checks, "pending"
	}
	for _, c := range checks {
		if !c.Passed {
			return checks, "pending"
		}
	}
	return checks, "live"
}
