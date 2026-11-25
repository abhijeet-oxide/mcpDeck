package tool

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

type fakeSender struct {
	mu    sync.Mutex
	calls []proxy.Call
	delay time.Duration
	reply func(c proxy.Call) (*proxy.Response, error)
}

func (f *fakeSender) Do(_ context.Context, c proxy.Call) (*proxy.Response, error) {
	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.mu.Unlock()
	time.Sleep(f.delay)
	if f.reply != nil {
		return f.reply(c)
	}
	b, _ := json.Marshal(map[string]any{"id": c.Args.Flat["id"], "name": "Northwind", "tier": "gold", "billingId": "B-1"})
	return &proxy.Response{Status: 200, Body: b, Size: len(b)}, nil
}

func env(s Sender) Env {
	return Env{Sender: s, Timeout: 2 * time.Second, Clients: []Client{
		{API: store.API{ID: "a1", Name: "crm"}, Requests: []store.Request{{ID: "r1", Name: "getAccount"}}},
		{API: store.API{ID: "a2", Name: "billing"}, Requests: []store.Request{{ID: "r2", Name: "getSummary"}}},
	}}
}

const goodTool = `export const tool = {
  name: "customer_360",
  description: "One view of a customer.",
  inputSchema: { type: "object", properties: { accountId: { type: "string" } }, required: ["accountId"] },
};
export default async function (input: { accountId: string }) {
  const account = await crm.getAccount({ id: input.accountId });
  const [a, b] = await Promise.all([billing.getSummary({ id: account.billingId }), crm.getAccount({ id: "x" })]);
  console.log("tier", account.tier);
  return { name: account?.name, tier: account.tier, both: [a.name, b.name].length, ...{ ok: true } };
}`

// REQ: TOOL-1, TOOL-2
func TestRun_ToolCallsClientsAndReturnsOutput(t *testing.T) {
	d, err := Load(goodTool)
	if err != nil || d.Name != "customer_360" {
		t.Fatalf("load: %+v %v", d, err)
	}
	f := &fakeSender{delay: 30 * time.Millisecond}
	r := Run(context.Background(), goodTool, map[string]any{"accountId": "001"}, env(f))
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	if r.OutputJSON != `{"both":2,"name":"Northwind","ok":true,"tier":"gold"}` {
		t.Fatalf("output %s", r.OutputJSON)
	}
	if len(r.Calls) != 3 || len(r.Console) != 1 || r.OutputTokens == 0 {
		t.Fatalf("calls %d console %v tokens %d", len(r.Calls), r.Console, r.OutputTokens)
	}
	// Promise.all ran the two calls together: 3 calls of 30ms take ~60ms, not 90.
	if r.DurationMS > 85 {
		t.Fatalf("Promise.all calls should run in parallel, took %dms", r.DurationMS)
	}
}

// REQ: TOOL-4
func TestSandbox_EscapeSuite(t *testing.T) {
	cases := map[string]string{
		"filesystem":  `return require("fs").readFileSync("/etc/passwd")`,
		"process":     `return process.env`,
		"network":     `return await fetch("http://example.com")`,
		"timers":      `return setTimeout(() => 1, 10)`,
		"eval":        `return eval("1+1")`,
		"function":    `return new Function("return 1")()`,
		"ctor escape": `return (function(){}).constructor("return this")()`,
		"async ctor":  `return (async function(){}).constructor("return 1")`,
		"loop":        `while (true) {}`,
		"recursion":   `const f = (n) => f(n + 1); return f(0)`,
		"huge string": `return "x".repeat(1e9).length`,
		"redos":       `return /^(a+)+$/.test("a".repeat(40) + "!")`,
		"memory bomb": `const a = []; for (;;) a.push(new Array(1e6).fill(1));`,
		"import":      `const m = await import("fs"); return 1`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			e := env(&fakeSender{})
			e.Timeout = 1500 * time.Millisecond
			start := time.Now()
			r := RunScript(context.Background(), body, nil, e)
			if r.Error == "" {
				t.Fatalf("expected an error, got output %s", r.OutputJSON)
			}
			if time.Since(start) > 5*time.Second {
				t.Fatalf("took too long: %v", time.Since(start))
			}
		})
	}
}

// REQ: TOOL-4
func TestSandbox_PrototypePollutionStaysInRun(t *testing.T) {
	e := env(&fakeSender{})
	_ = RunScript(context.Background(), `Object.prototype.polluted = 1; return 1`, nil, e)
	r := RunScript(context.Background(), `return ({}).polluted === undefined`, nil, e)
	if r.OutputJSON != "true" {
		t.Fatalf("each run must start fresh, got %s %s", r.OutputJSON, r.Error)
	}
}

