package proxy

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/e2e/mockenv"
	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/vault"
)

type portal struct {
	p   *Proxy
	s   *store.Store
	env *mockenv.Env
	b   *mockenv.ExpenseBrowser
	api store.API
}

func expensePortal(t *testing.T) *portal {
	t.Helper()
	ctx := context.Background()
	env, err := mockenv.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(env.Close)
	s, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	v, _ := vault.Open(&vault.FakeKeychain{}, t.TempDir())
	base := env.URLs["expenses"]
	api := store.API{ID: "exp", Name: "expenses", Auth: store.Auth{Type: "browser", StartURL: base + "/expenses/claims"}, FilesDir: t.TempDir()}
	_ = s.PutAPI(ctx, api)
	_ = s.PutVariable(ctx, store.Variable{ID: "v1", APIID: "exp", Name: "baseUrl", Source: "literal", Value: base})
	for _, r := range []store.Request{
		{ID: "list", Name: "listClaims", Method: "GET", URL: "{{baseUrl}}/expenses/claims", Access: "read"},
		{ID: "newForm", Name: "newClaimForm", Method: "GET", URL: "{{baseUrl}}/expenses/claims/new", Access: "read"},
		{ID: "create", Name: "createClaim", Method: "POST", URL: "{{baseUrl}}/expenses/claims", BodyType: "form", Body: "event=save", Access: "write"},
		{ID: "claim", Name: "getClaim", Method: "GET", URL: "{{baseUrl}}/expenses/claims/{id}", Access: "read"},
		{ID: "receipt", Name: "uploadReceipt", Method: "POST", URL: "{{baseUrl}}/expenses/claims/{id}/receipt", BodyType: "form", Access: "write"},
	} {
		r.APIID = "exp"
		_ = s.PutRequest(ctx, r)
	}
	b := &mockenv.ExpenseBrowser{Env: env}
	p := New(s, v, clock.Real{}, &clock.Seq{Prefix: "id"}, nil)
	p.SetBrowser(b)
	return &portal{p: p, s: s, env: env, b: b, api: api}
}

var agent = Actor{Kind: "agent", ID: "a1", Name: "copilot"}

func (pt *portal) do(t *testing.T, req string, args Args) (*Response, error) {
	t.Helper()
	return pt.p.Do(context.Background(), Call{APIID: "exp", RequestID: req, Args: args, Actor: agent, Access: "read_write"})
}

var stateRe = regexp.MustCompile(`name="_state" value="([^"]+)"`)

