package deck_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/vault"
)

func newDeck(t *testing.T) *deck.Deck {
	t.Helper()
	ctx := context.Background()
	s, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	v, _ := vault.Open(&vault.FakeKeychain{}, t.TempDir())
	c := clock.Real{}
	ids := clock.ULIDs{Clock: c}
	return deck.New(s, v, proxy.New(s, v, c, ids, nil), audit.New(s, c, ids), c, ids, t.TempDir(), 47800)
}

// REQ: UI-11
func TestWelcomeShowsOnceUntilSeen(t *testing.T) {
	ctx := context.Background()
	d := newDeck(t)
	h, err := d.Home(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if h.WelcomeSeen {
		t.Fatal("a new install hasn't seen the welcome page")
	}
	if err := d.SetSettings(ctx, map[string]string{"welcomeSeen": "true"}); err != nil {
		t.Fatal(err)
	}
	h, _ = d.Home(ctx)
	if !h.WelcomeSeen {
		t.Fatal("welcomeSeen must stick once set")
	}
	if err := d.SetSettings(ctx, map[string]string{"nope": "1"}); err == nil {
		t.Fatal("unknown settings must still be refused")
	}
}

// REQ: UI-11
func TestGettingStartedFollowsTheJourney(t *testing.T) {
	ctx := context.Background()
	d := newDeck(t)
	h, err := d.Home(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, s := range h.Steps {
		keys = append(keys, s.Key)
		if s.Done {
			t.Fatalf("nothing is done on a new install: %+v", s)
		}
	}
	want := "api auth send example toolkit tool agent"
	if got := strings.Join(keys, " "); got != want {
		t.Fatalf("steps = %q, want %q", got, want)
	}
	if h.Steps[1].Reason == "" || h.Steps[2].Reason == "" || h.Steps[4].Reason != "" {
		t.Fatalf("auth and send need an application; a toolkit needs nothing: %+v", h.Steps)
	}
	if _, err := d.CreateAPI(ctx, deck.CreateAPIInput{Name: "crm", BaseURL: "http://127.0.0.1:1"}); err != nil {
		t.Fatal(err)
	}
	h, _ = d.Home(ctx)
	if !h.Steps[0].Done || h.Steps[1].Reason != "" || h.Steps[1].Done || h.Steps[2].Reason != "" {
		t.Fatalf("with an application, sign-in is the next step: %+v", h.Steps)
	}
	if _, err := d.CreateAPI(ctx, deck.CreateAPIInput{Name: "billing", BaseURL: "http://127.0.0.1:1",
		Auth: store.Auth{Type: "bearer", Token: "{{token}}"}}); err != nil {
		t.Fatal(err)
	}
	h, _ = d.Home(ctx)
	if !h.Steps[1].Done {
		t.Fatalf("an application with auth set completes sign-in: %+v", h.Steps)
	}
}

// REQ: NET-1, NET-2
func TestNetworkSettings_PasswordIsSealedAndWriteOnly(t *testing.T) {
	ctx := context.Background()
	d := newDeck(t)
	v, _ := d.Settings(ctx, "")
	if v.Values["requestTimeoutSeconds"] != "30" || v.Values["proxyMode"] != "system" || v.Values["proxyPasswordSet"] != "false" {
		t.Fatalf("defaults: %v", v.Values)
	}
	err := d.SetSettings(ctx, map[string]string{"proxyMode": "manual", "proxyHttp": "proxy.corp:3128", "proxyUser": "me",
		"proxyPassword": "hunter2-canary", "noProxy": "internal.example", "requestTimeoutSeconds": "90"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ = d.Settings(ctx, "")
	if v.Values["proxyPasswordSet"] != "true" || v.Values["requestTimeoutSeconds"] != "90" {
		t.Fatalf("after save: %v", v.Values)
	}
	for k, x := range v.Values {
		if strings.Contains(x, "hunter2") {
			t.Fatalf("settings return the proxy password in %s", k)
		}
	}
	all, _ := d.Store.Settings(ctx)
	for k, x := range all {
		if strings.Contains(x, "hunter2") {
			t.Fatalf("the proxy password is stored in plain text under %s", k)
		}
	}
	// Leaving the password out keeps it; an empty one clears it.
	_ = d.SetSettings(ctx, map[string]string{"proxyUser": "me2"})
	if v, _ = d.Settings(ctx, ""); v.Values["proxyPasswordSet"] != "true" {
		t.Fatal("saving other settings dropped the password")
	}
	_ = d.SetSettings(ctx, map[string]string{"proxyPassword": ""})
	if v, _ = d.Settings(ctx, ""); v.Values["proxyPasswordSet"] != "false" {
		t.Fatal("an empty password didn't clear it")
	}
	for _, bad := range []map[string]string{{"proxyMode": "pac"}, {"proxyHttp": "ftp://x"}, {"requestTimeoutSeconds": "0"}, {"requestTimeoutMaxSeconds": "99999"}} {
		if err := d.SetSettings(ctx, bad); err == nil {
			t.Errorf("accepted %v", bad)
		}
	}
}

// REQ: NET-1
func TestNetworkSettings_AppliedToOutboundRequests(t *testing.T) {
	ctx := context.Background()
	d := newDeck(t)
	var saw string
	px := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saw = r.URL.String() + " " + r.Header.Get("Proxy-Authorization")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(px.Close)
	_ = d.Store.PutAPI(ctx, store.API{ID: "far", Name: "far"})
	_ = d.Store.PutRequest(ctx, store.Request{ID: "farGet", APIID: "far", Name: "get", Method: "GET", URL: "http://api.corp.example/items", Access: "read"})
	if err := d.SetSettings(ctx, map[string]string{"proxyMode": "manual", "proxyHttp": px.URL, "proxyUser": "me", "proxyPassword": "pw"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Proxy.Do(ctx, proxy.Call{APIID: "far", RequestID: "farGet"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(saw, "http://api.corp.example/items Basic ") {
		t.Fatalf("proxy saw %q", saw)
	}
	// A new deck on the same data picks the settings up at start.
	d2 := deck.New(d.Store, d.Vault, proxy.New(d.Store, d.Vault, clock.Real{}, clock.ULIDs{Clock: clock.Real{}}, nil), d.Audit, clock.Real{}, d.IDs, t.TempDir(), 47800)
	saw = ""
	if _, err := d2.Proxy.Do(ctx, proxy.Call{APIID: "far", RequestID: "farGet"}); err != nil || saw == "" {
		t.Fatalf("after restart the proxy wasn't used: %v", err)
	}
}
