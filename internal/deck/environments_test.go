package deck_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

func newAPI(t *testing.T, d *deck.Deck) *store.API {
	t.Helper()
	a, err := d.CreateAPI(context.Background(), deck.CreateAPIInput{Name: "scan", Title: "Scanner"})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// REQ: ENV-1, ENV-2
func TestEnvironmentRenameAndImport(t *testing.T) {
	ctx := context.Background()
	d := newDeck(t)
	a := newAPI(t, d)
	envs, _ := d.Store.Environments(ctx, a.ID)
	if len(envs) != 1 {
		t.Fatalf("want the default environment, got %v", envs)
	}
	e, err := d.RenameEnvironment(ctx, envs[0].ID, "Local")
	if err != nil || e.Name != "Local" {
		t.Fatalf("rename: %v %v", e, err)
	}
	if _, err := d.RenameEnvironment(ctx, envs[0].ID, " "); err == nil {
		t.Fatal("an empty name must be refused")
	}

	file := `{"name":"Local","values":[{"key":"baseUrl","value":"https://scan.example"},{"key":"password","value":"hunter2","type":"secret"}]}`
	prev, err := d.ImportEnvironment(ctx, a.ID, deck.ImportEnvironmentInput{Filename: "Local.postman_environment.json", Content: file, Preview: true})
	if err != nil || prev.Environment != nil || len(prev.Parsed.Vars) != 2 {
		t.Fatalf("preview: %+v %v", prev, err)
	}
	res, err := d.ImportEnvironment(ctx, a.ID, deck.ImportEnvironmentInput{Filename: "Local.postman_environment.json", Content: file})
	if err != nil {
		t.Fatal(err)
	}
	if res.Environment.Name != "Local 2" {
		t.Errorf("a clashing name gets a number: got %q", res.Environment.Name)
	}
	vars, _ := d.Store.Variables(ctx, a.ID)
	var sawSecret bool
	for _, v := range vars {
		if v.EnvID != res.Environment.ID {
			continue
		}
		if v.Name == "password" {
			sawSecret = true
			if v.Source != "secret" || v.Value != "" || v.Ciphertext == "" {
				t.Errorf("the secret must be sealed, got %+v", v)
			}
		}
	}
	if !sawSecret {
		t.Fatal("password variable missing")
	}
	b, _ := json.Marshal(res)
	if strings.Contains(string(b), "hunter2") {
		t.Fatal("the import result leaks the secret value") // CRED-4
	}
}

// REQ: EX-3
func TestSaveResponseKeepsOnePerStatus(t *testing.T) {
	ctx := context.Background()
	d := newDeck(t)
	a := newAPI(t, d)
	r, err := d.SaveRequest(ctx, a.ID, store.Request{Name: "login", Method: "POST", URL: "{{baseUrl}}/login"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveResponse(ctx, r.ID, deck.SaveResponseInput{Status: 401, Body: json.RawMessage(`{"error":"bad password"}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveResponse(ctx, r.ID, deck.SaveResponseInput{Status: 200, Body: json.RawMessage(`{"token":"t1","user":"a"}`)}); err != nil {
		t.Fatal(err)
	}
	got, err := d.SaveResponse(ctx, r.ID, deck.SaveResponseInput{Status: 200, Name: "Logged in", Body: json.RawMessage(`{"token":"t2","user":"b"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Responses) != 2 || got.Responses[0].Status != 200 || got.Responses[0].Name != "Logged in" || got.Responses[1].Name != "Not allowed" {
		t.Fatalf("responses = %+v", got.Responses)
	}
	if strings.Contains(string(got.Example), "t2") || !strings.Contains(string(got.Example), `"user":"b"`) {
		t.Errorf("the 2xx response becomes the masked example, got %s", got.Example)
	}
	got, _ = d.DeleteResponse(ctx, r.ID, got.Responses[1].ID)
	if len(got.Responses) != 1 {
		t.Fatalf("delete: %+v", got.Responses)
	}
	if _, err := d.SaveResponse(ctx, r.ID, deck.SaveResponseInput{Status: 200, Body: json.RawMessage(`<html>`)}); err == nil {
		t.Fatal("non-JSON must be refused")
	}
}

// REQ: TK-1
func TestDeleteToolkitWithAgents(t *testing.T) {
	ctx := context.Background()
	d := newDeck(t)
	name, desc := "sec", "x"
	tk, err := d.CreateToolkit(ctx, deck.ToolkitInput{Name: &name, Description: &desc})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Connect(ctx, deck.ConnectInput{Client: "cursor", ToolkitID: tk.ID, Name: "Cursor", Access: "read"}); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteToolkitForce(ctx, "sec", false); err == nil {
		t.Fatal("without force, a toolkit agents use is kept")
	}
	if err := d.DeleteToolkitForce(ctx, "sec", true); err != nil {
		t.Fatal(err)
	}
	if gs, _ := d.Store.Grants(ctx, "", tk.ID); len(gs) != 0 {
		t.Fatalf("grants left: %v", gs)
	}
}
