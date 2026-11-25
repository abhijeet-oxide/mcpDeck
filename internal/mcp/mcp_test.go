package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/vault"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type world struct {
	d     *deck.Deck
	srv   *httptest.Server
	token string
	tk    *store.Toolkit
}

func setup(t *testing.T, access string, build bool) *world {
	t.Helper()
	ctx := context.Background()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer canary-9c1" {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/accounts/"):
			_, _ = io.WriteString(w, `{"id":"`+strings.TrimPrefix(r.URL.Path, "/accounts/")+`","name":"Northwind","tier":"gold","notes":"long text","owner":{"email":"o@acme.example"}}`)
		case r.Method == "PATCH":
			_, _ = io.WriteString(w, `{"ok":true}`)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(up.Close)
	s, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	v, _ := vault.Open(&vault.FakeKeychain{}, t.TempDir())
	c := clock.Real{}
	ids := clock.ULIDs{Clock: c}
	d := deck.New(s, v, proxy.New(s, v, c, ids, nil), audit.New(s, c, ids), c, ids, t.TempDir(), 47800)
	spec := `{"openapi":"3.0.3","info":{"title":"CRM","version":"1"},"servers":[{"url":"` + up.URL + `"}],
"components":{"securitySchemes":{"b":{"type":"http","scheme":"bearer"}}},
"paths":{"/accounts/{id}":{"get":{"operationId":"getAccount","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"ok"}}},
"patch":{"operationId":"updateAccount","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"ok"}}}}}}`
	rep, err := d.Import(ctx, deck.ImportInput{ImportSource: deck.ImportSource{Text: spec}, Name: "crm"})
	if err != nil {
		t.Fatal(err)
	}
	api, _ := d.GetAPI(ctx, rep.APIID)
	for _, vv := range api.Variables {
		if vv.Name == "token" {
			_, _ = d.SetVariable(ctx, rep.APIID, deck.SetVariableInput{ID: vv.ID, Name: "token", Source: "secret", Value: "canary-9c1"})
		}
	}
	name := "support"
	scope := []string{"crm"}
	tk, err := d.CreateToolkit(ctx, deck.ToolkitInput{Name: &name, BuildScope: &scope})
	if err != nil {
		t.Fatal(err)
	}
	cr, err := d.Connect(ctx, deck.ConnectInput{Client: "claude-code", ToolkitID: tk.ID, Access: access, CanBuild: build})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp/", New(d))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &world{d: d, srv: srv, token: cr.Token, tk: tk}
}

type bearer struct {
	tok string
	rt  http.RoundTripper
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.tok)
	return b.rt.RoundTrip(r)
}

