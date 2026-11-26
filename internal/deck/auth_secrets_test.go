package deck_test

import (
	"context"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

func secretNames(t *testing.T, d *deck.Deck, apiID string) map[string]bool {
	t.Helper()
	vars, err := d.Store.Variables(context.Background(), apiID)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, v := range vars {
		if v.Source == "secret" {
			out[v.Name] = v.Ciphertext != ""
		}
	}
	return out
}

// Switching from a bearer token to a login request used to keep the old
// "{{token}}" in the saved auth, which recreated an empty "token" secret
// and warned that requests would fail.
// REQ: AUTH-1
func TestLoginAuthDoesNotAskForTheOldToken(t *testing.T) {
	ctx := context.Background()
	d := newDeck(t)
	a, err := d.CreateAPI(ctx, deck.CreateAPIInput{Name: "shop", Auth: store.Auth{Type: "bearer", Token: "{{token}}"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := secretNames(t, d, a.ID)["token"]; !ok {
		t.Fatal("a bearer auth creates its secret variable")
	}
	login, err := d.SaveRequest(ctx, a.ID, store.Request{Name: "login", Method: "POST", URL: "https://shop.example/login", BodyType: "json", Body: `{"user":"{{user}}","password":"{{password}}"}`})
	if err != nil {
		t.Fatal(err)
	}
	// The editor sends the whole form state, so the bearer token is still there.
	au := store.Auth{Type: "login", Token: "{{token}}", Steps: []store.LoginStep{{RequestID: login.ID, Captures: []store.Capture{{Name: "access_token", From: "json:access_token"}}}},
		Apply: []store.KV{{Key: "Authorization", Value: "Bearer {{login.access_token}}"}}}
	got, err := d.UpdateAPI(ctx, a.ID, deck.UpdateAPIInput{Auth: &au})
	if err != nil {
		t.Fatal(err)
	}
	if got.Auth.Token != "" {
		t.Errorf("the saved login auth keeps the bearer token %q", got.Auth.Token)
	}
	if _, ok := secretNames(t, d, a.ID)["token"]; ok {
		t.Error("the empty token secret from the bearer auth is still listed as a missing credential")
	}
	// Saving again must not bring it back.
	if _, err := d.UpdateAPI(ctx, a.ID, deck.UpdateAPIInput{Auth: &au}); err != nil {
		t.Fatal(err)
	}
	if _, ok := secretNames(t, d, a.ID)["token"]; ok {
		t.Error("saving the login auth again recreated the token secret")
	}
}

// A secret that has a value, or that a request still uses, stays.
// REQ: AUTH-1
func TestAuthChangeKeepsSecretsInUse(t *testing.T) {
	ctx := context.Background()
	d := newDeck(t)
	a, err := d.CreateAPI(ctx, deck.CreateAPIInput{Name: "shop", Auth: store.Auth{Type: "apikey", Key: "X-Key", Value: "{{apiKey}}", In: "header"}, Secret: "s3cret"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := d.UpdateAPI(ctx, a.ID, deck.UpdateAPIInput{Auth: &store.Auth{Type: "bearer", Token: "{{token}}"}})
	if b == nil {
		t.Fatal("bearer save failed")
	}
	if set, ok := secretNames(t, d, a.ID)["apiKey"]; !ok || !set {
		t.Error("a secret with a value must stay")
	}
	if _, err := d.SaveRequest(ctx, a.ID, store.Request{Name: "me", Method: "GET", URL: "https://shop.example/me", Headers: []store.KV{{Key: "X-T", Value: "{{token}}"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.UpdateAPI(ctx, a.ID, deck.UpdateAPIInput{Auth: &store.Auth{Type: "none"}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := secretNames(t, d, a.ID)["token"]; !ok {
		t.Error("a secret a request uses must stay")
	}
}
