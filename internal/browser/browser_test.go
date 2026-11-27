package browser_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
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
