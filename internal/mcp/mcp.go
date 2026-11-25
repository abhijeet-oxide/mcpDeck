// Package mcp serves each toolkit as its own MCP server (TK-2).
//
//	/mcp/<toolkit>        published tools + search/request/report: what a
//	                      working agent needs, and nothing that fills its context
//	/mcp/<toolkit>/build  the build tools, for a builder subagent only (MCP-3)
//
// Each (grant, mode) gets its own server and Streamable HTTP handler, so
// sessions never cross agents. The token is checked on every request, so
// revoking takes effect at once (AGT-4).
package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/version"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type entry struct {
	mu      sync.Mutex
	server  *sdk.Server
	handler http.Handler
	sess    deck.AgentSession
	names   []string
}

// Manager routes /mcp/ requests.
type Manager struct {
	deck *deck.Deck
	mu   sync.Mutex
	ents map[string]*entry // grantID|mode
}

// New returns a manager and subscribes it to tool changes.
func New(d *deck.Deck) *Manager {
	m := &Manager{deck: d, ents: map[string]*entry{}}
	d.Listen(m)
	return m
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

// localOrigin allows no Origin (CLI clients) or a localhost one (RUN-5).
func localOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	u, err := url.Parse(o)
	if err != nil {
		return false
	}
	h := u.Hostname()
	return h == "127.0.0.1" || h == "localhost" || h == "::1" || strings.HasPrefix(o, "vscode-") || strings.HasPrefix(o, "vscode-webview://")
}

