package scenarios

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The linter: blocks only clear violations on save, advises on the rest,
// and gives the same answer in the UI endpoint, the CLI and the admin MCP.

func init() {
	add("lint", "saving a tool with a hard-coded token is refused, with the rule and fix", func(t *testing.T) {
		need(t, "billing")
		k := kit(t, "billing")
		src := toolSrc(uniq("t"), "Lists the latest invoices with amount and status.", "", nil,
			`  const h = { Authorization: "Bearer sk-live-abcdefghijklmnopqrstuvwx" };
  return (await billing.listInvoices({ limit: 2 })).data.length;`)
		err := h.api("POST", "/api/toolkits/"+k+"/save", map[string]any{"source": src, "tests": []any{}}, nil)
		expect(t, err != nil && strings.Contains(err.Error(), "422") && strings.Contains(err.Error(), "secret-in-definition"), "got %v", err)
	})

	add("lint", "an input named api_key is refused for agents and people alike", func(t *testing.T) {
		need(t, "billing")
		k := kit(t, "billing")
		src := toolSrc(uniq("t"), "Lists the latest invoices with amount and status.", `api_key: { type: "string", description: "key" }`, []string{"api_key"},
			`  return (await billing.listInvoices({ limit: 2 })).data.length;`)
		err := h.api("POST", "/api/toolkits/"+k+"/save", map[string]any{"source": src, "tests": []any{}}, nil)
		expect(t, err != nil && strings.Contains(err.Error(), "secret-input"), "person: %v", err)
		err = h.apiAs("lint-agent", "POST", "/api/toolkits/"+k+"/save", map[string]any{"source": src, "tests": []any{}}, nil)
		expect(t, err != nil && strings.Contains(err.Error(), "secret-input"), "agent: %v", err)
	})

	add("lint", "a write tool that claims readOnlyHint is refused", func(t *testing.T) {
		need(t, "shop")
		k := kit(t, "shop")
		src := strings.Replace(toolSrc(uniq("t"), "Cancels one order that hasn't shipped yet.", `order_id: { type: "string", description: "ord_..." }`, []string{"order_id"},
			`  return (await shop.cancelOrder({ id: input.order_id })).status;`),
			"inputSchema:", `dontUseWhen: "just looking up an order", annotations: { readOnlyHint: true },
  inputSchema:`, 1)
		err := h.api("POST", "/api/toolkits/"+k+"/save", map[string]any{"source": src, "tests": []any{}}, nil)
		expect(t, err != nil && strings.Contains(err.Error(), "write-annotation"), "got %v", err)
	})

	add("lint", "an invalid input schema is refused", func(t *testing.T) {
		need(t, "billing")
		k := kit(t, "billing")
		src := toolSrc(uniq("t"), "Lists the latest invoices with amount and status.", `limit: { type: "int", description: "how many" }`, []string{"limit"},
			`  return (await billing.listInvoices({ limit: input.limit })).data.length;`)
		err := h.api("POST", "/api/toolkits/"+k+"/save", map[string]any{"source": src, "tests": []any{}}, nil)
		expect(t, err != nil && strings.Contains(err.Error(), "schema-valid"), "got %v", err)
	})

	add("lint", "warnings and hints don't block; the save reports them with fixes", func(t *testing.T) {
		need(t, "billing")
		k := kit(t, "billing")
		var out struct {
			Lint []struct{ Rule, Level, Fix, Source string } `json:"lint"`
		}
		h.mustAPI(t, "POST", "/api/toolkits/"+k+"/save", map[string]any{"source": toolSrc(uniq("t"), "Invoices.", `status: { type: "string", description: "paid or open" }`, []string{"status"},
			`  return (await billing.listInvoices({})).data.map((i: any) => i.id);`), "tests": []any{}}, &out)
		got := map[string]string{}
		for _, f := range out.Lint {
			got[f.Rule] = f.Level
			expect(t, f.Fix != "" && f.Source != "", "finding %s has no fix or source", f.Rule)
		}
		expect(t, got["description"] == "warn" && got["enum"] == "hint" && got["pagination"] == "hint", "got %v", got)
	})

	add("lint", "the editor lints an unsaved draft as you type", func(t *testing.T) {
		need(t, "billing")
		k := kit(t, "billing")
		var out struct {
			Draft []struct{ Rule, Level string } `json:"draft"`
		}
		h.mustAPI(t, "POST", "/api/toolkits/"+k+"/lint", map[string]any{"draft": toolSrc("get_data", "Gets it.", `password: { type: "string" }`, []string{"password"}, `  return 1;`)}, &out)
		got := map[string]string{}
		for _, f := range out.Draft {
			got[f.Rule] = f.Level
		}
		expect(t, got["secret-input"] == "block" && got["name"] == "warn", "got %v", got)
	})

	add("lint", "mcpdeck lint --json and --strict for CI", func(t *testing.T) {
		need(t, "billing")
		k := kit(t, "billing")
		h.saveTool(t, k, toolSrc(uniq("t"), "Invoices.", "", nil, `  return (await billing.listInvoices({ limit: 2 })).data.length;`))
		out := h.mustCLI(t, "lint", k, "--json")
		var rep struct {
			Findings []struct{ Rule, Level, Source string } `json:"findings"`
		}
		expect(t, json.Unmarshal([]byte(out), &rep) == nil && len(rep.Findings) > 0, "lint --json: %s", clip(out, 200))
		_, err := h.cli(t, "lint", k)
		expect(t, err == nil, "warnings alone fail lint: %v", err)
		_, err = h.cli(t, "lint", k, "--strict")
		expect(t, err != nil, "--strict passes with warnings")
	})

	add("lint", "the rule catalog the UI shows matches docs/lint-rules.md", func(t *testing.T) {
		var cat struct {
			Rules []struct{ ID, Severity string } `json:"rules"`
		}
		h.mustAPI(t, "GET", "/api/lint/rules", nil, &cat)
		doc := readRepoFile(t, "docs/lint-rules.md")
		expect(t, len(cat.Rules) >= 30, "only %d rules", len(cat.Rules))
		for _, r := range cat.Rules {
			expect(t, strings.Contains(doc, "| `"+r.ID+"` | "+r.Severity+" |"), "docs/lint-rules.md is missing %s (%s)", r.ID, r.Severity)
		}
	})

	add("lint", "admin MCP save_tool returns lint findings to the agent", func(t *testing.T) {
		need(t, "billing")
		k := kit(t, "billing")
		a := h.adminFor(t, "Lint agent")
		text, _ := a.call("save_tool", map[string]any{"toolkit": k, "source": toolSrc(uniq("t"), "Invoices.", "", nil, `  return (await billing.listInvoices({ limit: 2 })).data.length;`)})
		expect(t, strings.Contains(text, `"lint"`) && strings.Contains(text, "description"), "got %s", clip(text, 300))
		text, isErr := a.call("save_tool", map[string]any{"toolkit": k, "source": toolSrc(uniq("t"), "Lists the latest invoices with amount and status.", `password: { type: "string", description: "p" }`, []string{"password"}, `  return 1;`)})
		expect(t, isErr && strings.Contains(text, "secret-input"), "got %s", clip(text, 300))
	})
}

// readRepoFile reads a file at the repository root.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
