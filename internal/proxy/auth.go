package proxy

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // some signing schemes put an MD5 of the body in the string to sign
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // HMAC-SHA1 is still what some APIs sign with
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// A session is what an API's sign-in produced: captured values (tokens,
// CSRF tokens, refresh tokens) and when they stop working (AUTH-2).
// Values are never shown to tools or agents; they only fill templates
// inside the proxy, and every value is redacted from what comes back.
type session struct {
	values  map[string]string
	expires time.Time
}

// defaultSessionTTL is how long a sign-in is reused when the API says nothing.
const defaultSessionTTL = 30 * time.Minute

func (p *Proxy) sessionLock(apiID string) *sync.Mutex {
	p.mu.Lock()
	defer p.mu.Unlock()
	m, ok := p.sessLocks[apiID]
	if !ok {
		m = &sync.Mutex{}
		p.sessLocks[apiID] = m
	}
	return m
}

// usesSession reports whether an auth setup signs in before calls.
func usesSession(a store.Auth) bool { return a.Type == "login" || a.Type == "oauth2" }

// normalizeLogin turns the one-step shorthand into steps and Apply headers.
func normalizeLogin(a store.Auth) store.Auth {
	if a.Type != "login" || len(a.Steps) > 0 || a.LoginRequestID == "" {
		return a
	}
	step := store.LoginStep{RequestID: a.LoginRequestID}
	switch {
	case strings.HasPrefix(a.Capture, "json:"):
		step.Captures = []store.Capture{{Name: "token", From: a.Capture}}
		if len(a.Apply) == 0 {
			a.Apply = []store.KV{{Key: "Authorization", Value: "Bearer {{login.token}}"}}
		}
	case a.Capture != "" && a.Capture != "cookie":
		step.Captures = []store.Capture{{Name: "token", From: "header:" + a.Capture}}
		if len(a.Apply) == 0 {
			a.Apply = []store.KV{{Key: a.Capture, Value: "{{login.token}}"}}
		}
	}
	a.Steps = []store.LoginStep{step}
	return a
}

// ensureSession signs in when there's no live session (or force is set,
// after the API refused the current one), then adds the session's values
// to res as {{login.<name>}} and to the redaction list.
func (p *Proxy) ensureSession(ctx context.Context, api store.API, a store.Auth, res *resolved, force bool) error {
	a = normalizeLogin(a)
	lock := p.sessionLock(api.ID)
	lock.Lock()
	defer lock.Unlock()
	p.mu.Lock()
	s := p.sessions[api.ID]
	p.mu.Unlock()
	now := p.clock.Now()
	if s == nil || force || !now.Before(s.expires) {
		var err error
		var prev map[string]string
		if s != nil {
			prev = s.values
		}
		s, err = p.signIn(ctx, api, a, *res, prev)
		if err != nil {
			p.mu.Lock()
			delete(p.sessions, api.ID)
			p.mu.Unlock()
			return err
		}
		p.mu.Lock()
		p.sessions[api.ID] = s
		p.mu.Unlock()
	}
	res.withSession(s)
	return nil
}

func (r *resolved) withSession(s *session) {
	if s == nil {
		return
	}
	vars := make(map[string]string, len(r.vars)+len(s.values))
	for k, v := range r.vars {
		vars[k] = v
	}
	for k, v := range s.values {
		vars["login."+k] = v
		if len(v) >= 6 {
			r.secrets = append(r.secrets, v)
		}
	}
	r.vars = vars
	r.secrets.sortLongestFirst()
}

// dropSession forgets an API's session but keeps a refresh token, so the
// next call refreshes instead of signing in from scratch.
func (p *Proxy) dropSession(apiID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if s := p.sessions[apiID]; s != nil {
		s.expires = time.Time{}
	}
}

