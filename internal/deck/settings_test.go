package deck_test

import (
	"context"
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
	want := "api send example toolkit tool agent"
	if got := strings.Join(keys, " "); got != want {
		t.Fatalf("steps = %q, want %q", got, want)
	}
	if h.Steps[1].Reason == "" || h.Steps[3].Reason != "" {
		t.Fatalf("send needs an application; a toolkit needs nothing: %+v", h.Steps)
	}
	if _, err := d.CreateAPI(ctx, deck.CreateAPIInput{Name: "crm", BaseURL: "http://127.0.0.1:1"}); err != nil {
		t.Fatal(err)
	}
	h, _ = d.Home(ctx)
	if !h.Steps[0].Done || h.Steps[1].Reason != "" {
		t.Fatalf("with an application, sending is the next step: %+v", h.Steps)
	}
}