// REQ: TOOL-2
func TestRun_NonOKThrowsApiError(t *testing.T) {
	f := &fakeSender{reply: func(proxy.Call) (*proxy.Response, error) {
		return &proxy.Response{Status: 404, Body: []byte(`{"error":"no such account"}`)}, nil
	}}
	r := RunScript(context.Background(), `try { await crm.getAccount({ id: "9" }) } catch (e) { return { status: e.status, name: e.name, body: e.body } }`, nil, env(f))
	if r.OutputJSON != `{"body":{"error":"no such account"},"name":"ApiError","status":404}` {
		t.Fatalf("got %s %s", r.OutputJSON, r.Error)
	}
}

// REQ: ACC-2
func TestRun_BlockedWriteSurfacesProxyCode(t *testing.T) {
	f := &fakeSender{reply: func(proxy.Call) (*proxy.Response, error) {
		return nil, &proxy.Error{Code: proxy.CodeWriteBlocked, Message: "write blocked: ..."}
	}}
	r := RunScript(context.Background(), `return await crm.getAccount({ id: "9" })`, nil, env(f))
	if r.ErrorCode != proxy.CodeWriteBlocked {
		t.Fatalf("got %+v", r)
	}
}

// REQ: TOOL-8
func TestRunTests_PMAssertions(t *testing.T) {
	tests := []store.ToolTest{
		{Name: "returns tier", Input: json.RawMessage(`{"accountId":"001"}`), Script: `pm.test("tier", () => pm.expect(output.tier).to.equal("gold")); pm.test("name", () => pm.expect(output).to.have.property("name"))`},
		{Name: "fails", Input: json.RawMessage(`{"accountId":"001"}`), Script: `pm.test("wrong", () => pm.expect(output.tier).to.equal("silver"))`},
	}
	res := RunTests(context.Background(), goodTool, tests, env(&fakeSender{}))
	if !res[0].Passed || res[1].Passed || !strings.Contains(res[1].Error, "silver") {
		t.Fatalf("got %+v", res)
	}
}

// REQ: TOOL-1
func TestLoad_RejectsBadDefinitions(t *testing.T) {
	for _, src := range []string{
		`export default async function () {}`,
		`export const tool = { name: "Bad Name", description: "x" }; export default async function () {}`,
		`export const tool = { name: "ok_name", description: "" }; export default async function () {}`,
		`export const tool = { name: "ok_name", description: "d" };`,
		`export const tool = { name: "ok_name", description: "d", inputSchema: { type: "string" } }; export default async function () {}`,
	} {
		if _, err := Load(src); err == nil {
			t.Fatalf("want error for %s", src)
		}
	}
}

func TestValidateInput(t *testing.T) {
	d, _ := Load(goodTool)
	if err := ValidateInput(d.InputSchema, map[string]any{}); err == nil || !strings.Contains(err.Error(), "accountId") {
		t.Fatalf("missing required should fail naming the field: %v", err)
	}
	if err := ValidateInput(d.InputSchema, map[string]any{"accountId": "1"}); err != nil {
		t.Fatal(err)
	}
}

func TestUsedCalls(t *testing.T) {
	calls, apis := UsedCalls(goodTool+`; call("crm", "GET /owners/{id}", {})`, env(nil).Clients)
	if strings.Join(calls, ",") != "billing.getSummary,crm GET /owners/{id},crm.getAccount" || strings.Join(apis, ",") != "billing,crm" {
		t.Fatalf("got %v %v", calls, apis)
	}
}

// REQ: FLOW-3
func TestRun_PickKeepsOnlyChosenFields(t *testing.T) {
	src := `export const tool = { name: "vulns", description: "x", inputSchema: { type: "object" } };
export default async function () {
  const v = { total: 2, items: [{ id: "a", sev: "high", cvss: { score: 9, vector: "x" }, long: "…" }, { id: "b", sev: "low", cvss: { score: 2 } }] };
  return { picked: pick(v, ["total", "items[].id", "items[].cvss.score"]), arr: pick([{ a: 1, b: 2 }], ["[].a"]), none: pick(v, []).total };
}`
	r := Run(context.Background(), src, map[string]any{}, env(&fakeSender{}))
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	b, _ := json.Marshal(r.Output)
	want := `{"arr":[{"a":1}],"none":2,"picked":{"items":[{"cvss":{"score":9},"id":"a"},{"cvss":{"score":2},"id":"b"}],"total":2}}`
	if string(b) != want {
		t.Fatalf("got  %s\nwant %s", b, want)
	}
}
