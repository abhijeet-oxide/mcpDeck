// Package server is mcpDeck's HTTP API, MCP mount and embedded UI.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/connect"
	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/mcp"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/version"
	"github.com/abhijeet-oxide/mcpdeck/skills"
)

// Opener opens URLs and folders on the user's machine. Tests use a fake.
type Opener interface {
	Open(target string) error
}

// Options wires a server.
type Options struct {
	Deck      *deck.Deck
	Guard     *Guard
	UI        fs.FS // the dist folder
	OS        connect.OS
	Opener    Opener
	StartDemo func(ctx context.Context) (string, error) // returns the demo's launch URL
	ReturnURL string                                    // in a demo instance: the user's own mcpDeck
	Shutdown  func()
}

// Server handles every route.
type Server struct {
	opt   Options
	mcp   *mcp.Manager
	mux   *http.ServeMux
	mu    sync.Mutex
	plans map[string]*planRec
}

type planRec struct {
	in      connect.Input
	plan    *connect.Plan
	agentID string
	created time.Time
}

// New builds the handler.
func New(o Options) *Server {
	s := &Server{opt: o, mcp: mcp.New(o.Deck), mux: http.NewServeMux(), plans: map[string]*planRec{}}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g := s.opt.Guard
	if !g.HostOK(r) {
		http.Error(w, "mcpDeck only answers on 127.0.0.1 and localhost", http.StatusMisdirectedRequest)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	switch {
	case r.URL.Path == "/healthz":
		_, _ = io.WriteString(w, "ok")
	case r.URL.Path == "/launch":
		g.Launch(w, r)
	case strings.HasPrefix(r.URL.Path, "/mcp/"):
		s.mcp.ServeHTTP(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/"):
		if !g.OriginOK(r) {
			writeErr(w, http.StatusForbidden, "cross_origin", "requests from other sites are refused")
			return
		}
		who, ok := g.Authed(r)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "no_session", "open mcpDeck from the link it printed when it started, or run mcpdeck again")
			return
		}
		r = withAdminAgent(r, who)
		if why := blockedForAgent(r); why != "" {
			writeErr(w, http.StatusForbidden, "person_only", why)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		s.mux.ServeHTTP(w, r)
	default:
		s.ui(w, r)
	}
}

func (s *Server) ui(w http.ResponseWriter, r *http.Request) {
	if s.opt.UI == nil {
		http.Error(w, "UI not built: run make build", http.StatusNotFound)
		return
	}
	p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if p == "" {
		p = "index.html"
	}
	if f, err := fs.Stat(s.opt.UI, p); err != nil || f.IsDir() {
		p = "index.html"
	}
	b, err := fs.ReadFile(s.opt.UI, p)
	if err != nil {
		http.Error(w, "UI not built: run make build", http.StatusNotFound)
		return
	}
	if p == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; worker-src 'self' blob:; script-src 'self' 'unsafe-eval' blob:; font-src 'self' data:; connect-src 'self'")
	} else if strings.HasPrefix(p, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeContent(w, r, p, time.Time{}, strings.NewReader(string(b)))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"code": code, "error": msg})
}

func fail(w http.ResponseWriter, err error) {
	status := deck.StatusOf(err)
	code := "error"
	var de *deck.Error
	if errors.As(err, &de) {
		code = de.Code
	}
	msg := err.Error()
	if status == http.StatusInternalServerError {
		msg = "something went wrong: " + msg
	}
	writeErr(w, status, code, msg)
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 16<<20))
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return &deck.Error{Status: 400, Code: "bad_json", Message: "request body isn't valid JSON: " + err.Error()}
	}
	return nil
}

