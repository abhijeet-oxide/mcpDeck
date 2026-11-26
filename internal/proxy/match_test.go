package proxy

import (
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

func TestMatchRequest_PrefersLiteralSegments(t *testing.T) {
	reqs := []store.Request{
		{ID: "a", Method: "GET", URL: "{{baseUrl}}/orders/{id}"},
		{ID: "b", Method: "GET", URL: "{{baseUrl}}/orders/recent"},
		{ID: "c", Method: "POST", URL: "{{baseUrl}}/orders/{id}"},
		{ID: "d", Method: "GET", URL: "{{baseUrl}}/repos/:owner/:repo"},
	}
	if r, p := MatchRequest(reqs, "get", "/orders/ord_1"); r == nil || r.ID != "a" || p["id"] != "ord_1" {
		t.Fatalf("got %v %v", r, p)
	}
	if r, _ := MatchRequest(reqs, "GET", "/orders/recent"); r == nil || r.ID != "b" {
		t.Fatalf("literal path should win, got %v", r)
	}
	if r, p := MatchRequest(reqs, "GET", "/repos/acme/api"); r == nil || r.ID != "d" || p["owner"] != "acme" || p["repo"] != "api" {
		t.Fatalf("colon params: %v %v", r, p)
	}
	if r, _ := MatchRequest(reqs, "DELETE", "/orders/1"); r != nil {
		t.Fatalf("method must match, got %v", r)
	}
}

func TestClassifyRaw_GraphQL(t *testing.T) {
	q := Args{Body: map[string]any{"query": "query { orders(first: 3) { edges { node { id } } } }"}, HasBody: true}
	m := Args{Body: map[string]any{"query": "mutation Cancel($id: ID!) { cancelOrder(id: $id) { order { id } } }"}, HasBody: true}
	if got := ClassifyRaw("POST", "/graphql", q); got != "read" {
		t.Fatalf("query: %s", got)
	}
	if got := ClassifyRaw("POST", "/graphql", m); got != "write" {
		t.Fatalf("mutation: %s", got)
	}
	if got := ClassifyRaw("POST", "/orders", q); got != "write" {
		t.Fatalf("plain POST: %s", got)
	}
	if IsGraphQLMutation(`{ mutationLog { id } }`) {
		t.Fatal("a field named mutationLog is not a mutation")
	}
}

// REQ: AUTH-3
func TestTokenURLPattern(t *testing.T) {
	re := tokenURLPattern([]string{"idp.example.com", "127.0.0.1:8443"})
	for u, want := range map[string]bool{
		"https://idp.example.com/oauth2/token":            true,
		"https://IDP.example.com:9443/token":              true,
		"http://127.0.0.1:8443/token":                     true,
		"https://idp.example.com?x=1":                     true,
		"https://idp.example.com":                         true,
		"https://idp.example.com.evil.test/token":         false,
		"https://idp.example.com@evil.test/token":         false,
		"https://evil.test/https://idp.example.com/token": false,
		"https://127.0.0.1:9999/token":                    false,
		"file:///etc/passwd":                              false,
		"ftp://idp.example.com/token":                     false,
		"https://idpxexample.com/token":                   false,
	} {
		if got := re.MatchString(u); got != want {
			t.Errorf("%s: got %v, want %v", u, got, want)
		}
	}
	if tokenURLPattern(nil).MatchString("https://idp.example.com/") {
		t.Error("an empty host list matched")
	}
}
