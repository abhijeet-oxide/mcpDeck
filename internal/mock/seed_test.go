package mock

import (
	"context"
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/vault"
)

// REQ: MOCK-3, TOOL-1, CRED-1
func TestSeed_DemoWorldRunsTheTool1Example(t *testing.T) {
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
	d := deck.New(s, v, proxy.New(s, v, c, ids, nil), audit.New(s, c, ids), c, ids, t.TempDir(), 47800)
	if err := Seed(ctx, d, w, true); err != nil {
		t.Fatal(err)
	}
	tk, _ := d.Store.ToolkitByName(ctx, "support")
	tool, _ := d.Store.ToolByName(ctx, "customer_360")
	v360, err := d.Store.LiveVersion(ctx, tool.ID)
	if err != nil {
		t.Fatalf("customer_360 should be live: %v", err)
	}
	r, _ := d.TestTool(ctx, deck.Caller{Actor: audit.You, Toolkit: &tk}, deck.TestInput{Source: v360.Source, Input: map[string]any{"accountId": "001Ab00000NWTRD"}})
	if r.Error != "" || !strings.Contains(r.OutputJSON, `"tier":"gold"`) || !strings.Contains(r.OutputJSON, "INV-2087") || !strings.Contains(r.OutputJSON, "NWT-482") {
		t.Fatalf("customer_360: %s %s", r.OutputJSON, r.Error)
	}
	if r.UpstreamBytes <= r.OutputBytes*3 {
		t.Fatalf("the tool should return far less than the APIs did: %d vs %d", r.OutputBytes, r.UpstreamBytes)
	}
	// Legacy cookie login through a tool with no login code (CRED-1).
	lr := d.RunScript(ctx, deck.Caller{Actor: audit.You}, `return await legacy.getCreditHold({ account: "001Ab00000FBRKM" })`, nil)
	if lr.Error != "" || !strings.Contains(lr.OutputJSON, `"CREDIT_HOLD":true`) {
		t.Fatalf("legacy: %s %s", lr.OutputJSON, lr.Error)
	}
	pend, _ := d.Store.PendingVersions(ctx)
	if len(pend) != 1 || pend[0].Name != "reassign_ticket" {
		t.Fatalf("want reassign_ticket pending, got %+v", pend)
	}
	h, _ := d.Home(ctx)
	if h.CallsToday == 0 || len(h.NeedsYou) < 2 {
		t.Fatalf("home: %+v", h)
	}
	// No demo secret is stored in plain text anywhere.
	rows, _ := s.DB.QueryContext(ctx, `SELECT data FROM call UNION ALL SELECT data FROM variable UNION ALL SELECT data FROM request UNION ALL SELECT detail FROM audit_event`)
	defer rows.Close()
	for rows.Next() {
		var x string
		_ = rows.Scan(&x)
		for _, sec := range []string{CRMToken, BillingKey, SupportToken, LegacyPass} {
			if strings.Contains(x, sec) {
				t.Fatalf("secret %s stored in plain text: %.200s", sec, x)
			}
		}
	}
}