// h adapts a handler that returns a value or an error.
func h(fn func(r *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := fn(r)
		if err != nil {
			fail(w, err)
			return
		}
		if v == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
}

func body[T any](r *http.Request) (T, error) {
	var v T
	err := decode(r, &v)
	return v, err
}

func (s *Server) routes() {
	d := s.opt.Deck
	m := s.mux
	m.HandleFunc("GET /api/info", h(func(r *http.Request) (any, error) {
		return map[string]any{"version": version.Version, "port": d.Port, "baseUrl": d.BaseURL(), "os": s.opt.OS.GOOS(),
			"demo": d.Demo, "practice": d.Practice, "returnUrl": s.opt.ReturnURL, "dataDir": d.DataDir}, nil
	}))
	m.HandleFunc("GET /api/home", h(func(r *http.Request) (any, error) { return d.Home(r.Context()) }))

	// APIs
	m.HandleFunc("GET /api/apis", h(func(r *http.Request) (any, error) { return d.ListAPIs(r.Context()) }))
	m.HandleFunc("POST /api/apis", h(func(r *http.Request) (any, error) {
		in, err := body[deck.CreateAPIInput](r)
		if err != nil {
			return nil, err
		}
		return d.CreateAPI(r.Context(), in)
	}))
	m.HandleFunc("GET /api/apis/{id}", h(func(r *http.Request) (any, error) { return d.GetAPI(r.Context(), r.PathValue("id")) }))
	m.HandleFunc("PATCH /api/apis/{id}", h(func(r *http.Request) (any, error) {
		in, err := body[deck.UpdateAPIInput](r)
		if err != nil {
			return nil, err
		}
		return d.UpdateAPI(r.Context(), r.PathValue("id"), in)
	}))
	m.HandleFunc("DELETE /api/apis/{id}", h(func(r *http.Request) (any, error) { return nil, d.DeleteAPI(r.Context(), r.PathValue("id")) }))
	m.HandleFunc("GET /api/apis/{id}/history", h(func(r *http.Request) (any, error) { return d.History(r.Context(), r.PathValue("id")) }))
	m.HandleFunc("POST /api/apis/{id}/cookies/clear", h(func(r *http.Request) (any, error) {
		d.Proxy.ClearCookies(r.PathValue("id"))
		return nil, nil
	}))
	m.HandleFunc("POST /api/import/preview", h(func(r *http.Request) (any, error) {
		in, err := body[deck.ImportSource](r)
		if err != nil {
			return nil, err
		}
		return d.PreviewImport(r.Context(), in)
	}))
	m.HandleFunc("POST /api/import", h(func(r *http.Request) (any, error) {
		in, err := body[deck.ImportInput](r)
		if err != nil {
			return nil, err
		}
		return d.Import(r.Context(), in)
	}))
	m.HandleFunc("POST /api/apis/{id}/requests", h(func(r *http.Request) (any, error) {
		in, err := body[store.Request](r)
		if err != nil {
			return nil, err
		}
		in.ID = ""
		return d.SaveRequest(r.Context(), r.PathValue("id"), in)
	}))
	m.HandleFunc("PUT /api/requests/{id}", h(func(r *http.Request) (any, error) {
		in, err := body[store.Request](r)
		if err != nil {
			return nil, err
		}
		old, err := d.Store.Request(r.Context(), r.PathValue("id"))
		if err != nil {
			return nil, &deck.Error{Status: 404, Code: "not_found", Message: "request not found"}
		}
		in.ID = old.ID
		return d.SaveRequest(r.Context(), old.APIID, in)
	}))
	m.HandleFunc("DELETE /api/requests/{id}", h(func(r *http.Request) (any, error) { return nil, d.DeleteRequest(r.Context(), r.PathValue("id")) }))
	m.HandleFunc("POST /api/requests/{id}/access", h(func(r *http.Request) (any, error) {
		in, err := body[struct{ Access, Reason string }](r)
		if err != nil {
			return nil, err
		}
		return d.SetAccess(r.Context(), r.PathValue("id"), in.Access, in.Reason)
	}))
	m.HandleFunc("POST /api/requests/{id}/example", h(func(r *http.Request) (any, error) {
		in, err := body[struct {
			Body json.RawMessage `json:"body"`
		}](r)
		if err != nil {
			return nil, err
		}
		return d.SaveExample(r.Context(), r.PathValue("id"), in.Body)
	}))
	m.HandleFunc("POST /api/send", h(func(r *http.Request) (any, error) {
		in, err := body[deck.SendInput](r)
		if err != nil {
			return nil, err
		}
		return d.Send(r.Context(), in)
	}))
	m.HandleFunc("POST /api/apis/{id}/environments", h(func(r *http.Request) (any, error) {
		in, err := body[struct{ Name string }](r)
		if err != nil {
			return nil, err
		}
		return d.CreateEnvironment(r.Context(), r.PathValue("id"), in.Name)
	}))
	m.HandleFunc("PATCH /api/environments/{id}", h(func(r *http.Request) (any, error) {
		in, err := body[struct{ Name string }](r)
		if err != nil {
			return nil, err
		}
		return d.RenameEnvironment(r.Context(), r.PathValue("id"), in.Name)
	}))
	m.HandleFunc("POST /api/apis/{id}/environments/import", h(func(r *http.Request) (any, error) {
		in, err := body[deck.ImportEnvironmentInput](r)
		if err != nil {
			return nil, err
		}
		return d.ImportEnvironment(r.Context(), r.PathValue("id"), in)
	}))
	m.HandleFunc("POST /api/requests/{id}/responses", h(func(r *http.Request) (any, error) {
		in, err := body[deck.SaveResponseInput](r)
		if err != nil {
			return nil, err
		}
		return d.SaveResponse(r.Context(), r.PathValue("id"), in)
	}))
	m.HandleFunc("DELETE /api/requests/{id}/responses/{rid}", h(func(r *http.Request) (any, error) {
		return d.DeleteResponse(r.Context(), r.PathValue("id"), r.PathValue("rid"))
	}))
	m.HandleFunc("DELETE /api/environments/{id}", h(func(r *http.Request) (any, error) { return nil, d.DeleteEnvironment(r.Context(), r.PathValue("id")) }))
	m.HandleFunc("PUT /api/apis/{id}/variables", h(func(r *http.Request) (any, error) {
		in, err := body[deck.SetVariableInput](r)
		if err != nil {
			return nil, err
		}
		return d.SetVariable(r.Context(), r.PathValue("id"), in)
	}))
	m.HandleFunc("DELETE /api/variables/{id}", h(func(r *http.Request) (any, error) { return nil, d.DeleteVariable(r.Context(), r.PathValue("id")) }))

	// Toolkits and tools
	m.HandleFunc("GET /api/toolkits", h(func(r *http.Request) (any, error) { return d.ListToolkits(r.Context()) }))
	m.HandleFunc("POST /api/toolkits", h(func(r *http.Request) (any, error) {
		in, err := body[deck.ToolkitInput](r)
		if err != nil {
			return nil, err
		}
		return d.CreateToolkit(r.Context(), in)
	}))
	m.HandleFunc("GET /api/toolkits/{ref}", h(func(r *http.Request) (any, error) { return d.GetToolkit(r.Context(), r.PathValue("ref")) }))
	m.HandleFunc("GET /api/toolkits/{ref}/agent-view", h(func(r *http.Request) (any, error) {
		q := r.URL.Query()
		return d.AgentView(r.Context(), r.PathValue("ref"), q.Get("access"), q.Get("build") == "1", q.Get("client"))
	}))
	m.HandleFunc("PATCH /api/toolkits/{ref}", h(func(r *http.Request) (any, error) {
		in, err := body[deck.ToolkitInput](r)
		if err != nil {
			return nil, err
		}
		return d.UpdateToolkit(r.Context(), r.PathValue("ref"), in)
	}))
	m.HandleFunc("DELETE /api/toolkits/{ref}", h(func(r *http.Request) (any, error) {
		return nil, d.DeleteToolkitForce(r.Context(), r.PathValue("ref"), r.URL.Query().Get("force") == "1")
	}))
	m.HandleFunc("POST /api/toolkits/{ref}/tools", h(func(r *http.Request) (any, error) {
		in, err := body[struct{ Tool string }](r)
		if err != nil {
			return nil, err
		}
		return nil, d.AddToolToToolkit(r.Context(), r.PathValue("ref"), in.Tool)
	}))
	m.HandleFunc("DELETE /api/toolkits/{ref}/tools/{tool}", h(func(r *http.Request) (any, error) {
		return nil, d.RemoveToolFromToolkit(r.Context(), r.PathValue("ref"), r.PathValue("tool"))
	}))
	m.HandleFunc("POST /api/toolkits/{ref}/lint", h(func(r *http.Request) (any, error) {
		in, err := body[deck.LintInput](r)
		if err != nil {
			return nil, err
		}
		return d.LintToolkit(r.Context(), r.PathValue("ref"), in)
	}))
	m.HandleFunc("POST /api/lint/output", h(func(r *http.Request) (any, error) {
		in, err := body[struct {
			Value json.RawMessage `json:"value"`
		}](r)
		if err != nil {
			return nil, err
		}
		return d.LintOutput(in.Value)
	}))
	m.HandleFunc("GET /api/toolkits/{ref}/types.d.ts", func(w http.ResponseWriter, r *http.Request) {
		t, err := d.TypesFor(r.Context(), r.PathValue("ref"))
		if err != nil {
			fail(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, t)
	})
	m.HandleFunc("POST /api/toolkits/{ref}/save", h(func(r *http.Request) (any, error) {
		in, err := body[deck.SaveInput](r)
		if err != nil {
			return nil, err
		}
		return d.SaveTool(r.Context(), r.PathValue("ref"), deck.Caller{Actor: you(r)}, in)
	}))
	m.HandleFunc("POST /api/toolkits/{ref}/test", h(func(r *http.Request) (any, error) {
		in, err := body[deck.TestInput](r)
		if err != nil {
			return nil, err
		}
		tk, err := d.ToolkitByRef(r.Context(), r.PathValue("ref"))
		if err != nil {
			return nil, err
		}
		return d.TestTool(r.Context(), deck.Caller{Actor: you(r), Toolkit: &tk}, in)
	}))
	m.HandleFunc("POST /api/tools/generate", h(func(r *http.Request) (any, error) {
		in, err := body[deck.GenerateInput](r)
		if err != nil {
			return nil, err
		}
		return d.GenerateTool(r.Context(), in)
	}))
	m.HandleFunc("GET /api/tools", h(func(r *http.Request) (any, error) { return d.Library(r.Context()) }))
	m.HandleFunc("GET /api/tools/{ref}", h(func(r *http.Request) (any, error) { return d.GetTool(r.Context(), r.PathValue("ref")) }))
	m.HandleFunc("DELETE /api/tools/{ref}", h(func(r *http.Request) (any, error) { return nil, d.DeleteTool(r.Context(), r.PathValue("ref")) }))
	m.HandleFunc("GET /api/versions/{id}", h(func(r *http.Request) (any, error) { return d.Store.Version(r.Context(), r.PathValue("id")) }))
	m.HandleFunc("POST /api/versions/{id}/approve", h(func(r *http.Request) (any, error) { return d.Decide(r.Context(), r.PathValue("id"), true) }))
	m.HandleFunc("POST /api/versions/{id}/reject", h(func(r *http.Request) (any, error) { return d.Decide(r.Context(), r.PathValue("id"), false) }))
	m.HandleFunc("POST /api/versions/{id}/rollback", h(func(r *http.Request) (any, error) { return d.Rollback(r.Context(), r.PathValue("id")) }))
	m.HandleFunc("POST /api/tool-requests/{id}/close", h(func(r *http.Request) (any, error) { return nil, d.CloseRequest(r.Context(), r.PathValue("id")) }))

	// Agents and connecting
	m.HandleFunc("GET /api/agents", h(func(r *http.Request) (any, error) { return d.ListAgents(r.Context()) }))
	m.HandleFunc("GET /api/agents/{id}", h(func(r *http.Request) (any, error) { return d.GetAgent(r.Context(), r.PathValue("id")) }))
	m.HandleFunc("POST /api/agents/{id}/revoke", h(func(r *http.Request) (any, error) { return nil, d.Revoke(r.Context(), r.PathValue("id")) }))
	m.HandleFunc("POST /api/agents/{id}/rotate", h(func(r *http.Request) (any, error) {
		in, _ := body[planOpts](r)
		res, err := d.Rotate(r.Context(), r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		in.Client = res.Agent.Client
		return s.withPlan(res, in)
	}))
	m.HandleFunc("PATCH /api/grants/{id}", h(func(r *http.Request) (any, error) {
		in, err := body[deck.GrantInput](r)
		if err != nil {
			return nil, err
		}
		return d.UpdateGrant(r.Context(), r.PathValue("id"), in)
	}))
	m.HandleFunc("DELETE /api/grants/{id}", h(func(r *http.Request) (any, error) { return nil, d.DeleteGrant(r.Context(), r.PathValue("id")) }))
	m.HandleFunc("POST /api/connect", h(func(r *http.Request) (any, error) {
		in, err := body[struct {
			deck.ConnectInput
			planOpts
		}](r)
		if err != nil {
			return nil, err
		}
		if you(r).Kind == "agent" && (in.Access != "read" || in.CanBuild) {
			return nil, deck.Forbidden("an agent can connect clients with read access only; ask the user to give write or build access in mcpDeck")
		}
		res, err := d.Connect(r.Context(), in.ConnectInput)
		if err != nil {
			return nil, err
		}
		in.planOpts.Client = in.ConnectInput.Client
		return s.withPlan(res, in.planOpts)
	}))
	m.HandleFunc("POST /api/plans/{id}", h(func(r *http.Request) (any, error) {
		in, err := body[planOpts](r)
		if err != nil {
			return nil, err
		}
		return s.replan(r.PathValue("id"), in)
	}))
	m.HandleFunc("POST /api/plans/{id}/apply", h(func(r *http.Request) (any, error) {
		s.mu.Lock()
		rec, ok := s.plans[r.PathValue("id")]
		s.mu.Unlock()
		if !ok {
			return nil, &deck.Error{Status: 404, Code: "plan_expired", Message: "this connection plan expired; rotate the token to make a new one"}
		}
		written, err := connect.Apply(rec.plan)
		if err != nil {
			return nil, &deck.Error{Status: 500, Code: "write_failed", Message: err.Error()}
		}
		a, _ := d.Store.Agent(r.Context(), rec.agentID)
		_ = d.Audit.Append(r.Context(), you(r), "connect.write_config", a.Name, strings.Join(written, ", "))
		return map[string]any{"written": written}, nil
	}))

	// Activity
	m.HandleFunc("GET /api/calls", h(func(r *http.Request) (any, error) {
		q := r.URL.Query()
		f := store.CallFilter{SessionID: q.Get("session"), ActorID: q.Get("agent"), ToolkitID: q.Get("toolkit"), Kind: q.Get("kind"),
			Status: q.Get("status"), Name: q.Get("q"), TopLevel: q.Get("all") != "1"}
		if n, err := strconv.Atoi(q.Get("limit")); err == nil {
			f.Limit = n
		}
		if since := q.Get("since"); since != "" {
			f.Since = time.Now().Add(-rangeOf(since))
		}
		return d.Calls(r.Context(), f)
	}))
	m.HandleFunc("GET /api/calls/{id}", h(func(r *http.Request) (any, error) { return d.GetCall(r.Context(), r.PathValue("id")) }))
	m.HandleFunc("GET /api/usage", h(func(r *http.Request) (any, error) { return d.Usage(r.Context(), r.URL.Query().Get("range")) }))
	m.HandleFunc("GET /api/audit", h(func(r *http.Request) (any, error) { return d.AuditLog(r.Context(), 500) }))
	m.HandleFunc("POST /api/audit/verify", h(func(r *http.Request) (any, error) { return d.VerifyAudit(r.Context()) }))

	// Settings and the machine
	m.HandleFunc("GET /api/settings", h(func(r *http.Request) (any, error) { return d.Settings(r.Context(), version.Version) }))
	m.HandleFunc("PUT /api/settings", h(func(r *http.Request) (any, error) {
		in, err := body[map[string]string](r)
		if err != nil {
			return nil, err
		}
		return nil, d.SetSettings(r.Context(), in)
	}))
	m.HandleFunc("POST /api/reveal-data", h(func(r *http.Request) (any, error) {
		if s.opt.Opener == nil {
			return nil, &deck.Error{Status: 501, Code: "unsupported", Message: "can't open folders here"}
		}
		return nil, s.opt.Opener.Open(d.DataDir)
	}))
	m.HandleFunc("POST /api/demo", h(func(r *http.Request) (any, error) {
		if s.opt.StartDemo == nil {
			return nil, &deck.Error{Status: 501, Code: "unsupported", Message: "run mcpdeck dev to open the demo"}
		}
		u, err := s.opt.StartDemo(r.Context())
		if err != nil {
			return nil, &deck.Error{Status: 500, Code: "demo_failed", Message: err.Error()}
		}
		return map[string]string{"url": u}, nil
	}))
	m.HandleFunc("POST /api/shutdown", h(func(r *http.Request) (any, error) {
		if s.opt.Shutdown != nil {
			go func() { time.Sleep(200 * time.Millisecond); s.opt.Shutdown() }()
		}
		return map[string]string{"returnUrl": s.opt.ReturnURL}, nil
	}))
	m.HandleFunc("POST /api/launch-url", h(func(r *http.Request) (any, error) {
		return map[string]string{"url": s.opt.Guard.LaunchURL(r.URL.Query().Get("next"))}, nil
	}))
}

func rangeOf(s string) time.Duration {
	switch s {
	case "1h":
		return time.Hour
	case "24h":
		return 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	}
	return 7 * 24 * time.Hour
}

type planOpts struct {
	Client     string `json:"-"`
	Scope      string `json:"scope"`
	ProjectDir string `json:"projectDir"`
	Skills     *bool  `json:"skills"`
}

func (s *Server) buildPlan(agentID, token string, grants []deck.GrantRow, o planOpts) (*connect.Plan, connect.Input, error) {
	in := connect.Input{Client: o.Client, Token: token, BaseURL: s.opt.Deck.BaseURL(), Scope: o.Scope, ProjectDir: o.ProjectDir,
		Skills: o.Skills == nil || *o.Skills, DataDir: s.opt.Deck.DataDir}
	for _, g := range grants {
		in.Toolkits = append(in.Toolkits, connect.Toolkit{Name: g.ToolkitName, CanBuild: g.CanBuild})
	}
	p, err := connect.Build(in, s.opt.OS, skills.FS, skills.Names)
	if err != nil {
		return nil, in, &deck.Error{Status: 400, Code: "bad_plan", Message: err.Error()}
	}
	return p, in, nil
}

func (s *Server) withPlan(res *deck.ConnectResult, o planOpts) (any, error) {
	p, in, err := s.buildPlan(res.Agent.ID, res.Token, res.Grants, o)
	if err != nil {
		return nil, err
	}
	id := randHex(12)
	s.mu.Lock()
	for k, v := range s.plans {
		if time.Since(v.created) > 30*time.Minute {
			delete(s.plans, k)
		}
	}
	s.plans[id] = &planRec{in: in, plan: p, agentID: res.Agent.ID, created: time.Now()}
	s.mu.Unlock()
	return map[string]any{"result": res, "plan": p, "planId": id}, nil
}

func (s *Server) replan(id string, o planOpts) (any, error) {
	s.mu.Lock()
	rec, ok := s.plans[id]
	s.mu.Unlock()
	if !ok {
		return nil, &deck.Error{Status: 404, Code: "plan_expired", Message: "this connection plan expired; rotate the token to make a new one"}
	}
	in := rec.in
	if o.Scope != "" {
		in.Scope = o.Scope
	}
	in.ProjectDir = o.ProjectDir
	if o.Skills != nil {
		in.Skills = *o.Skills
	}
	p, err := connect.Build(in, s.opt.OS, skills.FS, skills.Names)
	if err != nil {
		return nil, &deck.Error{Status: 400, Code: "bad_plan", Message: err.Error()}
	}
	s.mu.Lock()
	rec.in, rec.plan = in, p
	s.mu.Unlock()
	return map[string]any{"plan": p, "planId": id}, nil
}

// Addr formats the listen address.
func Addr(port int) string { return fmt.Sprintf("127.0.0.1:%d", port) }
