package scenarios

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/abhijeet-oxide/mcpdeck/internal/tokens"
)

// Agents that use the tools: cheap discovery, asking for what's missing
// without gaining access, and outputs that fit their context.

func init() {
	add("consumer", "a 30-tool toolkit lists 15; search_tools and use_tool reach the rest", func(t *testing.T) {
		need(t, "billing")
		k := kit(t, "billing")
		var names []string
		for i := 0; i < 30; i++ {
			n := fmt.Sprintf("invoice_view_%02d_%s", i, uniq("v"))
			names = append(names, n)
			h.saveTool(t, k, toolSrc(n, fmt.Sprintf("Invoice report variant %d: counts invoices for dashboard %d.", i, i), "", nil,
				`  return (await billing.listInvoices({ limit: 2 })).data.length;`))
		}
		a := h.agentFor(t, k, "read")
		listed := a.tools()
		seen := map[string]bool{}
		for _, n := range listed {
			seen[n] = true
		}
		expect(t, len(listed) < 25 && seen["search_tools"] && seen["use_tool"], "tools/list has %d tools: %v", len(listed), listed)
		hidden := ""
		for i, n := range names {
			if !seen[n] {
				hidden = n
				text, _ := a.call("search_tools", map[string]any{"query": fmt.Sprintf("dashboard %d", i)})
				expect(t, strings.Contains(text, n), "search_tools didn't find %s: %s", n, clip(text, 300))
				break
			}
		}
		expect(t, hidden != "", "every tool was listed")
		text, isErr := a.call("use_tool", map[string]any{"name": hidden, "input": map[string]any{}})
		expect(t, !isErr && strings.TrimSpace(text) == "2", "use_tool: %s", clip(text, 200))
	})

	add("consumer", "request_tool records the need but grants nothing", func(t *testing.T) {
		need(t, "shop")
		k := kit(t, "shop")
		w := uniq("t")
		h.saveTool(t, k, toolSrc(w, "Cancels one order that hasn't shipped yet.", `order_id: { type: "string", description: "ord_..." }`, []string{"order_id"},
			`  return (await shop.cancelOrder({ id: input.order_id })).status;`))
		a := h.agentFor(t, k, "read")
		for _, n := range a.tools() {
			expect(t, n != w, "a read agent sees the write tool %s", w)
		}
		desc := "Cancel order " + uniq("o") + " for the customer; I only have read tools."
		text, isErr := a.call("request_tool", map[string]any{"description": desc, "tried": "search_tools 'cancel'"})
		expect(t, !isErr && strings.Contains(text, "Request"), "request_tool: %s", text)
		var home struct {
			NeedsYou []struct{ Kind, Detail string } `json:"needsYou"`
		}
		h.mustAPI(t, "GET", "/api/home", nil, &home)
		found := false
		for _, n := range home.NeedsYou {
			found = found || n.Kind == "request" && n.Detail == desc
		}
		expect(t, found, "the request isn't on Home for a person")
		a.fails("use_tool", map[string]any{"name": w, "input": map[string]any{"order_id": "ord_0003"}})
		for _, n := range a.tools() {
			expect(t, n != w, "asking for a tool widened access")
		}
	})

	add("consumer", "a read builder's write tool waits for a person and stays out of its reach", func(t *testing.T) {
		need(t, "shop")
		k := kit(t, "shop")
		b := h.builderFor(t, k, "read")
		w := uniq("t")
		text, _ := b.call("save_tool", map[string]any{"source": toolSrc(w, "Cancels one order that hasn't shipped yet.", `order_id: { type: "string", description: "ord_..." }`, []string{"order_id"},
			`  return (await shop.cancelOrder({ id: input.order_id })).status;`),
			"tests": []any{map[string]any{"name": "example", "input": map[string]any{"order_id": "ord_0003"}}}})
		expect(t, strings.Contains(text, "pending"), "write tool from a read builder: %s", clip(text, 300))
		var tk struct {
			ToolRows []struct {
				Name   string `json:"name"`
				Latest struct {
					ID string `json:"id"`
				} `json:"latest"`
			} `json:"toolRows"`
		}
		h.mustAPI(t, "GET", "/api/toolkits/"+k, nil, &tk)
		for _, r := range tk.ToolRows {
			if r.Name == w {
				h.mustAPI(t, "POST", "/api/versions/"+r.Latest.ID+"/approve", nil, nil)
			}
		}
		use := h.agentFor(t, k, "read")
		use.fails("use_tool", map[string]any{"name": w, "input": map[string]any{"order_id": "ord_0003"}})
	})

	add("consumer", "an output over the token budget is cut to fit, with a hint", func(t *testing.T) {
		k := kit(t)
		n := uniq("t")
		h.saveTool(t, k, toolSrc(n, "Lists every audit entry with its note, for a full export.", "", nil,
			`  return { total: 6000, entries: Array.from({ length: 6000 }, (_, i) => ({ id: "e" + i, note: "routine check passed on node " + i })) };`))
		a := h.agentFor(t, k, "read")
		text, isErr := a.call(n, map[string]any{})
		expect(t, !isErr, "error: %s", clip(text, 200))
		expect(t, tokens.Count(text) <= 20000, "output is %d tokens", tokens.Count(text))
		expect(t, strings.Contains(text, `"_mcpdeck"`) && strings.Contains(text, `"total":6000`) && strings.Contains(text, "limit"), "no truncation note: %s", clip(text[len(text)-400:], 400))
	})

	add("consumer", "server instructions and tools/list stay small", func(t *testing.T) {
		need(t, "shop")
		k := kit(t, "shop")
		h.saveTool(t, k, toolSrc(uniq("t"), "Count all orders for the signed-in customer.", "", nil, `  return (await shop.listOrders({})).meta.total;`))
		tok := h.connect(t, k, "read", false)
		cl := sdk.NewClient(&sdk.Implementation{Name: "size-probe", Version: "1"}, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cs, err := cl.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: h.base + "/mcp/" + k, HTTPClient: &http.Client{Transport: bearerRT{tok, http.DefaultTransport}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()
		instr := cs.InitializeResult().Instructions
		list, err := cs.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		size := tokens.Count(instr) + tokens.Count(asJSON(list.Tools))
		t.Logf("use endpoint: %d tools, %d tokens with instructions", len(list.Tools), size)
		expect(t, instr != "" && size < 1500, "instructions + tools/list = %d tokens", size)
	})
}
