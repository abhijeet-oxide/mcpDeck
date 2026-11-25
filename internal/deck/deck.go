// Package deck is mcpDeck's application layer. The HTTP API, the MCP
// endpoints and the CLI all go through it, so every action has one
// implementation (RUN-6).
package deck

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/vault"
)

// Error is an error with an HTTP status and a message that says what to do.
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

func errf(status int, code, format string, a ...any) *Error {
	return &Error{Status: status, Code: code, Message: fmt.Sprintf(format, a...)}
}

func badRequest(format string, a ...any) *Error {
	return errf(http.StatusBadRequest, "bad_request", format, a...)
}
func notFound(what string) *Error {
	return errf(http.StatusNotFound, "not_found", "%s not found", what)
}
func conflict(format string, a ...any) *Error {
	return errf(http.StatusConflict, "conflict", format, a...)
}

// StatusOf maps any error to an HTTP status.
func StatusOf(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.Status
	}
	if errors.Is(err, store.ErrNotFound) {
		return http.StatusNotFound
	}
	var pe *proxy.Error
	if errors.As(err, &pe) {
		switch pe.Code {
		case proxy.CodeWriteBlocked, proxy.CodeHostNotAllowed:
			return http.StatusForbidden
		case proxy.CodeNotFound:
			return http.StatusNotFound
		}
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

// Listener is told when things agents see have changed.
type Listener interface {
	ToolsChanged(toolkitIDs []string)
	AgentRevoked(agentID string)
}

// Deck holds the dependencies.
type Deck struct {
	Store   *store.Store
	Vault   *vault.Vault
	Proxy   *proxy.Proxy
	Audit   *audit.Log
	Clock   clock.Clock
	IDs     clock.IDs
	DataDir string
	Port    int
	Demo    bool

	mu        sync.RWMutex
	listeners []Listener
}

// New wires a Deck.
func New(s *store.Store, v *vault.Vault, p *proxy.Proxy, a *audit.Log, c clock.Clock, ids clock.IDs, dataDir string, port int) *Deck {
	return &Deck{Store: s, Vault: v, Proxy: p, Audit: a, Clock: c, IDs: ids, DataDir: dataDir, Port: port}
}

// Listen registers a listener (the MCP manager).
func (d *Deck) Listen(l Listener) {
	d.mu.Lock()
	d.listeners = append(d.listeners, l)
	d.mu.Unlock()
}

func (d *Deck) toolsChanged(ids []string) {
	d.mu.RLock()
	ls := append([]Listener{}, d.listeners...)
	d.mu.RUnlock()
	for _, l := range ls {
		l.ToolsChanged(ids)
	}
}

func (d *Deck) agentRevoked(id string) {
	d.mu.RLock()
	ls := append([]Listener{}, d.listeners...)
	d.mu.RUnlock()
	for _, l := range ls {
		l.AgentRevoked(id)
	}
}

func (d *Deck) audit(ctx context.Context, a audit.Actor, action, target, detail string) {
	_ = d.Audit.Append(context.WithoutCancel(ctx), a, action, target, detail)
}

// BaseURL is where mcpDeck listens.
func (d *Deck) BaseURL() string { return fmt.Sprintf("http://127.0.0.1:%d", d.Port) }

// Endpoint is a toolkit's MCP URL; build is the builder endpoint.
func (d *Deck) Endpoint(toolkit string, build bool) string {
	u := d.BaseURL() + "/mcp/" + toolkit
	if build {
		u += "/build"
	}
	return u
}

var slugRe = strings.NewReplacer(" ", "-", "_", "-")

// Slug makes a toolkit name: lowercase letters, digits and dashes.
func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.Replace(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}