// BearerToken reads Authorization: Bearer <token>.
func BearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !localOrigin(r) {
		writeErr(w, http.StatusForbidden, "browser pages from other sites can't use mcpDeck")
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/mcp/"), "/")
	toolkit, mode := rest, "use"
	if strings.HasSuffix(rest, "/build") {
		toolkit, mode = strings.TrimSuffix(rest, "/build"), "build"
	}
	if toolkit == "" || strings.Contains(toolkit, "/") {
		writeErr(w, http.StatusNotFound, "use /mcp/<toolkit> or /mcp/<toolkit>/build")
		return
	}
	ctx := r.Context()
	a, g, tk, err := m.deck.Authenticate(ctx, BearerToken(r), toolkit)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer realm="mcpdeck"`)
		writeErr(w, deck.StatusOf(err), err.Error())
		return
	}
	if mode == "build" && !g.CanBuild {
		writeErr(w, http.StatusForbidden, a.Name+" may not build tools in "+tk.Name+"; a person can allow it in mcpDeck (toolkit › Agents › May build)")
		return
	}
	e := m.entry(ctx, a, g, tk, mode, r)
	sid := r.Header.Get("Mcp-Session-Id")
	if sid != "" && r.Method == http.MethodPost {
		go m.deck.SessionSeen(context.WithoutCancel(ctx), a, tk, mode, sid, "", "")
	}
	e.handler.ServeHTTP(w, r)
}

func (m *Manager) entry(ctx context.Context, a store.Agent, g store.Grant, tk store.Toolkit, mode string, r *http.Request) *entry {
	key := g.ID + "|" + mode
	m.mu.Lock()
	e, ok := m.ents[key]
	if ok {
		m.mu.Unlock()
		e.mu.Lock()
		e.sess.Agent, e.sess.Grant, e.sess.Toolkit = a, g, tk
		e.mu.Unlock()
		return e
	}
	e = &entry{sess: deck.AgentSession{Agent: a, Grant: g, Toolkit: tk, Mode: mode}}
	m.ents[key] = e
	m.mu.Unlock()

	instr := deck.BuildInstructions(tk)
	if mode == "use" {
		instr = m.deck.UseInstructions(tk, g, a.Client)
	}
	s := sdk.NewServer(&sdk.Implementation{Name: "mcpdeck-" + tk.Name, Title: "mcpDeck " + tk.Name, Version: version.Version}, &sdk.ServerOptions{
		Instructions: instr,
		HasTools:     true,
		InitializedHandler: func(ctx context.Context, req *sdk.InitializedRequest) {
			name, ver := "", ""
			if p := req.Session.InitializeParams(); p != nil && p.ClientInfo != nil {
				name, ver = p.ClientInfo.Name, p.ClientInfo.Version
			}
			e.mu.Lock()
			sess := e.sess
			e.mu.Unlock()
			m.deck.SessionSeen(context.WithoutCancel(ctx), sess.Agent, sess.Toolkit, mode, req.Session.ID(), name, ver)
		},
	})
	s.AddReceivingMiddleware(func(next sdk.MethodHandler) sdk.MethodHandler {
		return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
			res, err := next(ctx, method, req)
			if method == "tools/list" && err == nil {
				b, _ := json.Marshal(res)
				e.mu.Lock()
				sess := e.sess
				e.mu.Unlock()
				if ss, ok := req.GetSession().(*sdk.ServerSession); ok {
					sess.SessionID = ss.ID()
				}
				m.deck.RecordList(ctx, sess, b)
			}
			return res, err
		}
	})
	e.server = s
	e.handler = sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return s }, &sdk.StreamableHTTPOptions{})
	m.refresh(ctx, e)
	return e
}

// refresh replaces the server's tools; the SDK sends tools/list_changed (AB-7).
func (m *Manager) refresh(ctx context.Context, e *entry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if g, err := m.deck.Store.Grant(ctx, e.sess.Grant.ID); err == nil {
		e.sess.Grant = g
	}
	if tk, err := m.deck.Store.Toolkit(ctx, e.sess.Toolkit.ID); err == nil {
		e.sess.Toolkit = tk
	}
	var defs []deck.ToolDef
	if e.sess.Mode == "build" {
		defs = deck.BuildToolDefs()
	} else {
		defs = append(m.deck.MCPToolDefs(ctx, e.sess.Toolkit, e.sess.Grant.Access), m.deck.UseExtrasFor(e.sess.Grant.CanBuild)...)
	}
	want := map[string]bool{}
	for _, d := range defs {
		want[d.Name] = true
	}
	var stale []string
	for _, n := range e.names {
		if !want[n] {
			stale = append(stale, n)
		}
	}
	if len(stale) > 0 {
		e.server.RemoveTools(stale...)
	}
	e.names = e.names[:0]
	for _, d := range defs {
		d := d
		ro, destr := d.ReadOnly, d.Destructive
		t := &sdk.Tool{Name: d.Name, Description: d.Description, InputSchema: d.InputSchema,
			Annotations: &sdk.ToolAnnotations{ReadOnlyHint: ro, DestructiveHint: &destr, OpenWorldHint: boolp(false)}}
		e.server.AddTool(t, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
			e.mu.Lock()
			sess := e.sess
			e.mu.Unlock()
			if req.Session != nil {
				sess.SessionID = req.Session.ID()
			}
			text, isErr := m.deck.AgentCall(ctx, sess, d.Name, req.Params.Arguments)
			return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: text}}, IsError: isErr}, nil
		})
		e.names = append(e.names, d.Name)
	}
}

func boolp(b bool) *bool { return &b }

// ToolsChanged refreshes every server of those toolkits.
func (m *Manager) ToolsChanged(ids []string) {
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	m.mu.Lock()
	var es []*entry
	for _, e := range m.ents {
		if set[e.sess.Toolkit.ID] {
			es = append(es, e)
		}
	}
	m.mu.Unlock()
	for _, e := range es {
		m.refresh(context.Background(), e)
	}
}

// AgentRevoked ends the agent's sessions (AGT-4).
func (m *Manager) AgentRevoked(agentID string) {
	m.mu.Lock()
	var es []*entry
	for k, e := range m.ents {
		if e.sess.Agent.ID == agentID {
			es = append(es, e)
			delete(m.ents, k)
		}
	}
	m.mu.Unlock()
	for _, e := range es {
		for ss := range e.server.Sessions() {
			_ = ss.Close()
		}
	}
}
