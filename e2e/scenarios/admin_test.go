package scenarios

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/e2e/mockenv"
	"github.com/abhijeet-oxide/mcpdeck/internal/tokens"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Builder parity: an agent sets mcpDeck up through the admin MCP server
// (`mcpdeck mcp`) alone, with the same reach as the CLI and the UI, and
// without the decisions only a person makes.

// adminFor starts `mcpdeck mcp` on stdio as an MCP client would.
func (h *harness) adminFor(t *testing.T, name string, env ...string) *agent {
	t.Helper()
	cmd := exec.Command(h.bin, "mcp", "--agent", name, "--data", h.data)
	cmd.Env = append(append(os.Environ(), "MCPDECK_NO_KEYCHAIN=1"), env...)
	cl := sdk.NewClient(&sdk.Implementation{Name: "scenario-admin", Version: "1"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cs, err := cl.Connect(ctx, &sdk.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("admin MCP: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return &agent{t: t, cs: cs, h: h}
}

func (a *agent) must(name string, args map[string]any) string {
	a.t.Helper()
	text, isErr := a.call(name, args)
	if isErr {
		a.t.Fatalf("%s: %s", name, text)
	}
	return text
}

func init() {
	add("parity", "admin MCP sets up mutual TLS with the client key from its environment", func(t *testing.T) {
		api := strings.ReplaceAll(uniq("adminVault"), "_", "")
		ad := h.adminFor(t, "TLS agent", "VAULT_TOKEN="+mockenv.MTLSToken, "VAULT_CLIENT_KEY="+string(h.env.ClientKey))
		ad.must("import_api", map[string]any{"source": `curl "` + h.env.URLs["vault"] + `/v1/sys/health"`, "api": api, "ca_pem": string(h.env.CACert)})
		ad.must("secret_from_env", map[string]any{"api": api, "name": "vaultToken", "env": "VAULT_TOKEN"})
		ad.must("secret_from_env", map[string]any{"api": api, "name": "clientKey", "env": "VAULT_CLIENT_KEY"})
		ad.must("set_auth", map[string]any{"api": api, "auth": map[string]any{"type": "apikey", "key": "X-Vault-Token", "in": "header", "value": "{{vaultToken}}"}})
		ad.must("set_tls", map[string]any{"api": api, "client_cert_pem": string(h.env.ClientCert), "client_key_var": "clientKey"})
		got := ad.must("get_api", map[string]any{"api": api})
		expect(t, !strings.Contains(got, "PRIVATE KEY") && !strings.Contains(got, mockenv.MTLSToken), "get_api leaks a secret: %s", got)
		var info struct{ Requests []string }
		_ = json.Unmarshal([]byte(got), &info)
		expect(t, len(info.Requests) == 1, "get_api: %s", clip(got, 300))
		sent := ad.must("send_request", map[string]any{"api": api, "request": strings.Fields(info.Requests[0])[0]})
		expect(t, strings.Contains(sent, `"status":200`) && strings.Contains(sent, "client_cn"), "send_request over mTLS: %s", clip(sent, 300))
	})

	add("parity", "admin MCP onboards a login API end to end with no UI or CLI", func(t *testing.T) {
		api, kitName := strings.ReplaceAll(uniq("adminShop"), "_", ""), strings.ReplaceAll(uniq("admin-orders"), "_", "-")
		ad := h.adminFor(t, "Onboarding agent", "SHOP_PASSWORD="+mockenv.ShopPass)
		ad.must("import_api", map[string]any{"source": h.env.URLs["shop"] + "/openapi.json", "api": api})
		ad.must("set_variable", map[string]any{"api": api, "name": "email", "value": mockenv.ShopUser})
		ad.must("secret_from_env", map[string]any{"api": api, "name": "password", "env": "SHOP_PASSWORD"})
		ad.must("update_request", map[string]any{"api": api, "request": "login", "body": `{"email": "{{email}}", "password": "{{password}}"}`})
		ad.must("set_auth", map[string]any{"api": api, "auth": map[string]any{"type": "login",
			"steps": []any{map[string]any{"requestId": "login", "captures": []any{map[string]any{"name": "token", "from": "json:access_token"}}}},
			"apply": []any{map[string]any{"key": "Authorization", "value": "Bearer {{login.token}}"}}}})
		sent := ad.must("send_request", map[string]any{"api": api, "request": "listOrders"})
		expect(t, strings.Contains(sent, `"status":200`) && strings.Contains(sent, "ord_0001"), "send_request: %s", clip(sent, 300))
		ad.must("create_toolkit", map[string]any{"name": kitName, "apis": []any{api}})
		gen := ad.must("generate_tool", map[string]any{"toolkit": kitName, "from": api + ".listOrders", "name": "recent_orders", "description": "Recent orders, newest first, ten per page."})
		expect(t, strings.Contains(gen, "live"), "a generated read tool should go live: %s", clip(gen, 300))
		lint := ad.must("lint", map[string]any{"toolkit": kitName})
		expect(t, strings.Contains(lint, "findings"), "lint: %s", clip(lint, 200))
		var c struct{ URL, Token string }
		_ = json.Unmarshal([]byte(ad.must("connect_client", map[string]any{"toolkit": kitName, "client": "other", "name": "scenario reader"})), &c)
		expect(t, c.Token != "", "connect_client gave no token")
		out := h.agentWithToken(t, kitName, c.Token, "").ok("recent_orders", map[string]any{})
		expect(t, strings.Contains(asJSON(out), "ord_"), "got %s", clip(asJSON(out), 200))
	})

	add("parity", "admin MCP onboards OAuth 2.0 behind a company CA", func(t *testing.T) {
		api := strings.ReplaceAll(uniq("adminBilling"), "_", "")
		ad := h.adminFor(t, "Onboarding agent", "BILLING_SECRET="+mockenv.OAuthSecret)
		ad.must("import_api", map[string]any{"source": h.env.URLs["billing"] + "/openapi.json", "api": api, "ca_pem": string(h.env.CACert)})
		ad.must("set_variable", map[string]any{"api": api, "name": "clientId", "value": mockenv.OAuthClientID})
		ad.must("secret_from_env", map[string]any{"api": api, "name": "clientSecret", "env": "BILLING_SECRET"})
		ad.must("set_auth", map[string]any{"api": api, "auth": map[string]any{"type": "oauth2", "grant": "client_credentials",
			"tokenUrl": h.env.URLs["idp"] + "/oauth2/token", "clientId": "{{clientId}}", "clientSecret": "{{clientSecret}}", "scope": "billing.read"}})
		sent := ad.must("send_request", map[string]any{"api": api, "request": "listInvoices"})
		expect(t, strings.Contains(sent, `"status":200`), "send_request: %s", clip(sent, 300))
	})

	add("parity", "admin MCP write tool waits for a person; the agent can't approve it", func(t *testing.T) {
		need(t, "shop")
		k := kit(t, "shop")
		ad := h.adminFor(t, "Builder agent")
		_, isErr := ad.call("generate_tool", map[string]any{"toolkit": k, "from": "shop.cancelOrder"})
		expect(t, isErr, "a write tool was generated without allow_write")
		text := ad.must("generate_tool", map[string]any{"toolkit": k, "from": "shop.cancelOrder", "name": "cancel_order", "description": "Cancel an order that has not shipped yet.", "allow_write": true})
		expect(t, strings.Contains(text, "pending"), "an agent's write tool went live: %s", clip(text, 300))
		var tk struct {
			PendingList []struct{ ID, Name string } `json:"pendingList"`
		}
		h.mustAPI(t, "GET", "/api/toolkits/"+k, nil, &tk)
		expect(t, len(tk.PendingList) == 1, "pending: %+v", tk.PendingList)
		err := h.apiAs("Builder agent", "POST", "/api/versions/"+tk.PendingList[0].ID+"/approve", nil, nil)
		expect(t, err != nil && strings.Contains(err.Error(), "403"), "an agent approved its own write tool: %v", err)
		h.mustCLI(t, "tool", "approve", k, "cancel_order")
		reviews := ad.must("pending_reviews", map[string]any{"toolkit": k})
		expect(t, !strings.Contains(reviews, "cancel_order"), "still pending after a person approved: %s", reviews)
	})

	add("parity", "admin MCP can't widen access or see secret values", func(t *testing.T) {
		need(t, "shop")
		k := kit(t, "shop")
		err := h.apiAs("Builder agent", "POST", "/api/connect", map[string]any{"client": "other", "toolkitId": k, "access": "read_write"}, nil)
		expect(t, err != nil && strings.Contains(err.Error(), "403"), "an agent connected a client with write access: %v", err)
		ad := h.adminFor(t, "Builder agent")
		_, isErr := ad.call("set_variable", map[string]any{"api": "shop", "name": "api_token", "value": "x"})
		expect(t, isErr, "set_variable accepted a secret by value")
		text := ad.must("get_api", map[string]any{"api": "shop"})
		h.noLeaks(t, "get_api", text)
		expect(t, strings.Contains(text, "password (secret)"), "get_api should list the secret by name: %s", clip(text, 300))
	})

	add("parity", "admin MCP actions are audited under the agent's name", func(t *testing.T) {
		ad := h.adminFor(t, "Audit probe agent")
		ad.must("create_toolkit", map[string]any{"name": strings.ReplaceAll(uniq("audit-kit"), "_", "-"), "apis": []any{}})
		var log []map[string]any
		h.mustAPI(t, "GET", "/api/audit", nil, &log)
		found := false
		for _, e := range log {
			if strings.Contains(asJSON(e), "Audit probe agent (admin MCP)") {
				found = true
			}
		}
		expect(t, found, "no audit entry names the admin agent")
	})

	add("parity", "admin MCP context an agent reads stays small", func(t *testing.T) {
		ad := h.adminFor(t, "Size probe")
		res, err := ad.cs.ListTools(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(res.Tools)
		n := tokens.Count(string(b)) + tokens.Count(ad.cs.InitializeResult().Instructions)
		t.Logf("admin MCP: %d tools, %d tokens with instructions", len(res.Tools), n)
		expect(t, n < 2500, "admin MCP costs %d tokens to load; keep it under 2500", n)
	})
}
