package mock

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/vault"
)

type shippedTool struct {
	Origin string `json:"origin"`
	Source string `json:"source"`
}

const shippedFixture = "../../web/src/test/fixtures/shipped-tools.json"

// shippedTools lists every tool source mcpDeck ships or writes: the demo's
// tools, the examples, the skill's sample tools and the starter tool the
// generator makes for every demo request. The web tests round-trip each one
// through the visual builder (FLOW-1).
func shippedTools(t *testing.T) []shippedTool {
	t.Helper()
	out := []shippedTool{
		{"demo/customer_360", customer360},
		{"demo/open_tickets_for_account", openTickets},
		{"demo/invoice_status", invoiceStatus},
		{"demo/reassign_ticket", reassignTicket},
		{"test/scanner_flow", scanFlow},
	}
	for _, glob := range []string{"../../examples/*/*/*.ts", "../../skills/testdata/*.ts"} {
		files, _ := filepath.Glob(glob)
		sort.Strings(files)
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			rel, _ := filepath.Rel("../..", f)
			out = append(out, shippedTool{filepath.ToSlash(rel), string(b)})
		}
	}
	ctx := context.Background()
	w, err := Start(0)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s, _ := store.Open(ctx, t.TempDir())
	defer s.Close()
	v, _ := vault.Open(vault.NoKeychain{}, t.TempDir())
	c := clock.Real{}
	ids := clock.ULIDs{Clock: c}
	d := deck.New(s, v, proxy.New(s, v, c, ids, nil), audit.New(s, c, ids), c, ids, t.TempDir(), 47801)
	if err := Seed(ctx, d, w, true); err != nil {
		t.Fatal(err)
	}
	apis, _ := d.ListAPIs(ctx)
	sort.Slice(apis, func(i, j int) bool { return apis[i].Name < apis[j].Name })
	for _, a := range apis {
		det, err := d.GetAPI(ctx, a.ID)
		if err != nil {
			t.Fatal(err)
		}
		reqs := det.Requests
		sort.Slice(reqs, func(i, j int) bool { return reqs[i].Name < reqs[j].Name })
		for _, r := range reqs {
			g, err := d.GenerateTool(ctx, deck.GenerateInput{From: a.Name + "." + r.Name})
			if err != nil && strings.Contains(err.Error(), "can't be a tool") {
				continue // login requests aren't tools
			}
			if err != nil {
				t.Fatalf("generate %s.%s: %v", a.Name, r.Name, err)
			}
			out = append(out, shippedTool{"generated/" + a.Name + "." + r.Name, g.Source})
		}
	}
	return out
}

// REQ: FLOW-1
func TestShippedToolsFixtureIsCurrent(t *testing.T) {
	b, err := json.MarshalIndent(shippedTools(t), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	if os.Getenv("UPDATE_FIXTURES") == "1" {
		if err := os.WriteFile(shippedFixture, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	cur, err := os.ReadFile(shippedFixture)
	if err != nil || string(cur) != string(b) {
		t.Fatal("web/src/test/fixtures/shipped-tools.json is out of date; run: UPDATE_FIXTURES=1 go test ./internal/mock -run TestShippedToolsFixtureIsCurrent")
	}
}
