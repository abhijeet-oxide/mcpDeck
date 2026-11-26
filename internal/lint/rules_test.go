package lint

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func sch(s string) json.RawMessage { return json.RawMessage(s) }

const okSchema = `{"type":"object","properties":{"app_name":{"type":"string","description":"App name"}},"required":["app_name"]}`

func base() Tool {
	return Tool{Name: "find_vulnerabilities", Description: "Lists open vulnerabilities of one application image, with severity and fix version.",
		UseWhen: "A question is about security issues in an app", InputSchema: sch(okSchema), Source: `return pick(v, ["items[].id"]);`, Access: "read"}
}

func with(f func(*Tool)) Tool { t := base(); f(&t); return t }

func manyTools(n int, f func(i int) Tool) []Tool {
	var out []Tool
	for i := 0; i < n; i++ {
		out = append(out, f(i))
	}
	return out
}

func yes() *bool { b := true; return &b }
func no() *bool  { b := false; return &b }

// ruleCase is one rule with an input that triggers it and one that doesn't.
type ruleCase struct {
	rule     string
	severity string
	hit      func() []Finding
	miss     func() []Finding
}

func def(t Tool) func() []Finding    { return func() []Finding { return ToolDef(t, 5) } }
func kit(ts []Tool) func() []Finding { return func() []Finding { return Toolkit(ts).Findings } }
func outp(v any) func() []Finding    { return func() []Finding { return Output(v).Findings } }

func bigList(n int) []any {
	var l []any
	for i := 0; i < n; i++ {
		l = append(l, map[string]any{"n": i})
	}
	return l
}

