package audit

import (
	"context"
	"testing"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// REQ: AUD-2
func TestVerify_DetectsEditedRow(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := clock.NewFake(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	l := New(s, c, &clock.Seq{Prefix: "e"})
	for _, a := range []string{"api.create", "toolkit.create", "agent.revoke"} {
		if err := l.Append(ctx, You, a, "x", ""); err != nil {
			t.Fatal(err)
		}
	}
	r, _ := l.Verify(ctx)
	if !r.OK || r.Events != 3 {
		t.Fatalf("want intact chain, got %+v", r)
	}
	// Someone bypasses the triggers and edits a row.
	if _, err := s.DB.ExecContext(ctx, `DROP TRIGGER audit_no_update`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.ExecContext(ctx, `UPDATE audit_event SET target = 'y' WHERE seq = 2`); err != nil {
		t.Fatal(err)
	}
	r, _ = l.Verify(ctx)
	if r.OK || r.BadSeq != 2 {
		t.Fatalf("want broken at 2, got %+v", r)
	}
}
