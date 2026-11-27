package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/browser"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// CodeSigninRequired is a call that needs a person to sign in (AUTH-5).
const CodeSigninRequired = "signin_required"

// DefaultSigninTimeout is how long a sign-in window waits for the person.
const DefaultSigninTimeout = 3 * time.Minute

// signin is one browser sign-in in progress for an API.
type signin struct {
	done chan struct{}
	err  error
}

// SetBrowser sets how mcpDeck opens a browser for a person (AUTH-5, REC-1).
func (p *Proxy) SetBrowser(b browser.Opener) {
	p.mu.Lock()
	p.browser = b
	p.mu.Unlock()
}

// Browser is the opener, or nil.
func (p *Proxy) Browser() browser.Opener {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.browser
}

// SignedInTest is the "signed in" check for a browser auth setup: the page
// URL contains SignedIn, or else is on the start URL's host and path and
// doesn't look like a sign-in page.
func SignedInTest(a store.Auth) func(string) bool {
	return func(u string) bool {
		if a.SignedIn != "" {
			return strings.Contains(u, a.SignedIn)
		}
		su, err1 := url.Parse(a.StartURL)
		pu, err2 := url.Parse(u)
		if err1 != nil || err2 != nil || !strings.EqualFold(su.Host, pu.Host) {
			return false
		}
		if looksLikeSignIn(pu.Path + "?" + pu.RawQuery) {
			return false
		}
		return strings.HasPrefix(pu.Path, su.Path)
	}
}

func looksLikeSignIn(s string) bool {
	s = strings.ToLower(s)
	for _, w := range []string{"login", "signin", "sign_in", "sign-in", "/sso", "/auth", "saml", "oauth"} {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// hasBrowserSession reports a browser sign-in that is still good.
func (p *Proxy) hasBrowserSession(apiID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.browserUA[apiID]
	return ok
}

// dropBrowserSession forgets an API's browser cookies.
func (p *Proxy) dropBrowserSession(apiID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.browserUA, apiID)
	jar, _ := cookiejar.New(nil)
	p.jars[apiID] = jar
}

// UseBrowserSession loads cookies from a browser into an API's jar: only
// those of its allowed hosts (AUTH-5).
func (p *Proxy) UseBrowserSession(ctx context.Context, api store.API, s browser.Session) int {
	allowed := p.AllowedHosts(ctx, api)
	jar, _ := cookiejar.New(nil)
	n := 0
	for _, c := range s.Cookies {
		d := strings.TrimPrefix(c.Domain, ".")
		for _, h := range allowed {
			hostOnly := h
			if i := strings.LastIndex(h, ":"); i > 0 && !strings.Contains(h[i:], "]") {
				hostOnly = h[:i]
			}
			if !strings.EqualFold(hostOnly, d) && !strings.HasSuffix(strings.ToLower(hostOnly), "."+strings.ToLower(d)) {
				continue
			}
			hc := &http.Cookie{Name: c.Name, Value: c.Value, Path: c.Path, Secure: c.Secure, HttpOnly: c.HTTPOnly}
			if strings.HasPrefix(c.Domain, ".") {
				hc.Domain = d
			}
			if c.Expires > 0 {
				hc.Expires = time.Unix(int64(c.Expires), 0)
			}
			scheme := "http"
			if c.Secure {
				scheme = "https"
			}
			jar.SetCookies(&url.URL{Scheme: scheme, Host: h, Path: "/"}, []*http.Cookie{hc})
			n++
			break
		}
	}
	p.mu.Lock()
	p.jars[api.ID] = jar
	p.browserUA[api.ID] = s.UserAgent
	p.mu.Unlock()
	return n
}

// SignIn opens the browser for a person to sign in to api now, and waits.
func (p *Proxy) SignIn(ctx context.Context, apiID string) error {
	api, err := p.store.API(ctx, apiID)
	if err != nil {
		return &Error{Code: CodeNotFound, Message: "API not found"}
	}
	if api.Auth.Type != "browser" {
		return &Error{Code: CodeLoginFailed, Message: "the " + api.Name + " application doesn't sign in through a browser; set its auth type to Browser sign-in"}
	}
	p.dropBrowserSession(api.ID)
	return p.ensureBrowser(ctx, api, api.Auth)
}

// ensureBrowser makes sure api has a browser session. Without one it opens
// the browser once (later callers wait on the same window) and waits for
// the person until ctx ends; the window stays open after that, so the next
// call finds the session.
func (p *Proxy) ensureBrowser(ctx context.Context, api store.API, a store.Auth) error {
	if p.hasBrowserSession(api.ID) {
		return nil
	}
	b := p.Browser()
	if b == nil {
		return &Error{Code: CodeSigninRequired, Message: "the " + api.Name + " application needs a browser sign-in, which this mcpDeck can't open; run mcpDeck on your computer"}
	}
	if a.StartURL == "" {
		return &Error{Code: CodeSigninRequired, Message: "the " + api.Name + " application has no sign-in URL; set it on its Authentication tab"}
	}
	p.mu.Lock()
	s := p.signins[api.ID]
	if s == nil {
		s = &signin{done: make(chan struct{})}
		p.signins[api.ID] = s
		go func() {
			sctx, cancel := context.WithTimeout(context.Background(), p.SigninTimeout)
			defer cancel()
			sess, err := b.SignIn(sctx, api.ID, a.StartURL, SignedInTest(a))
			if err == nil {
				p.UseBrowserSession(sctx, api, sess)
			}
			p.mu.Lock()
			s.err = err
			delete(p.signins, api.ID)
			p.mu.Unlock()
			close(s.done)
		}()
	}
	p.mu.Unlock()
	select {
	case <-s.done:
		if s.err != nil {
			msg := "sign-in to " + api.Name + " didn't finish: " + s.err.Error() + ". Call again to open the sign-in window."
			if errors.Is(s.err, browser.ErrNoBrowser) {
				msg = "sign-in to " + api.Name + " needs Edge or Chrome on this computer; install one or set MCPDECK_BROWSER"
			}
			return &Error{Code: CodeSigninRequired, Message: msg}
		}
		return nil
	case <-ctx.Done():
		return &Error{Code: CodeSigninRequired, Message: "the user needs to sign in to " + api.Name + ": mcpDeck opened a browser window for it. Ask them to sign in there, then call this again."}
	}
}

// browserUserAgent is the User-Agent of the browser the session came from.
func (p *Proxy) browserUserAgent(apiID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.browserUA[apiID]
}

// redirectsOff reports a redirect to a host the API doesn't allow, such as a
// company sign-in page: the browser session has ended.
func redirectsOff(hr *http.Response, allowed []string) bool {
	if hr.StatusCode < 300 || hr.StatusCode >= 400 {
		return false
	}
	u, err := url.Parse(hr.Header.Get("Location"))
	if err != nil || u.Host == "" {
		return false
	}
	return !hostAllowed(u.Host, allowed)
}
