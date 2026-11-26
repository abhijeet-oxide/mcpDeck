package deck_test

import (
	"context"
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// REQ: TOOL-10
// The Petstore's findPetsByStatus has the summary "Finds Pets by status." and
// a description that starts with a usage note; the tool should say what it does.
func TestGenerateTool_DescriptionFromSummary(t *testing.T) {
	ctx := context.Background()
	d := newDeck(t)
	a := newAPI(t, d)
	if _, err := d.SaveRequest(ctx, a.ID, store.Request{Name: "findPetsByStatus", Method: "GET", URL: "{{baseUrl}}/pet/findByStatus",
		Title: "Finds Pets by status.", Description: "Multiple status values can be provided with comma separated strings."}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveRequest(ctx, a.ID, store.Request{Name: "getThing", Method: "GET", URL: "{{baseUrl}}/thing",
		Title: "getThing", Description: "Returns one thing. More text."}); err != nil {
		t.Fatal(err)
	}
	g, err := d.GenerateTool(ctx, deck.GenerateInput{From: "scan.findPetsByStatus"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.Source, `description: "Finds Pets by status."`) {
		t.Errorf("want the summary as the description:\n%s", g.Source)
	}
	g, err = d.GenerateTool(ctx, deck.GenerateInput{From: "scan.getThing"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.Source, `description: "Returns one thing."`) {
		t.Errorf("a summary that only repeats the name falls back to the description:\n%s", g.Source)
	}
}
