package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// REQ: RUN-3
func TestStore_RoundTripsDocuments(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := API{ID: "a1", Name: "crm", Title: "Acme CRM", CreatedAt: time.Now().UTC()}
	if err := s.PutAPI(ctx, a); err != nil {
		t.Fatal(err)
	}
	got, err := s.APIByName(ctx, "crm")
	if err != nil || got.Title != "Acme CRM" {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := s.API(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	ag := Agent{ID: "ag1", Name: "Claude Code", TokenHash: "h1"}
	if err := s.PutAgent(ctx, ag); err != nil {
		t.Fatal(err)
	}
	back, err := s.AgentByTokenHash(ctx, "h1")
	if err != nil || back.ID != "ag1" || back.TokenHash != "h1" {
		t.Fatalf("agent by hash: %+v %v", back, err)
	}
}

// REQ: AUD-1
func TestStore_AuditRowsCannotBeChanged(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	tx, _ := s.DB.BeginTx(ctx, nil)
	if err := s.InsertAudit(ctx, tx, AuditEvent{ID: "e1", At: time.Now(), ActorKind: "you", Action: "api.create", Hash: "x"}); err != nil {
		t.Fatal(err)
	}
	_ = tx.Commit()
	if _, err := s.DB.ExecContext(ctx, `UPDATE audit_event SET action = 'x'`); err == nil {
		t.Fatal("update of audit row should fail")
	}
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM audit_event`); err == nil {
		t.Fatal("delete of audit row should fail")
	}
}