// REQ: AUTH-5
func TestBrowserSignIn_OpensOnceAndKeepsOnlyThePortalsCookies(t *testing.T) {
	pt := expensePortal(t)
	r, err := pt.do(t, "list", Args{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != 200 || !strings.Contains(string(r.Body), "Expense reports") {
		t.Fatalf("status %d body %.200s", r.Status, r.Body)
	}
	if _, err := pt.do(t, "list", Args{}); err != nil {
		t.Fatal(err)
	}
	if n := pt.b.Opened.Load(); n != 1 {
		t.Fatalf("browser opened %d times, want 1", n)
	}
	if n := pt.env.Count("expenses.signin"); n != 1 {
		t.Fatalf("signed in %d times", n)
	}
	for _, c := range pt.p.jars["exp"].Cookies(mustURL("https://login.corp.example/")) {
		t.Fatalf("a sign-in service cookie reached the portal jar: %s", c.Name)
	}
	for k, v := range r.Headers {
		if strings.EqualFold(k, "Set-Cookie") && v[0] != "[masked]" {
			t.Fatal("Set-Cookie returned unmasked")
		}
	}
}

// REQ: AUTH-5
func TestBrowserSignIn_ExpiryRetriesReadsButNeverResendsWrites(t *testing.T) {
	pt := expensePortal(t)
	if _, err := pt.do(t, "list", Args{}); err != nil {
		t.Fatal(err)
	}
	pt.env.ExpireExpenses()
	r, err := pt.do(t, "list", Args{})
	if err != nil || r.Status != 200 {
		t.Fatalf("read after expiry: %v %v", r, err)
	}
	if n := pt.b.Opened.Load(); n != 2 {
		t.Fatalf("browser opened %d times, want 2", n)
	}
	page, _ := pt.do(t, "newForm", Args{})
	state := stateRe.FindStringSubmatch(string(page.Body))[1]
	pt.env.ExpireExpenses()
	_, err = pt.do(t, "create", Args{Flat: map[string]any{"_state": state, "date": "2026-11-01", "amount": "10", "purpose": "x"}})
	if !IsCode(err, CodeSigninRequired) {
		t.Fatalf("write after expiry: %v, want signin_required", err)
	}
	if n := pt.env.Count("expenses.create"); n != 0 {
		t.Fatalf("claim created %d times after the session ended", n)
	}
}

// REQ: AUTH-5
func TestBrowserSignIn_ClosedWindowSaysSignInIsNeeded(t *testing.T) {
	pt := expensePortal(t)
	pt.b.Decline.Store(true)
	_, err := pt.do(t, "list", Args{})
	if !IsCode(err, CodeSigninRequired) {
		t.Fatalf("got %v", err)
	}
}

// REQ: AUTH-5
func TestSignedInTest(t *testing.T) {
	ok := SignedInTest(store.Auth{StartURL: "https://exp.corp/OA_HTML/claims"})
	for u, want := range map[string]bool{
		"https://exp.corp/OA_HTML/claims?x=1":          true,
		"https://exp.corp/OA_HTML/claims/12":           true,
		"https://login.corp/saml":                      false,
		"https://exp.corp/OA_HTML/claims/login?next=/": false,
		"https://exp.corp/other":                       false,
	} {
		if ok(u) != want {
			t.Errorf("%s: got %v", u, !want)
		}
	}
	custom := SignedInTest(store.Auth{StartURL: "https://exp.corp/", SignedIn: "/Home"})
	if !custom("https://exp.corp/OA_HTML/Home") || custom("https://exp.corp/") {
		t.Error("custom signed-in text not used")
	}
}

// REQ: HTTP-3
func TestUpload_SendsMultipartFromTheFilesFolderOnly(t *testing.T) {
	pt := expensePortal(t)
	receipt := []byte("%PDF-1.4 november receipt bytes")
	if err := os.WriteFile(filepath.Join(pt.api.FilesDir, "november.pdf"), receipt, 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.pdf")
	_ = os.WriteFile(outside, []byte("private"), 0o600)
	_ = os.Symlink(outside, filepath.Join(pt.api.FilesDir, "link.pdf"))

	page, _ := pt.do(t, "newForm", Args{})
	state := stateRe.FindStringSubmatch(string(page.Body))[1]
	r, err := pt.do(t, "create", Args{Flat: map[string]any{"_state": state, "date": "2026-11-01", "amount": "52.10", "purpose": "Internet"}})
	if err != nil || r.Status != 302 {
		t.Fatalf("create: %v %v", r, err)
	}
	id := strings.TrimPrefix(r.Headers["Location"][0], "/expenses/claims/")
	for _, bad := range []string{"../secret.pdf", "link.pdf", "missing.pdf", filepath.Join("..", "x"), ""} {
		page, _ = pt.do(t, "claim", Args{Path: map[string]any{"id": id}})
		state = stateRe.FindStringSubmatch(string(page.Body))[1]
		_, err := pt.do(t, "receipt", Args{Path: map[string]any{"id": id}, Body: map[string]any{"_state": state, "receipt": map[string]any{"$file": bad}}, HasBody: true})
		if !IsCode(err, CodeNoFile) {
			t.Errorf("%q: got %v, want file_not_found", bad, err)
		}
	}
	page, _ = pt.do(t, "claim", Args{Path: map[string]any{"id": id}})
	state = stateRe.FindStringSubmatch(string(page.Body))[1]
	r, err = pt.do(t, "receipt", Args{Path: map[string]any{"id": id}, Flat: map[string]any{"_state": state, "receipt": map[string]any{"$file": "november.pdf"}}})
	if err != nil || r.Status != 302 {
		t.Fatalf("upload: %v %+v", err, r)
	}
	if n := pt.env.Count("expenses.upload"); n != 1 {
		t.Fatalf("uploads %d", n)
	}
	calls, _ := pt.s.Calls(context.Background(), store.CallFilter{})
	for _, c := range calls {
		if strings.Contains(c.Input, "november receipt bytes") {
			t.Fatal("the call log kept the file's contents")
		}
		if strings.Contains(c.Input, state) {
			t.Fatal("the call log kept the page state")
		}
	}
	files, err := pt.p.ListFiles(context.Background(), "exp")
	if err != nil || len(files) != 1 || files[0].Name != "november.pdf" {
		t.Fatalf("files %+v %v", files, err)
	}
}

// REQ: CONF-1
func TestConfirm_WritesWaitForTheConfirmation(t *testing.T) {
	pt := expensePortal(t)
	call := Call{APIID: "exp", RequestID: "create", Actor: agent, Access: "read_write", NeedConfirm: true,
		Args: Args{Flat: map[string]any{"date": "2026-11-01", "amount": "1", "purpose": "x"}}}
	if _, err := pt.p.Do(context.Background(), call); !IsCode(err, CodeConfirmRequired) {
		t.Fatalf("unconfirmed write: %v", err)
	}
	call.RequestID, call.Args = "list", Args{}
	if _, err := pt.p.Do(context.Background(), call); err != nil {
		t.Fatalf("reads don't need confirming: %v", err)
	}
}

// REQ: HTTP-3
func TestMaskBody_HidesFormStateAndSecrets(t *testing.T) {
	got := string(MaskBody([]byte("_state=abc123&password=pw&csrf_token=t&__VIEWSTATE=v&amount=10"), "application/x-www-form-urlencoded"))
	for _, leak := range []string{"abc123", "pw&", "=t&", "=v&"} {
		if strings.Contains(got+"&", leak) {
			t.Errorf("%q kept in %s", leak, got)
		}
	}
	if !strings.Contains(got, "amount=10") {
		t.Errorf("business field lost: %s", got)
	}
}

func mustURL(s string) *url.URL { u, _ := url.Parse(s); return u }
