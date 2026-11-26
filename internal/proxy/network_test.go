package proxy

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// testProxy is a forward HTTP proxy that answers itself and records what it saw.
func testProxy(t *testing.T) (*httptest.Server, chan *http.Request) {
	t.Helper()
	seen := make(chan *http.Request, 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r
		if r.Header.Get("Proxy-Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte("me:s3cret")) {
			w.Header().Set("Proxy-Authenticate", `Basic realm="corp"`)
			w.WriteHeader(http.StatusProxyAuthRequired)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"via":"proxy","host":"`+r.URL.Host+`"}`)
	}))
	t.Cleanup(srv.Close)
	return srv, seen
}

// An API whose host isn't loopback, so the proxy applies.
func farAPI(t *testing.T, f *fixture) {
	t.Helper()
	ctx := context.Background()
	_ = f.s.PutAPI(ctx, store.API{ID: "far", Name: "far"})
	_ = f.s.PutRequest(ctx, store.Request{ID: "farGet", APIID: "far", Name: "get", Method: "GET",
		URL: "http://api.corp.example/v1/items", Access: "read"})
}

// REQ: NET-1
func TestNetwork_ManualProxyCarriesRequestsAndSignsIn(t *testing.T) {
	f := setup(t, nil)
	farAPI(t, f)
	px, seen := testProxy(t)
	f.p.SetNetwork(Network{Mode: "manual", HTTP: px.URL, User: "me", Password: "s3cret"})
	res, err := f.p.Do(context.Background(), Call{APIID: "far", RequestID: "farGet"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res.Body), `"via":"proxy"`) {
		t.Fatalf("the request didn't go through the proxy: %s", res.Body)
	}
	r := <-seen
	if r.URL.Host != "api.corp.example" || r.URL.Path != "/v1/items" {
		t.Fatalf("proxy saw %s", r.URL)
	}

	// A wrong password: the proxy says so, and the password isn't echoed.
	f.p.SetNetwork(Network{Mode: "manual", HTTP: px.URL, User: "me", Password: "wrong"})
	res, err = f.p.Do(context.Background(), Call{APIID: "far", RequestID: "farGet"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != http.StatusProxyAuthRequired {
		t.Fatalf("status = %d, want 407", res.Status)
	}
}

// REQ: NET-1
func TestNetwork_NoProxyAndLoopbackGoDirect(t *testing.T) {
	f := setup(t, nil)
	px, seen := testProxy(t)
	f.p.SetNetwork(Network{Mode: "manual", HTTP: px.URL, User: "me", Password: "s3cret"})
	// The fixture's API is on 127.0.0.1: loopback never uses the proxy.
	if _, err := f.p.Do(context.Background(), Call{APIID: "api1", RequestID: "rGET", Args: Args{Flat: map[string]any{"id": "1"}}}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 0 {
		t.Fatal("a loopback request went through the proxy")
	}
	n := Network{Mode: "manual", HTTP: "http://proxy.corp:3128", HTTPS: "proxy.corp:3129",
		NoProxy: "internal.example, .svc.cluster.local, 10.0.0.0/8"}
	cases := map[string]string{
		"http://api.example.com/x":           "http://proxy.corp:3128",
		"https://api.example.com/x":          "http://proxy.corp:3129",
		"https://internal.example/x":         "",
		"https://a.b.internal.example/x":     "",
		"http://db.ns.svc.cluster.local/":    "",
		"http://10.1.2.3:8080/":              "",
		"http://11.1.2.3/":                   "http://proxy.corp:3128",
		"http://localhost:9000/":             "",
		"http://[::1]:9000/":                 "",
		"https://notinternal.example.org/x/": "http://proxy.corp:3129",
	}
	for raw, want := range cases {
		u, _ := url.Parse(raw)
		got, err := n.ProxyFor(u)
		if err != nil {
			t.Fatal(err)
		}
		g := ""
		if got != nil {
			g = got.String()
		}
		if g != want {
			t.Errorf("%s: proxy %q, want %q", raw, g, want)
		}
	}
	u, _ := url.Parse("http://api.example.com/")
	if got, _ := (Network{Mode: "none", HTTP: "http://proxy.corp:3128"}).ProxyFor(u); got != nil {
		t.Errorf("mode none still used %s", got)
	}
	if _, err := (Network{Mode: "manual", HTTP: "ftp://x"}).ProxyFor(u); err == nil {
		t.Error("an ftp proxy address was accepted")
	}
}

// REQ: NET-2
func TestTimeout_RequestOverridesTheDefaultAndSaysWhatToDo(t *testing.T) {
	slow := make(chan struct{})
	t.Cleanup(func() { close(slow) })
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-slow:
		case <-r.Context().Done():
		}
	})
	ctx := context.Background()
	_ = f.s.SetSetting(ctx, "requestTimeoutSeconds", "0.2")
	start := time.Now()
	_, err := f.p.Do(ctx, Call{APIID: "api1", RequestID: "rGET", Args: Args{Flat: map[string]any{"id": "1"}}})
	if !IsCode(err, CodeTimeout) || !strings.Contains(err.Error(), "timed out after 200ms") {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("the default timeout wasn't applied")
	}
	// The request's own timeout wins, capped by the maximum.
	r, _ := f.s.Request(ctx, "rGET")
	r.TimeoutSeconds = 0.4
	_ = f.s.PutRequest(ctx, r)
	_, err = f.p.Do(ctx, Call{APIID: "api1", RequestID: "rGET", Args: Args{Flat: map[string]any{"id": "1"}}})
	if !strings.Contains(err.Error(), "timed out after 400ms") {
		t.Fatalf("err = %v", err)
	}
	_ = f.s.SetSetting(ctx, "requestTimeoutMaxSeconds", "0.3")
	_, err = f.p.Do(ctx, Call{APIID: "api1", RequestID: "rGET", Args: Args{Flat: map[string]any{"id": "1"}}})
	if !strings.Contains(err.Error(), "timed out after 300ms") {
		t.Fatalf("err = %v", err)
	}
}

// REQ: NET-2
func TestTimeout_DefaultIsThirtySeconds(t *testing.T) {
	f := setup(t, nil)
	if got := f.p.timeoutFor(context.Background(), nil); got != 30*time.Second {
		t.Fatalf("default = %s", got)
	}
}

// REQ: NET-3
func TestCancel_StopsTheUpstreamCall(t *testing.T) {
	hit := make(chan struct{}, 1)
	gone := make(chan struct{}, 1)
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		hit <- struct{}{}
		<-r.Context().Done()
		gone <- struct{}{}
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-hit; cancel() }()
	_, err := f.p.Do(ctx, Call{APIID: "api1", RequestID: "rGET", Args: Args{Flat: map[string]any{"id": "1"}}})
	if !IsCode(err, CodeCanceled) {
		t.Fatalf("err = %v", err)
	}
	select {
	case <-gone:
	case <-time.After(3 * time.Second):
		t.Fatal("the upstream request kept running after cancel")
	}
}