var ruleCases = []ruleCase{
	{"schema-valid", Block, def(with(func(t *Tool) {
		t.InputSchema = sch(`{"type":"object","properties":{"a":{"type":"text"}},"required":["b"]}`)
	})), def(base())},
	{"secret-in-definition", Block, def(with(func(t *Tool) {
		t.Source = `const h = {Authorization: "Bearer abcdefghijklmnopqrstuvwxyz0123"}; return pick(v, ["a"]);`
	})), def(with(func(t *Tool) { t.Source = `const h = {Authorization: "Bearer {{token}}"}; return pick(v, ["a"]);` }))},
	{"secret-input", Block, def(with(func(t *Tool) {
		t.InputSchema = sch(`{"type":"object","properties":{"api_key":{"type":"string","description":"k"}},"required":["api_key"]}`)
	})), def(with(func(t *Tool) {
		t.InputSchema = sch(`{"type":"object","properties":{"page_token":{"type":"string","description":"k"}},"required":["page_token"]}`)
	}))},
	{"credential-in-output", Block, outp(map[string]any{"access_token": "eyJabc"}), outp(map[string]any{"next_token": "abc", "token_count": "12"})},
	{"write-annotation", Block, def(with(func(t *Tool) {
		t.Access, t.DontUseWhen, t.Annotations = "write", "just reading", &Annotations{ReadOnlyHint: yes()}
	})), def(with(func(t *Tool) {
		t.Access, t.DontUseWhen, t.Methods, t.Annotations = "write", "just reading", []string{"POST"}, &Annotations{DestructiveHint: no()}
	}))},
	{"name-format", Warn, def(with(func(t *Tool) { t.Name = "find vulns" })), def(base())},
	{"endpoint-wrappers", Warn, kit(manyTools(5, func(i int) Tool {
		return with(func(t *Tool) { t.Name = fmt.Sprintf("get_x_%d", i); t.Calls = []string{fmt.Sprintf("a.r%d", i)} })
	})), kit(manyTools(5, func(i int) Tool {
		return with(func(t *Tool) {
			t.Name = fmt.Sprintf("get_x_%d", i)
			t.Calls = []string{"a.login", fmt.Sprintf("a.r%d", i)}
		})
	}))},
	{"overlap", Warn, kit([]Tool{with(func(t *Tool) { t.Name = "a_one" }), with(func(t *Tool) { t.Name = "a_two" })}),
		kit([]Tool{with(func(t *Tool) { t.Name = "a_one" }), with(func(t *Tool) {
			t.Name = "b_two"
			t.Description = "Creates a ticket in the support queue for a customer."
		})})},
	{"catch-all", Warn, def(with(func(t *Tool) {
		t.InputSchema = sch(`{"type":"object","properties":{"method":{"type":"string","description":"m"},"path":{"type":"string","description":"p"}},"required":["method","path"]}`)
	})), def(base())},
	{"tool-count", Warn, kit(manyTools(16, func(i int) Tool { return with(func(t *Tool) { t.Name = fmt.Sprintf("get_x_%d", i) }) })),
		kit(manyTools(3, func(i int) Tool { return with(func(t *Tool) { t.Name = fmt.Sprintf("get_x_%d", i) }) }))},
	{"list-tokens", Warn, kit(manyTools(20, func(i int) Tool {
		return with(func(t *Tool) {
			t.Name = fmt.Sprintf("get_x_%d", i)
			t.Description = strings.Repeat("Lists the open items of one queue. ", 50)[:1000]
		})
	})), kit([]Tool{base()})},
	{"tool-tokens", Warn, def(with(func(t *Tool) {
		var p []string
		for i := 0; i < 8; i++ {
			p = append(p, fmt.Sprintf(`"p%d":{"type":"string","description":"%s"}`, i, strings.Repeat("word ", 200)))
		}
		t.InputSchema = sch(`{"type":"object","properties":{` + strings.Join(p, ",") + `},"required":["p0"]}`)
	})), def(base())},
	{"name", Warn, def(with(func(t *Tool) { t.Name = "get_data" })), def(base())},
	{"description", Warn, def(with(func(t *Tool) { t.Description = "Gets it." })), def(base())},
	{"use-when", Hint, def(with(func(t *Tool) { t.UseWhen = "" })), def(base())},
	{"write-guard", Warn, def(with(func(t *Tool) { t.Access = "write" })), def(with(func(t *Tool) { t.Access = "write"; t.DontUseWhen = "just reading" }))},
	{"actionable-errors", Hint, def(with(func(t *Tool) { t.Source = `if (!x) throw new Error("failed"); return pick(v, ["a"]);` })),
		def(with(func(t *Tool) {
			t.Source = `if (!x) throw new Error("No image named that; call list_images for names"); return pick(v, ["a"]);`
		}))},
	{"param-name", Warn, def(with(func(t *Tool) {
		t.InputSchema = sch(`{"type":"object","properties":{"id":{"type":"string","description":"x"}},"required":["id"]}`)
	})), def(base())},
	{"param-docs", Hint, def(with(func(t *Tool) {
		t.InputSchema = sch(`{"type":"object","properties":{"app_name":{"type":"string"}},"required":["app_name"]}`)
	})), def(base())},
	{"enum", Hint, def(with(func(t *Tool) {
		t.InputSchema = sch(`{"type":"object","properties":{"status":{"type":"string","description":"s"}},"required":["status"]}`)
	})), def(with(func(t *Tool) {
		t.InputSchema = sch(`{"type":"object","properties":{"status":{"type":"string","enum":["open","closed"]}},"required":["status"]}`)
	}))},
	{"required", Warn, def(with(func(t *Tool) {
		t.InputSchema = sch(`{"type":"object","properties":{"app_name":{"type":"string","description":"x"}}}`)
	})), def(base())},
	{"params", Warn, def(with(func(t *Tool) {
		var p []string
		for i := 0; i < 9; i++ {
			p = append(p, fmt.Sprintf(`"p%d":{"type":"string","description":"x"}`, i))
		}
		t.InputSchema = sch(`{"type":"object","properties":{` + strings.Join(p, ",") + `},"required":["p0"]}`)
	})), def(base())},
	{"no-params", Hint, def(with(func(t *Tool) { t.InputSchema = sch(`{"type":"object"}`) })),
		def(with(func(t *Tool) { t.InputSchema = sch(`{"type":"object","additionalProperties":false}`) }))},
	{"pagination", Hint, def(with(func(t *Tool) { t.Source = `return pick(res.items.map(i => i.name), []);` })),
		def(with(func(t *Tool) { t.Source = `return pick(res.items.slice(0, input.limit).map(i => i.name), []);` }))},
	{"raw-return", Warn, def(with(func(t *Tool) { t.Source = "const res = await a.b();\nreturn res;" })), def(base())},
	{"output-tokens", Warn, def(with(func(t *Tool) { t.OutputTokens = 3000 })), def(with(func(t *Tool) { t.OutputTokens = 300 }))},
	{"field-count", Warn, outp(func() map[string]any {
		m := map[string]any{}
		for i := 0; i < 30; i++ {
			m[fmt.Sprint("f", i)] = i
		}
		return m
	}()), outp(map[string]any{"a": 1})},
	{"unbounded-list", Warn, outp(map[string]any{"items": bigList(60)}), outp(map[string]any{"items": bigList(10)})},
	{"blob", Warn, outp(map[string]any{"img": strings.Repeat("QUJD", 80)}), outp(map[string]any{"img": "a.png"})},
	{"deep-nesting", Hint, outp(map[string]any{"a": map[string]any{"b": map[string]any{"c": map[string]any{"d": map[string]any{"e": 1}}}}}), outp(map[string]any{"a": 1})},
	{"empty-fields", Hint, outp(map[string]any{"note": nil, "a": 1}), outp(map[string]any{"a": 1})},
	{"long-text", Hint, outp(map[string]any{"body": strings.Repeat("word ", 500)}), outp(map[string]any{"body": "short"})},
	{"opaque-ids", Hint, outp(map[string]any{"a": "1b4e28ba-2fa1-11d2-883f-0016d3cca427", "b": "1b4e28ba-2fa1-11d2-883f-0016d3cca428", "c": "1b4e28ba-2fa1-11d2-883f-0016d3cca429"}),
		outp(map[string]any{"a": "1b4e28ba-2fa1-11d2-883f-0016d3cca427"})},
}