func (p *Proxy) signIn(ctx context.Context, api store.API, a store.Auth, res resolved, prev map[string]string) (*session, error) {
	if a.Type == "oauth2" {
		return p.oauthToken(ctx, api, a, res, prev)
	}
	if len(a.Steps) == 0 {
		return nil, &Error{Code: CodeLoginFailed, Message: "the " + api.Name + " API signs in with a login request but none is chosen; pick one on its Auth tab"}
	}
	// A refresh first, while a refresh token works.
	if a.RefreshRequestID != "" && prev["refresh_token"] != "" {
		s := &session{values: map[string]string{}}
		for k, v := range prev {
			s.values[k] = v
		}
		r2 := res
		r2.withSession(s)
		if err := p.runStep(ctx, api, a, store.LoginStep{RequestID: a.RefreshRequestID, Captures: a.Steps[0].Captures}, &r2, s, true); err == nil {
			return s, nil
		}
	}
	s := &session{values: map[string]string{}}
	r := res
	for i, st := range a.Steps {
		if err := p.runStep(ctx, api, a, st, &r, s, i == len(a.Steps)-1); err != nil {
			return nil, err
		}
		r = res
		r.withSession(s)
	}
	return s, nil
}

// runStep sends one login request and captures values from its response.
func (p *Proxy) runStep(ctx context.Context, api store.API, a store.Auth, st store.LoginStep, res *resolved, s *session, last bool) error {
	b, _, err := p.build(ctx, api, Call{RequestID: st.RequestID}, *res)
	if err != nil {
		return &Error{Code: CodeLoginFailed, Message: "the " + api.Name + " login step can't be sent: " + err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, b.method, b.url, bytes.NewReader(b.body))
	if err != nil {
		return err
	}
	req.Header = b.headers
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "mcpdeck")
	}
	// A login step may carry its own auth (basic for a token endpoint).
	if r, err := p.store.Request(ctx, st.RequestID); err == nil && r.Auth != nil && r.Auth.Type != "" && r.Auth.Type != "inherit" && !usesSession(*r.Auth) {
		if err := p.applyAuth(ctx, api, *r.Auth, req, *res, b.body); err != nil {
			return err
		}
	}
	cl, err := p.client(api, 30*time.Second, *res)
	if err != nil {
		return err
	}
	hr, err := cl.Do(req)
	if err != nil {
		return &Error{Code: CodeLoginFailed, Message: res.secrets.String("the " + api.Name + " login request failed: " + err.Error())}
	}
	defer hr.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(hr.Body, 1<<20))
	if hr.StatusCode >= 400 {
		msg := fmt.Sprintf("signing in to %s returned %s", api.Name, hr.Status)
		if d := UpstreamMessage(body, hr.Header.Get("Content-Type")); d != "" {
			msg += ": " + d
		}
		return &Error{Code: CodeLoginFailed, Message: res.secrets.String(msg + ". Check the credentials on the " + api.Name + " Auth tab in mcpDeck.")}
	}
	for _, c := range st.Captures {
		v := capture(c.From, hr, body, cl, req.URL)
		if v == "" {
			return &Error{Code: CodeLoginFailed, Message: fmt.Sprintf("the %s login response has nothing at %s for %s; check the capture on its Auth tab", api.Name, c.From, c.Name)}
		}
		s.values[c.Name] = v
	}
	if last {
		ttl := defaultSessionTTL
		if a.ExpiresFrom != "" {
			if sec, ok := expiresIn(a.ExpiresFrom, body); ok {
				ttl = sec
			}
		}
		s.expires = p.clock.Now().Add(early(ttl))
	}
	return nil
}

// early renews a little before expiry, so a call doesn't race the deadline.
func early(ttl time.Duration) time.Duration {
	m := ttl / 5
	if m > 30*time.Second {
		m = 30 * time.Second
	}
	return ttl - m
}

func expiresIn(from string, body []byte) (time.Duration, bool) {
	if n, err := strconv.ParseFloat(from, 64); err == nil && n > 0 {
		return time.Duration(n * float64(time.Second)), true
	}
	if path, ok := strings.CutPrefix(from, "json:"); ok {
		switch v := jsonPath(body, path).(type) {
		case float64:
			return time.Duration(v * float64(time.Second)), v > 0
		case string:
			n, err := strconv.ParseFloat(v, 64)
			return time.Duration(n * float64(time.Second)), err == nil && n > 0
		}
	}
	return 0, false
}

func jsonPath(body []byte, path string) any {
	var v any
	if json.Unmarshal(body, &v) != nil {
		return nil
	}
	for _, k := range strings.Split(path, ".") {
		switch t := v.(type) {
		case map[string]any:
			v = t[k]
		case []any:
			i, err := strconv.Atoi(k)
			if err != nil || i < 0 || i >= len(t) {
				return nil
			}
			v = t[i]
		default:
			return nil
		}
	}
	return v
}

