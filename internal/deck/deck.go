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

// Forbidden is an action this caller may not take, with what to do instead.
func Forbidden(msg string) *Error { return errf(http.StatusForbidden, "forbidden", "%s", msg) }

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
	// Practice is the demo scanner API to walk the builder flow with (MOCK-6).
	Practice *Practice

	// OpenConfirm shows the person a write waiting for them (CONF-1), e.g.
	// by opening the confirm page in their browser. Nil leaves it to the
	// Home banner.
	OpenConfirm func(id string)

	mu        sync.RWMutex
	listeners []Listener
	conf      *confirmations
	recording map[string]context.CancelFunc // API name → stop a recording (REC-1)
}

// Practice is a mock API for trying the full flow: import, log in, chain.
type Practice struct {
	SpecURL  string `json:"specUrl"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// New wires a Deck.
func New(s *store.Store, v *vault.Vault, p *proxy.Proxy, a *audit.Log, c clock.Clock, ids clock.IDs, dataDir string, port int) *Deck {
	d := &Deck{Store: s, Vault: v, Proxy: p, Audit: a, Clock: c, IDs: ids, DataDir: dataDir, Port: port}
	d.ApplyNetwork(context.Background())
	return d
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
	if over, ok := ctx.Value(actorKey{}).(audit.Actor); ok && a.Kind == "you" {
		a = over
	}
	_ = d.Audit.Append(context.WithoutCancel(ctx), a, action, target, detail)
}

type actorKey struct{}

// WithActor records actions in ctx as a's instead of the person's, for the
// agent behind the admin MCP server (PAR-2).
func WithActor(ctx context.Context, a audit.Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
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
