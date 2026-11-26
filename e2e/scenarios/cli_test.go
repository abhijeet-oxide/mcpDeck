package scenarios

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/e2e/mockenv"
)

// The terminal path from the README, the builder endpoint an agent uses to
// make tools, and install hygiene.

func init() {
	add("cli", "mcpdeck --version prints the version", func(t *testing.T) {
		out, err := exec.Command(h.bin, "--version").CombinedOutput()
		expect(t, err == nil && strings.Contains(string(out), "mcpdeck"), "got %q (%v)", out, err)
	})

	add("cli", "README terminal path: import, auth, toolkit, tool, connect, call", func(t *testing.T) {
		api := "cliShop"
		h.mustCLI(t, "import", h.env.URLs["shop"]+"/openapi.json", "--api", api)
		h.mustCLI(t, "var", "set", api, "email", mockenv.ShopUser)
		cmd := h.cliCmd("secret", "set", api, "password")
		cmd.Stdin = strings.NewReader(mockenv.ShopPass + "\n")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("secret set: %v %s", err, out)
		}
		h.mustCLI(t, "request", "set-body", api, "login", `{"email": "{{email}}", "password": "{{password}}"}`)
		h.mustCLI(t, "auth", "set", api, "--type", "login", "--request", "login", "--capture", "json:access_token")
		h.mustCLI(t, "toolkit", "create", "cli-orders", "--apis", api)
		h.mustCLI(t, "tool", "generate", "cli-orders", "--from", api+".listOrders", "--name", "recent_orders")
		var c struct {
			URL   string `json:"url"`
			Token string `json:"token"`
		}
		out := h.mustCLI(t, "connect", "claude", "--toolkit", "cli-orders", "--json", "--print")
		if err := json.Unmarshal([]byte(out[strings.Index(out, "{"):]), &c); err != nil || c.Token == "" {
			t.Fatalf("connect --json has no token: %s", out)
		}
		a := h.agentWithToken(t, "cli-orders", c.Token, "")
		res := a.ok("recent_orders", map[string]any{})
		expect(t, strings.Contains(asJSON(res), "ord_"), "got %s", clip(asJSON(res), 200))
	})

	add("cli", "toolkit list and tool list as JSON", func(t *testing.T) {
		need(t, "shop")
		k := kit(t, "shop")
		h.saveTool(t, k, toolSrc(uniq("t"), "Count orders.", "", nil, `  return (await shop.listOrders({})).meta.total;`))
		var kits []map[string]any
		expect(t, json.Unmarshal([]byte(h.mustCLI(t, "toolkit", "list", "--json")), &kits) == nil && len(kits) > 0, "toolkit list --json is not JSON")
		var tools []map[string]any
		expect(t, json.Unmarshal([]byte(h.mustCLI(t, "tool", "list", k, "--json")), &tools) == nil && len(tools) == 1, "tool list --json is not JSON")
	})

	add("cli", "help --json describes every command for agents", func(t *testing.T) {
		var cmds []struct {
			Name  string `json:"name"`
			Short string `json:"short"`
		}
		out := h.mustCLI(t, "help", "--json")
		expect(t, json.Unmarshal([]byte(out), &cmds) == nil && len(cmds) >= 10, "help --json: %s", clip(out, 200))
	})

	add("cli", "tool generate refuses a write request without --write", func(t *testing.T) {
		need(t, "shop")
		k := kit(t, "shop")
		_, err := h.cli(t, "tool", "generate", k, "--from", "shop.cancelOrder")
		expect(t, err != nil, "generated a write tool silently")
	})

	add("cli", "pull and push round-trip a toolkit as files", func(t *testing.T) {
		need(t, "shop")
		k := kit(t, "shop")
		name := uniq("t")
		h.saveTool(t, k, toolSrc(name, "Count orders.", "", nil, `  return (await shop.listOrders({})).meta.total;`))
		dir, _ := os.MkdirTemp("", "pull")
		defer os.RemoveAll(dir)
		h.mustCLI(t, "pull", k, "--dir", dir)
		f := filepath.Join(dir, name+".ts")
		b, err := os.ReadFile(f)
		expect(t, err == nil, "no %s after pull", f)
		_ = os.WriteFile(f, []byte(strings.Replace(string(b), "Count orders.", "Count all orders.", 1)), 0o644)
		h.mustCLI(t, "push", k, "--dir", dir)
		a := h.agentFor(t, k, "read")
		a.ok(name, map[string]any{})
	})

	add("install", "database and key files are private (0600)", func(t *testing.T) {
		filepath.Walk(h.data, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(p, ".db") || strings.HasSuffix(p, ".key") || strings.HasSuffix(p, "-wal")) {
				expect(t, info.Mode().Perm()&0o077 == 0, "%s is %v", filepath.Base(p), info.Mode().Perm())
			}
			return nil
		})
	})

	add("install", "no-keychain fallback is announced", func(t *testing.T) {
		b, _ := os.ReadFile(h.logFile)
		expect(t, strings.Contains(strings.ToLower(string(b)), "keychain"), "the server log never says the key is in a file instead of the keychain")
	})

	// ── Builder endpoint ──
	add("builder", "search_apis and describe_api find the right request", func(t *testing.T) {
		need(t, "jira")
		b := h.builderFor(t, kit(t, "jira"), "read")
		text, _ := b.call("search_apis", map[string]any{"query": "search issues jql"})
		expect(t, strings.Contains(text, "jira.searchForIssuesUsingJql"), "got %s", clip(text, 300))
		text, _ = b.call("describe_api", map[string]any{"request": "jira.searchForIssuesUsingJql"})
		expect(t, strings.Contains(text, "startAt"), "got %s", clip(text, 300))
	})

	add("builder", "run_api through a login chain", func(t *testing.T) {
		need(t, "shop")
		b := h.builderFor(t, kit(t, "shop"), "read")
		text, isErr := b.call("run_api", map[string]any{"request": "shop.listOrders", "args": map[string]any{"per_page": 2}})
		expect(t, !isErr && strings.Contains(text, "ord_0001"), "got %s", clip(text, 300))
	})

	add("builder", "save_tool read goes live, write waits", func(t *testing.T) {
		need(t, "shop")
		k := kit(t, "shop")
		b := h.builderFor(t, k, "read_write")
		r := uniq("t")
		text, _ := b.call("save_tool", map[string]any{"source": toolSrc(r, "Shipping address of one order.", `id: { type: "string" }`, []string{"id"}, `  return (await shop.getOrder({ id: input.id })).shipping_address;`),
			"tests": []any{map[string]any{"name": "city", "input": map[string]any{"id": "ord_0004"}, "script": `pm.test("city", () => pm.expect(output.city).to.equal("Springfield"))`}}})
		expect(t, strings.Contains(text, "live"), "read tool: %s", clip(text, 300))
		w := uniq("t")
		text, _ = b.call("save_tool", map[string]any{"source": toolSrc(w, "Cancel an order that hasn't shipped.", `id: { type: "string" }`, []string{"id"}, `  return await shop.cancelOrder({ id: input.id });`),
			"tests": []any{map[string]any{"name": "example", "input": map[string]any{"id": "ord_0003"}}}})
		expect(t, strings.Contains(text, "pending"), "write tool: %s", clip(text, 300))
	})

	add("builder", "test_tool runs against the live API", func(t *testing.T) {
		need(t, "billing")
		b := h.builderFor(t, kit(t, "billing"), "read")
		text, isErr := b.call("test_tool", map[string]any{"source": toolSrc("probe", "Probe.", "", nil, `  return (await billing.listInvoices({ limit: 2 })).data.length;`), "input": map[string]any{}})
		expect(t, !isErr && strings.Contains(text, "2"), "got %s", clip(text, 300))
	})

	add("builder", "a builder can't see or change auth settings", func(t *testing.T) {
		need(t, "shop")
		b := h.builderFor(t, kit(t, "shop"), "read_write")
		text, _ := b.call("run_script", map[string]any{"code": `return typeof vars === "object" ? Object.keys(vars) : null;`})
		expect(t, !strings.Contains(text, "password"), "a builder sees credential variable names or values: %s", clip(text, 200))
	})

	add("secrets", "secret values never appear in the API detail the UI loads", func(t *testing.T) {
		for _, s := range []string{"shop", "billing", "legacy", "payments", "jira", "github"} {
			a := need(t, s)
			var raw map[string]any
			h.mustAPI(t, "GET", "/api/apis/"+a.ID, nil, &raw)
			h.noLeaks(t, s+" API detail", asJSON(raw))
		}
	})

	add("secrets", "call log holds no credentials after the whole suite", func(t *testing.T) {
		h.noLeaksInCallLog(t)
	})
}
