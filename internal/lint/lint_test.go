package lint

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func rules(fs []Finding) map[string]string {
	m := map[string]string{}
	for _, f := range fs {
		m[f.Rule] = f.Level
	}
	return m
}

// REQ: GUIDE-2
func TestToolkit_CountTokensAndWrappers(t *testing.T) {
	var tools []Tool
	for i := 0; i < 42; i++ {
		tools = append(tools, Tool{Name: fmt.Sprintf("get_thing_%d", i), Description: "Returns one thing by its ID, with name and status.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"thing_id":{"type":"string","description":"x"}},"required":["thing_id"]}`),
			Calls:       []string{fmt.Sprintf("api.get%d", i)}})
	}
	r := Toolkit(tools)
	got := rules(r.Findings)
	if got["tool-count"] != Must || got["endpoint-wrappers"] != Avoid {
		t.Fatalf("findings %v", got)
	}
	if r.ListTokens == 0 || r.Score >= 100 {
		t.Fatalf("tokens %d score %d", r.ListTokens, r.Score)
	}
	small := Toolkit(tools[:3])
	if rules(small.Findings)["tool-count"] != "" {
		t.Fatal("3 tools is fine")
	}
}

// REQ: GUIDE-2
func TestToolDef_Rules(t *testing.T) {
	bad := Tool{Name: "new_tool", Description: "What it returns, in one line.", Access: "write",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}}}`),
		Source:      "export default async function (input) {\n  const res = await a.b();\n  return res;\n}", OutputTokens: 30000}
	got := rules(ToolDef(bad, 5))
	want := map[string]string{"name": Must, "description": Must, "use-when": Tip, "write-guard": Avoid, "param-name": Avoid,
		"param-docs": Tip, "required": Avoid, "raw-return": Avoid, "output-tokens": Must}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q (all %v)", k, got[k], v, got)
		}
	}
	good := Tool{Name: "find_vulnerabilities", Description: "Lists open vulnerabilities of one application image, with severity and fix version.",
		UseWhen: "A question is about security issues in an app", InputSchema: json.RawMessage(`{"type":"object","properties":{"app_name":{"type":"string","description":"App name"}},"required":["app_name"]}`),
		Source: `return pick(v, ["items[].id"]);`, OutputTokens: 400}
	if fs := ToolDef(good, 5); len(fs) != 0 {
		t.Errorf("a good tool has no findings, got %+v", fs)
	}
}

// REQ: GUIDE-3
func TestOutput_Analysis(t *testing.T) {
	var items []any
	for i := 0; i < 60; i++ {
		item := map[string]any{"id": fmt.Sprintf("1b4e28ba-2fa1-11d2-883f-0016d3cca4%02d", i), "note": nil}
		for j := 0; j < 30; j++ {
			item[fmt.Sprintf("f%d", j)] = j
		}
		items = append(items, item)
	}
	v := map[string]any{"items": items, "blob": strings.Repeat("QUJD", 80), "auth": map[string]any{"access_token": "x"}}
	r := Output(v)
	got := rules(r.Findings)
	for _, k := range []string{"field-count", "unbounded-list", "empty-fields", "blob", "credential-in-output"} {
		if got[k] == "" {
			t.Errorf("missing %s in %v", k, got)
		}
	}
	if r.Tokens == 0 || len(r.Largest) == 0 || r.Largest[0].Path != "blob" && r.Largest[0].Path != "items[].id" {
		t.Errorf("largest %+v", r.Largest)
	}
	if fs := Output(map[string]any{"name": "Northwind", "tier": "gold"}).Findings; len(fs) != 0 {
		t.Errorf("small output: %+v", fs)
	}
}
