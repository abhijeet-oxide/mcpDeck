package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/connect"
	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/vault"
)

func newServer(t *testing.T) (*Server, *Guard, string) {
	t.Helper()
	ctx := context.Background()
	s, _ := store.Open(ctx, t.TempDir())
	t.Cleanup(func() { s.Close() })
	v, _ := vault.Open(&vault.FakeKeychain{}, t.TempDir())
	c := clock.Real{}
	ids := clock.ULIDs{Clock: c}
	d := deck.New(s, v, proxy.New(s, v, c, ids, nil), audit.New(s, c, ids), c, ids, t.TempDir(), 47800)
	name := "support"
	tk, _ := d.CreateToolkit(ctx, deck.ToolkitInput{Name: &name})
	res, _ := d.Connect(ctx, deck.ConnectInput{Client: "claude-code", ToolkitID: tk.ID, Access: "read_write", CanBuild: true})
	g := NewGuard(47800)
	return New(Options{Deck: d, Guard: g, OS: connect.RealOS{}}), g, res.Token
}

// REQ: RUN-5, ACC-6
func TestGuard_HostOriginCookieToken(t *testing.T) {
	srv, g, agentTok := newServer(t)
	// Get a UI cookie through the launch link.
	launch := strings.TrimPrefix(g.LaunchURL("/"), "http://127.0.0.1:47800")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", launch, nil)
	req.Host = "127.0.0.1:47800"
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("launch: %d", rec.Code)
	}
	cookie := rec.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("cookie must be HttpOnly and SameSite=Strict")
	}
	// Second use of the same link fails.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", launch, nil)
	req.Host = "127.0.0.1:47800"
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("launch link reused: %d", rec.Code)
	}

	cases := []struct {
		name, method, path, host, origin, auth string
		cookie                                 bool
		want                                   int
	}{
		{"ui api with cookie", "GET", "/api/home", "127.0.0.1:47800", "", "", true, 200},
		{"localhost host ok", "GET", "/api/home", "localhost:47800", "http://localhost:47800", "", true, 200},
		{"no cookie", "GET", "/api/home", "127.0.0.1:47800", "", "", false, 401},
		{"dns rebinding host", "GET", "/api/home", "evil.example:47800", "", "", true, 421},
		{"wrong port in host", "GET", "/api/home", "127.0.0.1:80", "", "", true, 421},
		{"cross origin with cookie", "POST", "/api/toolkits", "127.0.0.1:47800", "https://evil.example", "", true, 403},
		{"cli token", "GET", "/api/home", "127.0.0.1:47800", "", "Bearer " + g.CLIToken, false, 200},
		{"agent token can't use the UI API", "GET", "/api/agents", "127.0.0.1:47800", "", "Bearer " + agentTok, false, 401},
		{"agent token can't approve", "POST", "/api/versions/x/approve", "127.0.0.1:47800", "", "Bearer " + agentTok, false, 401},
		{"agent token can't change access", "PATCH", "/api/grants/x", "127.0.0.1:47800", "", "Bearer " + agentTok, false, 401},
		{"agent token can't set credentials", "PUT", "/api/apis/x/variables", "127.0.0.1:47800", "", "Bearer " + agentTok, false, 401},
		{"mcp without token", "POST", "/mcp/support", "127.0.0.1:47800", "", "", false, 401},
		{"mcp with cookie only", "POST", "/mcp/support", "127.0.0.1:47800", "", "", true, 401},
		{"mcp browser origin", "POST", "/mcp/support", "127.0.0.1:47800", "https://evil.example", "Bearer " + agentTok, false, 403},
		{"mcp rebinding host", "POST", "/mcp/support", "evil.example:47800", "", "Bearer " + agentTok, false, 421},
		{"health open", "GET", "/healthz", "127.0.0.1:47800", "", "", false, 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, c.path, strings.NewReader(`{}`))
			req.Host = c.host
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			if c.auth != "" {
				req.Header.Set("Authorization", c.auth)
			}
			if c.cookie {
				req.AddCookie(cookie)
			}
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Fatalf("got %d want %d: %s", rec.Code, c.want, rec.Body.String())
			}
			if rec.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("no CORS headers, ever")
			}
		})
	}
}

// REQ: UI-12, RUN-5
// Browsers share cookies across ports on 127.0.0.1, so the demo (its own
// instance on the next port) must not replace the user's session.
func TestGuard_SessionCookiePerPort(t *testing.T) {
	main, demo := NewGuard(47800), NewGuard(47801)
	if main.CookieName() == demo.CookieName() {
		t.Fatalf("two instances share the cookie name %q", main.CookieName())
	}
	srv, g, _ := newServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", strings.TrimPrefix(g.LaunchURL("/"), "http://127.0.0.1:47800"), nil)
	req.Host = "127.0.0.1:47800"
	srv.ServeHTTP(rec, req)
	c := rec.Result().Cookies()[0]
	if c.Name != g.CookieName() {
		t.Fatalf("launch set %q, want %q", c.Name, g.CookieName())
	}
	// A demo cookie alongside it doesn't sign the user out.
	req = httptest.NewRequest("GET", "/api/info", nil)
	req.Host = "127.0.0.1:47800"
	req.AddCookie(&http.Cookie{Name: demo.CookieName(), Value: "demo-session"})
	req.AddCookie(c)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("with a demo cookie present: %d", rec.Code)
	}
}
