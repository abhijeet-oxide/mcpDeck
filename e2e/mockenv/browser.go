package mockenv

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/abhijeet-oxide/mcpdeck/internal/browser"
)

// ExpenseBrowser stands in for the person and their browser on the
// expense portal: it signs in with the mock's user, password and one-time
// code, and when recording it clicks through one claim (open the list,
// create, upload a receipt, submit) the way a person would.
type ExpenseBrowser struct {
	Env *Env
	// Opened counts browser windows, for assertions.
	Opened atomic.Int32
	// Decline makes the person close the window instead of signing in.
	Decline atomic.Bool
}

const fakeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36 Edg/142.0.0.0"

type recorder struct {
	cl    *http.Client
	steps []browser.Step
}

func (r *recorder) do(ctx context.Context, method, u, ct string, body []byte) (string, string, error) {
	for i := 0; i < 5; i++ {
		req, _ := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
		req.Header.Set("User-Agent", fakeUA)
		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		res, err := r.cl.Do(req)
		if err != nil {
			return "", "", err
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		st := browser.Step{Method: method, URL: u, Type: "Document", Status: res.StatusCode, Location: res.Header.Get("Location"), MimeType: strings.Split(res.Header.Get("Content-Type"), ";")[0]}
		if body != nil {
			st.ContentType, st.PostData = ct, string(body)
		}
		r.steps = append(r.steps, st)
		if res.StatusCode < 300 || res.StatusCode >= 400 {
			return u, string(b), nil
		}
		next, _ := url.Parse(u)
		loc, _ := next.Parse(res.Header.Get("Location"))
		u, method, body, ct = loc.String(), http.MethodGet, nil, ""
	}
	return "", "", errors.New("too many redirects")
}

var stateRe = regexp.MustCompile(`name="_state" value="([^"]+)"`)

func (b *ExpenseBrowser) open(ctx context.Context, start string) (*recorder, string, error) {
	b.Opened.Add(1)
	if b.Decline.Load() {
		return nil, "", browser.ErrClosed
	}
	jar, _ := cookiejar.New(nil)
	r := &recorder{cl: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	at, page, err := r.do(ctx, http.MethodGet, start, "", nil)
	if err != nil {
		return nil, "", err
	}
	if strings.Contains(at, "/sso/authorize") {
		ret := regexp.MustCompile(`name="return" value="([^"]*)"`).FindStringSubmatch(page)
		form := url.Values{"user": {ExpenseUser}, "password": {ExpensePass}, "otp": {ExpenseOTP}}
		if len(ret) == 2 {
			form.Set("return", strings.ReplaceAll(ret[1], "&amp;", "&"))
		}
		at, page, err = r.do(ctx, http.MethodPost, b.Env.URLs["expenses"]+"/sso/authorize", "application/x-www-form-urlencoded", []byte(form.Encode()))
		if err != nil {
			return nil, "", err
		}
	}
	_ = at
	return r, page, nil
}

func (b *ExpenseBrowser) session(r *recorder) browser.Session {
	u, _ := url.Parse(b.Env.URLs["expenses"])
	s := browser.Session{UserAgent: fakeUA}
	for _, c := range r.cl.Jar.Cookies(u) {
		s.Cookies = append(s.Cookies, browser.Cookie{Name: c.Name, Value: c.Value, Domain: u.Hostname(), Path: "/", HTTPOnly: true, Expires: -1})
	}
	// A cookie of the sign-in service on another host, which must not reach the portal's jar.
	s.Cookies = append(s.Cookies, browser.Cookie{Name: "ESTSAUTH", Value: "idp-cookie-C4N4RY", Domain: ".login.corp.example", Path: "/", Secure: true})
	return s
}

// SignIn implements browser.Opener.
func (b *ExpenseBrowser) SignIn(ctx context.Context, _, start string, signedIn func(string) bool) (browser.Session, error) {
	r, _, err := b.open(ctx, start)
	if err != nil {
		return browser.Session{}, err
	}
	last := r.steps[len(r.steps)-1].URL
	if !signedIn(last) {
		return browser.Session{}, errors.New("not signed in at " + last)
	}
	return b.session(r), nil
}

// Record implements browser.Opener: one claim, created, with a receipt, submitted.
func (b *ExpenseBrowser) Record(ctx context.Context, _, start string) (browser.Session, []browser.Step, error) {
	r, _, err := b.open(ctx, start)
	if err != nil {
		return browser.Session{}, nil, err
	}
	base := b.Env.URLs["expenses"]
	_, page, err := r.do(ctx, http.MethodGet, base+"/expenses/claims/new", "", nil)
	if err != nil {
		return browser.Session{}, nil, err
	}
	st := stateRe.FindStringSubmatch(page)
	form := url.Values{"_state": {st[1]}, "event": {"save"}, "date": {"2026-11-03"}, "amount": {"52.10"}, "category": {"Internet"}, "purpose": {"Home internet November"}}
	at, page, err := r.do(ctx, http.MethodPost, base+"/expenses/claims", "application/x-www-form-urlencoded", []byte(form.Encode()))
	if err != nil {
		return browser.Session{}, nil, err
	}
	sts := stateRe.FindAllStringSubmatch(page, -1)
	if len(sts) < 2 {
		return browser.Session{}, nil, errors.New("no receipt form on " + at)
	}
	var mb bytes.Buffer
	w := multipart.NewWriter(&mb)
	_ = w.WriteField("_state", sts[0][1])
	fw, _ := w.CreatePart(map[string][]string{"Content-Disposition": {`form-data; name="receipt"; filename="november-internet.pdf"`}, "Content-Type": {"application/pdf"}})
	_, _ = fw.Write([]byte("%PDF-1.4 receipt"))
	_ = w.Close()
	// Browsers leave file contents out of what they report for an upload.
	_, page, err = r.do(ctx, http.MethodPost, at+"/receipt", w.FormDataContentType(), mb.Bytes())
	if err != nil {
		return browser.Session{}, nil, err
	}
	for i := range r.steps {
		if strings.HasSuffix(r.steps[i].URL, "/receipt") {
			r.steps[i].PostData = strings.Replace(r.steps[i].PostData, "%PDF-1.4 receipt", "", 1)
		}
	}
	sts = stateRe.FindAllStringSubmatch(page, -1)
	_, _, err = r.do(ctx, http.MethodPost, at+"/submit", "application/x-www-form-urlencoded", []byte(url.Values{"_state": {sts[len(sts)-1][1]}}.Encode()))
	if err != nil {
		return browser.Session{}, nil, err
	}
	return b.session(r), r.steps, nil
}
