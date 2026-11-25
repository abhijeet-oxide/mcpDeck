// Package audit is the append-only, hash-chained audit log (AUD-1, AUD-2).
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// Actor is who made a change.
type Actor struct {
	Kind string `json:"kind"` // you | agent | system
	ID   string `json:"id"`
	Name string `json:"name"`
}

// You is the local person using the UI or CLI.
var You = Actor{Kind: "you", Name: "You"}

// Log appends and verifies audit events.
type Log struct {
	store *store.Store
	clock clock.Clock
	ids   clock.IDs
}

// New returns a log over the store.
func New(s *store.Store, c clock.Clock, ids clock.IDs) *Log {
	return &Log{store: s, clock: c, ids: ids}
}

// Hash is the chained hash of an event.
func Hash(e store.AuditEvent) string {
	h := sha256.New()
	for _, p := range []string{e.PrevHash, e.ID, e.At.UTC().Format(time.RFC3339Nano), e.ActorKind, e.ActorID, e.ActorName, e.Action, e.Target, e.Detail} {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Append records one event. Details must never contain secret values.
func (l *Log) Append(ctx context.Context, a Actor, action, target, detail string) error {
	tx, err := l.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	last, ok, err := l.store.LastAudit(ctx, tx)
	if err != nil {
		return err
	}
	prev := ""
	if ok {
		prev = last.Hash
	}
	e := store.AuditEvent{ID: l.ids.New(), At: l.clock.Now(), ActorKind: a.Kind, ActorID: a.ID, ActorName: a.Name,
		Action: action, Target: target, Detail: detail, PrevHash: prev}
	e.Hash = Hash(e)
	if err := l.store.InsertAudit(ctx, tx, e); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return tx.Commit()
}

// Result of verifying the chain.
type Result struct {
	OK      bool   `json:"ok"`
	Events  int    `json:"events"`
	BadSeq  int64  `json:"badSeq,omitempty"`
	Message string `json:"message"`
}

// Verify walks the chain and reports the first broken link.
func (l *Log) Verify(ctx context.Context) (Result, error) {
	events, err := l.store.AuditEvents(ctx, true, 0)
	if err != nil {
		return Result{}, err
	}
	prev := ""
	for _, e := range events {
		if e.PrevHash != prev || Hash(e) != e.Hash {
			return Result{OK: false, Events: len(events), BadSeq: e.Seq,
				Message: fmt.Sprintf("entry %d (%s) doesn't match the chain: the log was edited", e.Seq, strings.TrimSpace(e.Action))}, nil
		}
		prev = e.Hash
	}
	return Result{OK: true, Events: len(events), Message: fmt.Sprintf("%d entries, chain intact", len(events))}, nil
}
