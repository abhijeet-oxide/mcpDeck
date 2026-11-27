package browser_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/e2e/mockenv"
	"github.com/abhijeet-oxide/mcpdeck/internal/browser"
)

// person fills the mock portal's sign-in form, the way a person would.
func person(t *testing.T, then string) func(ctx context.Context, p *browser.Page) {
	return func(ctx context.Context, p *browser.Page) {
		for ctx.Err() == nil {
			v, err := p.Eval(ctx, `location.pathname`)
			if err != nil {
				return
			}
			var path string
			_ = json.Unmarshal(v, &path)
			switch {
			case path == "/sso/authorize":
				_, _ = p.Eval(ctx, `(function(){const f=document.forms[0];if(!f||!f.user)return;f.user.value=`+q(mockenv.ExpenseUser)+`;f.password.value=`+q(mockenv.ExpensePass)+`;f.otp.value=`+q(mockenv.ExpenseOTP)+`;f.submit()})()`)
			case path == "/expenses/claims" && then != "":
				js := then
				then = ""
				_, _ = p.Eval(ctx, js)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func q(s string) string { b, _ := json.Marshal(s); return string(b) }

func chrome(t *testing.T) *browser.Chrome {
	t.Helper()
	bin, err := browser.Find()
	if err != nil {
		// The Playwright build some CI images carry.
		bin, err = "/opt/pw-browsers/chromium", nil
		if _, serr := os.Stat(bin); serr != nil {
			err = serr
		}
	}
	if err != nil {
		t.Skip("no Chromium-family browser installed")
	}
	return &browser.Chrome{Exec: bin, Dir: t.TempDir(), Headless: true}
}

// REQ: AUTH-5
func TestSignInReturnsThePortalSession(t *testing.T) {
	b := chrome(t)
	env, err := mockenv.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	b.OnPage = person(t, "")
	base := env.URLs["expenses"]
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s, err := b.SignIn(ctx, "expenses", base+"/expenses/claims", func(u string) bool {
		return strings.HasPrefix(u, base+"/expenses/")
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range s.Cookies {
		if c.Name == "EXPSESSIONID" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no portal session cookie in %+v", s.Cookies)
	}
	if s.UserAgent == "" || strings.Contains(s.UserAgent, "Headless") {
		t.Fatalf("user agent %q", s.UserAgent)
	}
}

// REQ: REC-1
func TestRecordSeesPagesAndFormPosts(t *testing.T) {
	b := chrome(t)
	env, err := mockenv.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	// After sign-in: open the new-claim form, fill it and save, then close
	// the tab, as a person finishing would.
	b.OnPage = person(t, `(async function(){
		location.href='/expenses/claims/new';
	})()`)
	base := env.URLs["expenses"]
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	go func() {
		// The person fills the form once it's open, then closes the window.
		for ctx.Err() == nil {
			time.Sleep(300 * time.Millisecond)
			for _, c := range env.ExpenseClaims() {
				if c["purpose"] == "Recorded" {
					time.Sleep(500 * time.Millisecond)
					cancel()
					return
				}
			}
		}
	}()
	prev := b.OnPage
	b.OnPage = func(ctx context.Context, p *browser.Page) {
		go prev(ctx, p)
		for ctx.Err() == nil {
			v, _ := p.Eval(ctx, `location.pathname`)
			if string(v) == `"/expenses/claims/new"` {
				_, _ = p.Eval(ctx, `(function(){const f=document.forms.claimForm;f.date.value='2026-11-01';f.amount.value='50.00';f.purpose.value='Recorded';submitForm('claimForm','save')})()`)
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
	s, steps, _ := b.Record(ctx, "expenses", base+"/expenses/claims")
	var post *browser.Step
	for i := range steps {
		if steps[i].Method == "POST" && strings.HasSuffix(steps[i].URL, "/expenses/claims") {
			post = &steps[i]
		}
	}
	if post == nil {
		t.Fatalf("no form post recorded: %+v", steps)
	}
	if post.Status != 302 || !strings.Contains(post.Location, "/expenses/claims/") {
		t.Fatalf("post status %d location %q", post.Status, post.Location)
	}
	if !strings.Contains(post.PostData, "_state=") || !strings.Contains(post.ContentType, "x-www-form-urlencoded") {
		t.Fatalf("post data %q type %q", post.PostData, post.ContentType)
	}
	if len(s.Cookies) == 0 {
		t.Fatal("no cookies kept from the recording")
	}
}

// REQ: AUTH-5
// The start page answers slowly with a redirect to sign-in, as company
// portals do. The browser reports the start URL before the redirect, so
// sign-in must wait until the loaded page itself is past sign-in.
func TestSignInWaitsForTheLoadedPageNotTheRequestedURL(t *testing.T) {
	b := chrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/app":
			if c, err := r.Cookie("sid"); err == nil && c.Value == "ok" {
				_, _ = w.Write([]byte("<h1>Signed in</h1>"))
				return
			}
			time.Sleep(1500 * time.Millisecond)
			http.Redirect(w, r, "/login", http.StatusFound)
		case "/login":
			// The person signs in after a moment.
			_, _ = w.Write([]byte(`<script>setTimeout(()=>{document.cookie='sid=ok; path=/';location.href='/app'},800)</script>`))
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s, err := b.SignIn(ctx, "slow", srv.URL+"/app", func(u string) bool { return strings.HasPrefix(u, srv.URL+"/app") })
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range s.Cookies {
		if c.Name == "sid" && c.Value == "ok" {
			found = true
		}
	}
	if !found {
		t.Fatalf("sign-in returned before the person signed in: cookies %+v", s.Cookies)
	}
}

// REQ: REC-1
// A page the person leaves at once is still kept, as the server sent it.
func TestRecordKeepsPagesLeftAtOnce(t *testing.T) {
	b := chrome(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/list" {
			_, _ = w.Write([]byte(`<table><tr><th>Report</th><th>Status</th></tr><tr><td>1</td><td>Draft</td></tr></table><form name="f" method="post" action="/x"><input type="hidden" name="_s" value="1"></form><script>location.href='/next'</script>`))
			return
		}
		// The person has moved on: /list was kept before it could. Done.
		once.Do(cancel)
		_, _ = w.Write([]byte("<p>next</p>"))
	}))
	defer srv.Close()
	_, steps, _ := b.Record(ctx, "fast", srv.URL+"/list")
	for _, st := range steps {
		if strings.HasSuffix(st.URL, "/list") {
			if !strings.Contains(st.HTML, "<th>Report</th>") || !strings.Contains(st.HTML, `name="_s"`) {
				t.Fatalf("page left at once wasn't kept: %q", st.HTML)
			}
			return
		}
	}
	t.Fatalf("no /list step: %+v", steps)
}
