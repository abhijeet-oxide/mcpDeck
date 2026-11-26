package deck_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// REQ: RNR-1
func TestSend_UsesThePathVariablesSetOnTheRequest(t *testing.T) {
	ctx := context.Background()
	d := newDeck(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"path":"`+r.URL.Path+`"}`)
	}))
	t.Cleanup(srv.Close)
	a, err := d.CreateAPI(ctx, deck.CreateAPIInput{Name: "billing", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	draft := store.Request{Name: "getSummary", Method: "GET", URL: "{{baseUrl}}/customers/{id}/summary"}
	res, _ := d.Send(ctx, deck.SendInput{APIID: a.ID, Draft: &draft})
	if !strings.Contains(res.Error, "Path variables") {
		t.Fatalf("a missing path value should say where to set it: %q", res.Error)
	}
	draft.PathParams = []store.KV{{Key: "id", Value: "C 42"}}
	res, _ = d.Send(ctx, deck.SendInput{APIID: a.ID, Draft: &draft})
	if res.Error != "" || !strings.Contains(res.Body, `/customers/C 42/summary`) {
		t.Fatalf("got %q %q", res.Error, res.Body)
	}
	// Saved on the request, it is used when sending the saved request too.
	saved, err := d.SaveRequest(ctx, a.ID, draft)
	if err != nil {
		t.Fatal(err)
	}
	res, _ = d.Send(ctx, deck.SendInput{APIID: a.ID, RequestID: saved.ID})
	if !strings.Contains(res.Body, "/customers/C 42/summary") {
		t.Fatalf("saved request: %q %q", res.Error, res.Body)
	}
}