func has(fs []Finding, rule string) (Finding, bool) {
	for _, f := range fs {
		if f.Rule == rule {
			return f, true
		}
	}
	return Finding{}, false
}

// REQ: LINT-1
func TestRules_EachRuleHitsAndMisses(t *testing.T) {
	for _, c := range ruleCases {
		t.Run(c.rule, func(t *testing.T) {
			f, ok := has(c.hit(), c.rule)
			if !ok {
				t.Fatalf("expected %s, got %+v", c.rule, c.hit())
			}
			if f.Level != c.severity {
				t.Errorf("severity %s, want %s", f.Level, c.severity)
			}
			if f.Detail == "" || f.Fix == "" || f.Source == "" {
				t.Errorf("finding needs detail, fix and source: %+v", f)
			}
			if _, ok := has(c.miss(), c.rule); ok {
				t.Errorf("did not expect %s: %+v", c.rule, c.miss())
			}
		})
	}
}

// REQ: LINT-1
func TestRules_CatalogMatchesTests(t *testing.T) {
	tested := map[string]bool{}
	for _, c := range ruleCases {
		tested[c.rule] = true
		r, ok := ruleByID[c.rule]
		if !ok {
			t.Errorf("rule %s is reported but not in Rules", c.rule)
			continue
		}
		if r.Severity != c.severity {
			t.Errorf("rule %s: catalog says %s, finding says %s", c.rule, r.Severity, c.severity)
		}
	}
	for _, r := range Rules {
		if !tested[r.ID] {
			t.Errorf("rule %s has no test case", r.ID)
		}
		if len(r.Sources) == 0 || r.Why == "" || r.Fix == "" || r.Check == "" {
			t.Errorf("rule %s needs why, check, fix and a source", r.ID)
		}
		for _, s := range r.Sources {
			if _, ok := sourceByKey[s]; !ok {
				t.Errorf("rule %s cites unknown source %s", r.ID, s)
			}
		}
	}
}

// REQ: LINT-1
func TestRules_OnlyClearViolationsBlock(t *testing.T) {
	allowed := map[string]bool{"schema-valid": true, "secret-in-definition": true, "secret-input": true, "credential-in-output": true, "write-annotation": true}
	for _, r := range Rules {
		if r.Severity == Block && !allowed[r.ID] {
			t.Errorf("%s blocks, but only spec violations, credential leaks and wrong write annotations may", r.ID)
		}
	}
}

// REQ: LINT-1
func TestRules_DocIsCurrent(t *testing.T) {
	b, err := os.ReadFile("../../docs/lint-rules.md")
	if err != nil {
		t.Fatal(err)
	}
	// Windows checkouts may convert line endings.
	if strings.ReplaceAll(string(b), "\r\n", "\n") != Markdown() {
		t.Fatal("docs/lint-rules.md is out of date; run: go run ./tools/lintdoc > docs/lint-rules.md")
	}
}

// REQ: LINT-1
func TestFindSecret_NoFalsePositives(t *testing.T) {
	for _, s := range []string{`Authorization: "Bearer {{token}}"`, `"password": "{{password}}"`, "Returns the token count for a prompt.", "sk-short"} {
		if w := FindSecret(s); w != "" {
			t.Errorf("%q flagged as %s", s, w)
		}
	}
	for _, s := range []string{"-----BEGIN RSA PRIVATE KEY-----", "AKIAIOSFODNN7EXAMPLE", `{"password": "hunter22"}`, "xoxb-1234567890-abcdef"} {
		if FindSecret(s) == "" {
			t.Errorf("%q not flagged", s)
		}
	}
}