// capture reads one value: json:<path>, header:<name>, cookie:<name> or regex:<re>.
func capture(from string, hr *http.Response, body []byte, cl *http.Client, u *url.URL) string {
	kind, arg, _ := strings.Cut(from, ":")
	switch kind {
	case "json":
		switch v := jsonPath(body, arg).(type) {
		case string:
			return v
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
	case "header":
		return hr.Header.Get(arg)
	case "cookie":
		for _, c := range hr.Cookies() {
			if c.Name == arg {
				return c.Value
			}
		}
		if cl.Jar != nil {
			for _, c := range cl.Jar.Cookies(u) {
				if c.Name == arg {
					return c.Value
				}
			}
		}
	case "regex":
		re, err := regexp.Compile(arg)
		if err != nil {
			return ""
		}
		if m := re.FindSubmatch(body); len(m) > 1 {
			return string(m[1])
		}
	}
	return ""
}

// oauthToken gets an access token from an OAuth 2.0 token endpoint
// (RFC 6749 sections 4.3, 4.4 and 6), refreshing while it can.
func (p *Proxy) oauthToken(ctx context.Context, api store.API, a store.Auth, res resolved, prev map[string]string) (*session, error) {
	missing := map[string]bool{}
	tokenURL := res.expand(a.TokenURL, missing)
	id, secret := res.expand(a.ClientID, missing), res.expand(a.ClientSecret, missing)
	if len(missing) > 0 {
		return nil, unresolvedErr(api, missing)
	}
	if tokenURL == "" {
		return nil, &Error{Code: CodeLoginFailed, Message: "the " + api.Name + " API uses OAuth 2.0 but has no token URL; set it on its Auth tab"}
	}
	if u, err := url.Parse(tokenURL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, &Error{Code: CodeLoginFailed, Message: "the " + api.Name + " API's OAuth 2.0 token URL must be an absolute http or https URL; fix it on its Auth tab"}
	}
	grant := a.Grant
	if grant == "" {
		grant = "client_credentials"
	}
	try := func(form url.Values) (*session, error, bool) {
		if a.ClientAuth != "body" && id != "" {
			// RFC 6749 2.3.1: basic auth with form-encoded id and secret.
		} else {
			form.Set("client_id", id)
			if secret != "" {
				form.Set("client_secret", secret)
			}
		}
		if a.Scope != "" {
			form.Set("scope", res.expand(a.Scope, missing))
		}
		if a.Audience != "" {
			form.Set("audience", res.expand(a.Audience, missing))
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, err, false
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "mcpdeck")
		if a.ClientAuth != "body" && id != "" {
			req.SetBasicAuth(url.QueryEscape(id), url.QueryEscape(secret))
		}
		cl, err := p.client(api, 30*time.Second, res)
		if err != nil {
			return nil, err, false
		}
		hr, err := cl.Do(req)
		if err != nil {
			return nil, &Error{Code: CodeLoginFailed, Message: res.secrets.String("the " + api.Name + " token request failed: " + err.Error())}, true
		}
		defer hr.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(hr.Body, 1<<20))
		if hr.StatusCode >= 400 {
			msg := fmt.Sprintf("the %s token endpoint returned %s", api.Name, hr.Status)
			if d := UpstreamMessage(body, hr.Header.Get("Content-Type")); d != "" {
				msg += ": " + d
			}
			return nil, &Error{Code: CodeLoginFailed, Message: res.secrets.String(msg + ". Check the client ID and secret on the " + api.Name + " Auth tab in mcpDeck.")}, true
		}
		tok, _ := jsonPath(body, "access_token").(string)
		if tok == "" {
			return nil, &Error{Code: CodeLoginFailed, Message: "the " + api.Name + " token response has no access_token"}, true
		}
		s := &session{values: map[string]string{"access_token": tok}}
		if rt, _ := jsonPath(body, "refresh_token").(string); rt != "" {
			s.values["refresh_token"] = rt
		} else if prev["refresh_token"] != "" && grant != "client_credentials" {
			s.values["refresh_token"] = prev["refresh_token"]
		}
		ttl := time.Hour
		if d, ok := expiresIn("json:expires_in", body); ok {
			ttl = d
		}
		s.expires = p.clock.Now().Add(early(ttl))
		return s, nil, true
	}
	if rt := prev["refresh_token"]; rt != "" {
		if s, err, _ := try(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {rt}}); err == nil {
			return s, nil
		}
	}
	form := url.Values{"grant_type": {grant}}
	if grant == "password" {
		form.Set("username", res.expand(a.Username, missing))
		form.Set("password", res.expand(a.Password, missing))
		if len(missing) > 0 {
			return nil, unresolvedErr(api, missing)
		}
	}
	s, err, _ := try(form)
	return s, err
}