func connect(t *testing.T, w *world, path string, changed chan struct{}) *sdk.ClientSession {
	t.Helper()
	opts := &sdk.ClientOptions{}
	if changed != nil {
		opts.ToolListChangedHandler = func(context.Context, *sdk.ToolListChangedRequest) {
			select {
			case changed <- struct{}{}:
			default:
			}
		}
	}
	cl := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "1"}, opts)
	cs, err := cl.Connect(context.Background(), &sdk.StreamableClientTransport{Endpoint: w.srv.URL + path,
		HTTPClient: &http.Client{Transport: bearer{w.token, http.DefaultTransport}}}, nil)
	if err != nil {
		t.Fatalf("connect %s: %v", path, err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func call(t *testing.T, cs *sdk.ClientSession, name string, args any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return res.Content[0].(*sdk.TextContent).Text, res.IsError
}

func toolNames(t *testing.T, cs *sdk.ClientSession) []string {
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var n []string
	for _, x := range res.Tools {
		n = append(n, x.Name)
	}
	return n
}

const accountTool = `export const tool = {
  name: "account_tier",
  description: "Name and tier of one account.",
  useWhen: "A question about one account's tier",
  inputSchema: { type: "object", properties: { id: { type: "string" } }, required: ["id"] },
};
export default async function (input: { id: string }) {
  const a = await crm.getAccount({ id: input.id });
  return { name: a.name, tier: a.tier };
}`

// REQ: TK-2, TK-3, MCP-3, MCP-7, AB-1, AB-7
func TestBuilderSubagentBuildsToolForMainAgent(t *testing.T) {
	w := setup(t, "read", true)
	ctx := context.Background()
	changed := make(chan struct{}, 4)
	main := connect(t, w, "/mcp/support", changed)
	init := main.InitializeResult()
	if !strings.Contains(init.Instructions, "mcpdeck-support-builder subagent") {
		t.Fatalf("main instructions must hand building to the subagent:\n%s", init.Instructions)
	}
	before := toolNames(t, main)
	for _, n := range before {
		if n == "search_apis" || n == "save_tool" || n == "run_api" {
			t.Fatalf("build tool %s must not be on the main endpoint: %v", n, before)
		}
	}

	b := connect(t, w, "/mcp/support/build", nil)
	if !strings.Contains(b.InitializeResult().Instructions, "save_tool") {
		t.Fatal("builder instructions must carry the build loop")
	}
	out, isErr := call(t, b, "search_apis", map[string]any{"query": "account"})
	if isErr || !strings.Contains(out, "crm.getAccount({ id })") {
		t.Fatalf("search_apis: %s", out)
	}
	out, isErr = call(t, b, "run_api", map[string]any{"request": "crm.getAccount", "args": map[string]any{"id": "001"}})
	if isErr || !strings.Contains(out, "Northwind") || strings.Contains(out, "canary-9c1") {
		t.Fatalf("run_api: %s", out)
	}
	out, _ = call(t, b, "save_tool", map[string]any{"source": accountTool, "tests": []any{
		map[string]any{"name": "gold", "input": map[string]any{"id": "001"}, "script": `pm.test("tier", () => pm.expect(output.tier).to.equal("gold"))`},
	}})
	if !strings.Contains(out, "is live") {
		t.Fatalf("save_tool should go live: %s", out)
	}
	select {
	case <-changed:
	case <-time.After(3 * time.Second):
		t.Fatal("main agent didn't get tools/list_changed")
	}
	after := toolNames(t, main)
	if !strings.Contains(strings.Join(after, ","), "account_tier") {
		t.Fatalf("new tool missing: %v", after)
	}
	out, isErr = call(t, main, "account_tier", map[string]any{"id": "007"})
	if isErr || out != `{"name":"Northwind","tier":"gold"}` {
		t.Fatalf("account_tier: %s", out)
	}
	_, isErr = call(t, main, "account_tier", map[string]any{})
	if !isErr {
		t.Fatal("missing input must be an error")
	}
	calls, _ := w.d.Store.Calls(ctx, store.CallFilter{Limit: 1000})
	kinds := map[string]int{}
	for _, c := range calls {
		kinds[c.Kind]++
		if strings.Contains(c.Output+c.Input+c.URL+c.Error, "canary-9c1") {
			t.Fatalf("canary in recorded call: %+v", c)
		}
	}
	if kinds["tool"] < 2 || kinds["build"] < 3 || kinds["upstream"] < 2 || kinds["list"] < 1 {
		t.Fatalf("calls not recorded by kind: %v", kinds)
	}
}

// REQ: ACC-2, MCP-4
func TestReadGrantCantWriteThroughBuilder(t *testing.T) {
	w := setup(t, "read", true)
	b := connect(t, w, "/mcp/support/build", nil)
	out, isErr := call(t, b, "run_api", map[string]any{"request": "crm.updateAccount", "args": map[string]any{"id": "1", "tier": "x"}})
	if !isErr || !strings.Contains(out, "write blocked") || !strings.Contains(out, "read+write") {
		t.Fatalf("want actionable write block, got %s", out)
	}
	out, _ = call(t, b, "save_tool", map[string]any{"source": strings.ReplaceAll(strings.ReplaceAll(accountTool, "getAccount", "updateAccount"), "account_tier", "set_tier"), "tests": []any{}})
	if !strings.Contains(out, "pending") {
		t.Fatalf("write tool must wait for a person: %s", out)
	}
}

// REQ: ACC-6, TK-4
func TestNoBuildWithoutGrantAndNoCrossToolkit(t *testing.T) {
	w := setup(t, "read", false)
	req, _ := http.NewRequest("POST", w.srv.URL+"/mcp/support/build", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+w.token)
	res, _ := http.DefaultClient.Do(req)
	if res.StatusCode != 403 {
		t.Fatalf("build without grant: %d", res.StatusCode)
	}
	name := "finance"
	_, _ = w.d.CreateToolkit(context.Background(), deck.ToolkitInput{Name: &name})
	req, _ = http.NewRequest("POST", w.srv.URL+"/mcp/finance", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+w.token)
	res, _ = http.DefaultClient.Do(req)
	if res.StatusCode != 403 {
		t.Fatalf("other toolkit: %d", res.StatusCode)
	}
	req, _ = http.NewRequest("POST", w.srv.URL+"/mcp/support", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+w.token)
	req.Header.Set("Origin", "https://evil.example")
	res, _ = http.DefaultClient.Do(req)
	if res.StatusCode != 403 {
		t.Fatalf("foreign origin: %d", res.StatusCode)
	}
}

// REQ: AGT-4
func TestRevokeRefusesTokenAtOnce(t *testing.T) {
	w := setup(t, "read", false)
	main := connect(t, w, "/mcp/support", nil)
	_ = toolNames(t, main)
	agents, _ := w.d.Store.Agents(context.Background())
	if err := w.d.Revoke(context.Background(), agents[0].ID); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("POST", w.srv.URL+"/mcp/support", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+w.token)
	res, _ := http.DefaultClient.Do(req)
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 401 || !strings.Contains(string(body), "revoked") {
		t.Fatalf("revoked token: %d %s", res.StatusCode, body)
	}
	_ = json.Valid(body)
}

// REQ: ACC-3
func TestLeastExposure_BigToolkitListsTopToolsAndUseTool(t *testing.T) {
	w := setup(t, "read", false)
	ctx := context.Background()
	for i := 0; i < 27; i++ {
		src := strings.ReplaceAll(accountTool, "account_tier", "tier_"+string(rune('a'+i%26))+string(rune('a'+i/26)))
		if _, err := w.d.SaveTool(ctx, "support", deck.Caller{Actor: audit.You}, deck.SaveInput{Source: src}); err != nil {
			t.Fatal(err)
		}
	}
	main := connect(t, w, "/mcp/support", nil)
	names := toolNames(t, main)
	if len(names) != 15+4 || !strings.Contains(strings.Join(names, ","), "use_tool") {
		t.Fatalf("want the 15 most used plus 4 helpers, got %d: %v", len(names), names)
	}
	out, isErr := call(t, main, "use_tool", map[string]any{"name": "tier_ab", "input": map[string]any{"id": "1"}})
	if isErr || !strings.Contains(out, "gold") {
		t.Fatalf("use_tool: %s", out)
	}
	small := setup(t, "read", false)
	names = toolNames(t, connect(t, small, "/mcp/support", nil))
	if strings.Contains(strings.Join(names, ","), "use_tool") {
		t.Fatalf("small toolkits don't need use_tool: %v", names)
	}
}
