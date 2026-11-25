package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/vault"
)

const canary = "canary-secret-7f3a9c"

type fixture struct {
	p   *Proxy
	s   *store.Store
	api store.API
	srv *httptest.Server
	got chan *http.Request
}

func setup(t *testing.T, h http.HandlerFunc) *fixture {
	t.Helper()
	ctx := context.Background()
	s, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	v, _ := vault.Open(&vault.FakeKeychain{}, t.TempDir())
	got := make(chan *http.Request, 10)
	if h == nil {
		h = func(w http.ResponseWriter, r *http.Request) {
			got <- r
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ok":true,"echo":"`+r.Header.Get("Authorization")+`"}`)
		}
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	ids := &clock.Seq{Prefix: "id"}
	api := store.API{ID: "api1", Name: "crm", Auth: store.Auth{Type: "bearer", Token: "{{token}}"}}
	_ = s.PutAPI(ctx, api)
	_ = s.PutVariable(ctx, store.Variable{ID: "v1", APIID: "api1", Name: "baseUrl", Source: "literal", Value: srv.URL})
	sealed, _ := v.Seal(canary)
	_ = s.PutVariable(ctx, store.Variable{ID: "v2", APIID: "api1", Name: "token", Source: "secret", Ciphertext: sealed})
	for _, m := range []string{"GET", "HEAD", "OPTIONS", "POST", "PUT", "PATCH", "DELETE", "PURGE"} {
		_ = s.PutRequest(ctx, store.Request{ID: "r" + m, APIID: "api1", Name: strings.ToLower(m), Method: m,
			URL: "{{baseUrl}}/accounts/{id}", Access: ClassifyMethod(m)})
	}
	_ = s.PutRequest(ctx, store.Request{ID: "rSearch", APIID: "api1", Name: "search", Method: "POST", URL: "{{baseUrl}}/search", Access: "read"})
	_ = s.PutRequest(ctx, store.Request{ID: "rSoap", APIID: "api1", Name: "soap", Method: "GET", URL: "{{baseUrl}}/deleteThing", Access: "read", AccessFlag: true})
	return &fixture{p: New(s, v, clock.Real{}, ids, nil), s: s, api: api, srv: srv, got: got}
}

// REQ: ACC-2
func TestDo_ReadGrantBlocksEveryWrite(t *testing.T) {
	f := setup(t, nil)
	cases := []struct {
		req, access string
		blocked     bool
	}{
		{"rGET", "read", false}, {"rHEAD", "read", false}, {"rOPTIONS", "read", false},
		{"rPOST", "read", true}, {"rPUT", "read", true}, {"rPATCH", "read", true}, {"rDELETE", "read", true}, {"rPURGE", "read", true},
		{"rPOST", "read_write", false}, {"rDELETE", "read_write", false},
		{"rSearch", "read", false}, // POST overridden to read
		{"rSoap", "read", true},    // flagged "check access" counts as write
		{"rPOST", "", false},       // you, in the runner
	}
	for _, c := range cases {
		t.Run(c.req+"/"+c.access, func(t *testing.T) {
			_, err := f.p.Do(context.Background(), Call{APIID: "api1", RequestID: c.req, Access: c.access, ToolkitName: "support",
				Args: Args{Flat: map[string]any{"id": "7"}}, Actor: Actor{Kind: "agent", ID: "a"}})
			if c.blocked != IsCode(err, CodeWriteBlocked) {
				t.Fatalf("blocked=%v, err=%v", c.blocked, err)
			}
			if c.blocked && !strings.Contains(err.Error(), "read+write") {
				t.Fatalf("error must say what to do next: %v", err)
			}
		})
	}
}

// REQ: APP-2
func TestDo_RefusesHostsNotAllowed(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	api := f.api
	api.AllowedHosts = []string{"api.example.com"}
	_ = f.s.PutAPI(ctx, api)
	_, err := f.p.Do(ctx, Call{APIID: "api1", RequestID: "rGET", Args: Args{Flat: map[string]any{"id": "1"}}, Actor: Actor{Kind: "you"}})
	if !IsCode(err, CodeHostNotAllowed) {
		t.Fatalf("want host_not_allowed, got %v", err)
	}
	calls, _ := f.s.Calls(ctx, store.CallFilter{})
	if len(calls) != 1 || calls[0].Status != "blocked" {
		t.Fatalf("blocked call must be recorded: %+v", calls)
	}
}

// REQ: CRED-4, CRED-7
func TestDo_SecretNeverInResponseOrRecord(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	res, err := f.p.Do(ctx, Call{APIID: "api1", RequestID: "rGET", Args: Args{Flat: map[string]any{"id": "42"}}, Actor: Actor{Kind: "you"}})
	if err != nil {
		t.Fatal(err)
	}
	r := <-f.got
	if r.Header.Get("Authorization") != "Bearer "+canary {
		t.Fatalf("upstream must receive the real secret, got %q", r.Header.Get("Authorization"))
	}
	if r.URL.Path != "/accounts/42" {
		t.Fatalf("path param not applied: %s", r.URL.Path)
	}
	if strings.Contains(string(res.Body), canary) {
		t.Fatal("canary echoed back to caller")
	}
	rows, _ := f.s.DB.QueryContext(ctx, `SELECT data FROM call UNION ALL SELECT data FROM variable`)
	defer rows.Close()
	for rows.Next() {
		var d string
		_ = rows.Scan(&d)
		if strings.Contains(d, canary) {
			t.Fatalf("canary stored in plain text: %s", d)
		}
	}
}

// REQ: CRED-1
func TestDo_LoginRequestKeepsCookie(t *testing.T) {
	logins := 0
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			logins++
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "s1", Path: "/"})
			w.WriteHeader(200)
		default:
			if c, err := r.Cookie("SID"); err != nil || c.Value != "s1" {
				w.WriteHeader(401)
				return
			}
			_, _ = io.WriteString(w, `{"hold":false}`)
		}
	})
	ctx := context.Background()
	_ = f.s.PutRequest(ctx, store.Request{ID: "rLogin", APIID: "api1", Name: "login", Method: "POST", URL: "{{baseUrl}}/login", BodyType: "form", Body: "user=u&password={{token}}", Access: "write"})
	api := f.api
	api.Auth = store.Auth{Type: "login", LoginRequestID: "rLogin", Capture: "cookie"}
	_ = f.s.PutAPI(ctx, api)
	for i := 0; i < 2; i++ {
		res, err := f.p.Do(ctx, Call{APIID: "api1", RequestID: "rGET", Access: "read", Args: Args{Flat: map[string]any{"id": "1"}}, Actor: Actor{Kind: "agent"}})
		if err != nil || res.Status != 200 {
			t.Fatalf("call %d: %v %+v", i, err, res)
		}
	}
	if logins != 1 {
		t.Fatalf("want one login, got %d", logins)
	}
	_ = time.Second
}

// REQ: VAR-1
func TestDo_UnresolvedVariablesAreListed(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	_, err := f.p.Do(ctx, Call{APIID: "api1", Draft: &store.Request{Method: "GET", URL: "{{host}}/x"}, Actor: Actor{Kind: "you"}})
	if !IsCode(err, CodeUnresolved) || !strings.Contains(err.Error(), "{{host}}") {
		t.Fatalf("got %v", err)
	}
}