// sign computes an HMAC request signature (AUTH-4) and returns the
// values Apply headers can use as {{sign.*}}.
func sign(a store.Auth, req *http.Request, body []byte, res resolved, now time.Time) (map[string]string, error) {
	missing := map[string]bool{}
	secret := res.expand(a.Secret, missing)
	var nb [12]byte
	_, _ = rand.Read(nb[:])
	bs := sha256.Sum256(body)
	bm := md5.Sum(body) //nolint:gosec // part of some documented string-to-sign formats
	ts := strconv.FormatInt(now.Unix(), 10)
	nonce := hex.EncodeToString(nb[:])
	q := ""
	if req.URL.RawQuery != "" {
		q = req.URL.RawQuery
	}
	parts := map[string]string{
		"method": req.Method, "path": req.URL.RequestURI(), "pathOnly": req.URL.EscapedPath(), "query": q, "host": req.URL.Host,
		"timestamp": ts, "timestampMs": strconv.FormatInt(now.UnixMilli(), 10), "isoTime": now.UTC().Format(time.RFC3339),
		"nonce": nonce, "body": string(body), "bodySha256": hex.EncodeToString(bs[:]), "bodyMd5": hex.EncodeToString(bm[:]),
	}
	r := res
	vars := map[string]string{}
	for k, v := range res.vars {
		vars[k] = v
	}
	for k, v := range parts {
		vars[k] = v
	}
	r.vars = vars
	tmpl := strings.ReplaceAll(a.StringToSign, `\n`, "\n")
	if tmpl == "" {
		tmpl = "{{method}}\n{{path}}\n{{timestamp}}\n{{bodySha256}}"
	}
	msg := r.expand(tmpl, missing)
	if len(missing) > 0 {
		return nil, &Error{Code: CodeUnresolved, Message: "the signing settings use unknown values: " + strings.Join(keys(missing), ", ")}
	}
	var h func() hash.Hash
	switch strings.ToLower(a.Algorithm) {
	case "sha1":
		h = sha1.New
	case "sha512":
		h = sha512.New
	default:
		h = sha256.New
	}
	mac := hmac.New(h, []byte(secret))
	mac.Write([]byte(msg))
	sum := mac.Sum(nil)
	sig := hex.EncodeToString(sum)
	if strings.EqualFold(a.Encoding, "base64") {
		sig = base64.StdEncoding.EncodeToString(sum)
	}
	return map[string]string{"sign.signature": sig, "sign.timestamp": ts, "sign.nonce": nonce}, nil
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// looksLikeLoginRedirect reports a session that ended with a redirect to
// a sign-in page, as Django, Rails and many portals do instead of a 401.
func looksLikeLoginRedirect(hr *http.Response) bool {
	if hr.StatusCode < 300 || hr.StatusCode >= 400 {
		return false
	}
	loc := strings.ToLower(hr.Header.Get("Location"))
	return strings.Contains(loc, "login") || strings.Contains(loc, "signin") || strings.Contains(loc, "sign_in") || strings.Contains(loc, "/auth")
}

// preAuth is the auth a call will use, known before building it.
func (p *Proxy) preAuth(ctx context.Context, api store.API, c Call) store.Auth {
	if c.RequestID != "" {
		if r, err := p.store.Request(ctx, c.RequestID); err == nil {
			if isAuthRequest(api, &r) {
				return store.Auth{}
			}
			return effectiveAuth(api, &r)
		}
	}
	if c.Draft != nil {
		return effectiveAuth(api, c.Draft)
	}
	return api.Auth
}

// isAuthRequest reports a request the API's sign-in sends itself.
func isAuthRequest(api store.API, r *store.Request) bool {
	if r == nil || r.ID == "" {
		return false
	}
	for _, id := range api.Auth.AuthRequestIDs() {
		if id == r.ID {
			return true
		}
	}
	return false
}
