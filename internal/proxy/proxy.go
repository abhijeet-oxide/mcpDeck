// Package proxy is the only outbound HTTP path in mcpDeck (APP-2). Every
// request from the runner, tools, scripts and build tools comes through Do,
// which checks the allow-list and access, resolves variables and secrets,
// adds auth and cookies, sends, masks, and records the call.
//
// Secrets are decrypted only here and never leave this package (CRED-7).
package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/tokens"
	"github.com/abhijeet-oxide/mcpdeck/internal/vault"
)

// Error codes callers branch on. Messages say what to do next (MCP-4).
const (
	CodeHostNotAllowed = "host_not_allowed"
	CodeWriteBlocked   = "write_blocked"
	CodeUnresolved     = "unresolved_variables"
	CodeLoginFailed    = "login_failed"
	CodeUpstream       = "upstream_error"
	CodeNotFound       = "not_found"
	CodeNoExample      = "no_example"
	CodeBadTLS         = "bad_tls"
)

// Error is an actionable proxy error.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// IsCode reports whether err is a proxy error with code.
func IsCode(err error, code string) bool {
	var pe *Error
	return errors.As(err, &pe) && pe.Code == code
}

// Actor is who sends the request.
type Actor struct {
	Kind string `json:"kind"` // you | agent
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Args are values for a request. Flat values are spread to path, query or
// body as the request's shape allows (TOOL-2).
type Args struct {
	Path    map[string]any
	Query   map[string]any
	Headers map[string]string
	Body    any
	HasBody bool
	Flat    map[string]any
}

// Call describes one outbound request.
type Call struct {
	APIID       string
	RequestID   string         // a saved request
	Draft       *store.Request // an unsaved request from the runner
	Method      string         // with Path, for call("api", "GET /path")
	Path        string
	Args        Args
	Access      string // "" for you (no limit), "read" or "read_write" for agents
	ToolkitName string
	EnvID       string // "" means the API's active environment
	Actor       Actor
	SessionID   string
	ParentID    string
	ToolkitID   string
	Examples    bool // answer from the saved example, no network
}

// Response is a masked upstream response.
type Response struct {
	CallID      string              `json:"callId"`
	Status      int                 `json:"status"`
	StatusText  string              `json:"statusText"`
	Headers     map[string][]string `json:"headers"`
	Body        []byte              `json:"-"`
	DurationMS  int64               `json:"durationMs"`
	Size        int                 `json:"size"`
	Tokens      int                 `json:"tokens"`
	Method      string              `json:"method"`
	URL         string              `json:"url"`
	FromExample bool                `json:"fromExample"`
	Binary      bool                `json:"binary,omitempty"` // Body holds the facts of a binary file, not the file
	StartedAt   time.Time           `json:"startedAt"`
}

// JSON parses the body, or returns the text when it isn't JSON.
func (r *Response) JSON() any {
	var v any
	if err := json.Unmarshal(r.Body, &v); err != nil {
		return string(r.Body)
	}
	return v
}

// Proxy sends requests.
type Proxy struct {
	store *store.Store
	vault *vault.Vault
	clock clock.Clock
	ids   clock.IDs
	tr    http.RoundTripper

	mu        sync.Mutex
	jars      map[string]*cookiejar.Jar
	sessions  map[string]*session     // apiID → sign-in values (AUTH-2)
	sessLocks map[string]*sync.Mutex  // one sign-in at a time per API
	tls       map[string]tlsTransport // apiID → transport for its TLS settings
}

type tlsTransport struct {
	key    store.TLS
	keyPEM string
	rt     http.RoundTripper
	err    error
}

// New returns a proxy. tr may be nil for the default transport.
func New(s *store.Store, v *vault.Vault, c clock.Clock, ids clock.IDs, tr http.RoundTripper) *Proxy {
	if tr == nil {
		tr = &http.Transport{Proxy: http.ProxyFromEnvironment, MaxIdleConnsPerHost: 16, IdleConnTimeout: 60 * time.Second}
	}
	return &Proxy{store: s, vault: v, clock: c, ids: ids, tr: tr,
		jars: map[string]*cookiejar.Jar{}, sessions: map[string]*session{}, sessLocks: map[string]*sync.Mutex{}, tls: map[string]tlsTransport{}}
}

func (p *Proxy) client(api store.API, timeout time.Duration, res resolved) (*http.Client, error) {
	keyPEM := ""
	if api.TLS.ClientKey != "" {
		missing := map[string]bool{}
		keyPEM = res.expand(api.TLS.ClientKey, missing)
		if len(missing) > 0 {
			return nil, unresolvedErr(api, missing)
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	jar, ok := p.jars[api.ID]
	if !ok {
		jar, _ = cookiejar.New(nil)
		p.jars[api.ID] = jar
	}
	rt, err := p.transportLocked(api, keyPEM)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: rt, Jar: jar, Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}

// transportLocked is the transport for an API's TLS settings (API-9): the
// shared one by default, or a clone with its own TLS config.
func (p *Proxy) transportLocked(api store.API, keyPEM string) (http.RoundTripper, error) {
	if !api.TLS.Insecure && strings.TrimSpace(api.TLS.CACerts) == "" && strings.TrimSpace(api.TLS.ClientCert) == "" {
		return p.tr, nil
	}
	if t, ok := p.tls[api.ID]; ok && t.key == api.TLS && t.keyPEM == keyPEM {
		return t.rt, t.err
	}
	base, ok := p.tr.(*http.Transport)
	if !ok {
		// A test transport: TLS settings don't apply.
		return p.tr, nil
	}
	cfg, err := TLSConfig(api.TLS)
	if err == nil && strings.TrimSpace(api.TLS.ClientCert) != "" {
		cert, cerr := tls.X509KeyPair([]byte(api.TLS.ClientCert), []byte(keyPEM))
		if cerr != nil {
			err = &Error{Code: CodeBadTLS, Message: "the client certificate or key in the " + api.Name + " API's TLS settings can't be used (" + cerr.Error() + "); paste the PEM certificate and keep the key in a secret variable"}
		} else {
			cfg.Certificates = []tls.Certificate{cert}
		}
	}
	t := tlsTransport{key: api.TLS, keyPEM: keyPEM, err: err}
	if err == nil {
		c := base.Clone()
		c.TLSClientConfig = cfg
		t.rt = c
	}
	p.tls[api.ID] = t
	return t.rt, t.err
}

// TLSConfig builds the client TLS config for an API's settings.
func TLSConfig(s store.TLS) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if s.Insecure {
		cfg.InsecureSkipVerify = true //nolint:gosec // the person turned verification off for this API (API-9)
	}
	if pem := strings.TrimSpace(s.CACerts); pem != "" {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(pem)) {
			return nil, &Error{Code: CodeBadTLS, Message: "the trusted certificates in this API's TLS settings aren't valid PEM; paste certificates that start with -----BEGIN CERTIFICATE-----"}
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

// ClearCookies empties an API's cookie jar.
func (p *Proxy) ClearCookies(apiID string) {
	p.mu.Lock()
	delete(p.jars, apiID)
	delete(p.sessions, apiID)
	delete(p.tls, apiID)
	p.mu.Unlock()
}

// Fetch GETs a public document (an OpenAPI spec or docs page) for import.
// insecure skips certificate checks, for servers with self-signed
// certificates (API-9).
func (p *Proxy) Fetch(ctx context.Context, rawURL string, insecure bool) ([]byte, string, error) {
	return p.FetchTLS(ctx, rawURL, store.TLS{Insecure: insecure})
}

// FetchTLS is Fetch with an API's TLS settings, so a spec behind a company
// CA imports with that CA trusted (API-9).
func (p *Proxy) FetchTLS(ctx context.Context, rawURL string, ts store.TLS) ([]byte, string, error) {
	insecure := ts.Insecure
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, "", &Error{Code: CodeUpstream, Message: "use an http or https URL"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/json, application/yaml, text/html;q=0.9, */*;q=0.8")
	rt := p.tr
	if insecure || strings.TrimSpace(ts.CACerts) != "" {
		p.mu.Lock()
		rt, err = p.transportLocked(store.API{ID: "\x00fetch", TLS: store.TLS{Insecure: insecure, CACerts: ts.CACerts}}, "")
		p.mu.Unlock()
		if err != nil {
			return nil, "", err
		}
	}
	res, err := (&http.Client{Transport: rt, Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		if isCertErr(err) {
			return nil, "", &Error{Code: CodeBadTLS, Message: "couldn't fetch " + u.Host + ": its certificate isn't trusted (" + err.Error() + "). If you trust this server, turn on “Skip certificate check” and try again."}
		}
		return nil, "", &Error{Code: CodeUpstream, Message: "couldn't fetch " + u.Host + ": " + err.Error()}
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 20<<20))
	if err != nil {
		return nil, "", err
	}
	if res.StatusCode >= 400 {
		return nil, "", &Error{Code: CodeUpstream, Message: fmt.Sprintf("%s returned %d", u.Host, res.StatusCode)}
	}
	return b, res.Header.Get("Content-Type"), nil
}

var varRe = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.\-]+)\s*\}\}`)

type resolved struct {
	vars    map[string]string
	secrets redactor
}

// resolveVars loads API then environment variables (env wins), decrypting secrets.
func (p *Proxy) resolveVars(ctx context.Context, api store.API, envID string) (resolved, error) {
	vars, err := p.store.Variables(ctx, api.ID)
	if err != nil {
		return resolved{}, err
	}
	if envID == "" {
		envID = api.ActiveEnvID
	}
	r := resolved{vars: map[string]string{}}
	sort.SliceStable(vars, func(i, j int) bool { return vars[i].EnvID == "" && vars[j].EnvID != "" })
	for _, v := range vars {
		if v.EnvID != "" && v.EnvID != envID {
			continue
		}
		val := v.Value
		switch v.Source {
		case "secret":
			if v.Ciphertext == "" {
				continue
			}
			val, err = p.vault.Unseal(v.Ciphertext)
			if err != nil {
				return r, fmt.Errorf("credential %s for %s: %w", v.Name, api.Name, err)
			}
			r.secrets = append(r.secrets, val)
		case "os_env":
			val = os.Getenv(v.Value)
		}
		r.vars[v.Name] = val
	}
	// Longest first so a secret containing another is fully replaced.
	sort.Slice(r.secrets, func(i, j int) bool { return len(r.secrets[i]) > len(r.secrets[j]) })
	return r, nil
}

func (r resolved) expand(s string, missing map[string]bool) string {
	return varRe.ReplaceAllStringFunc(s, func(m string) string {
		name := varRe.FindStringSubmatch(m)[1]
		if v, ok := r.vars[name]; ok {
			return v
		}
		missing[name] = true
		return m
	})
}

// AllowedHosts returns the API's allow-list, defaulting to the hosts of its
// environments' base URLs and literal request URLs (APP-2).
func (p *Proxy) AllowedHosts(ctx context.Context, api store.API) []string {
	if len(api.AllowedHosts) > 0 {
		return api.AllowedHosts
	}
	seen := map[string]bool{}
	var out []string
	add := func(raw string) {
		if u, err := url.Parse(strings.TrimSpace(raw)); err == nil && u.Host != "" && !strings.Contains(u.Host, "{{") {
			h := strings.ToLower(u.Host)
			if !seen[h] {
				seen[h] = true
				out = append(out, h)
			}
		}
	}
	vars, _ := p.store.Variables(ctx, api.ID)
	for _, v := range vars {
		if v.Source == "literal" && (strings.HasPrefix(v.Value, "http://") || strings.HasPrefix(v.Value, "https://")) {
			add(v.Value)
		}
	}
	reqs, _ := p.store.Requests(ctx, api.ID)
	for _, r := range reqs {
		add(r.URL)
	}
	sort.Strings(out)
	return out
}

func hostAllowed(host string, allowed []string) bool {
	host = strings.ToLower(host)
	hostOnly := host
	if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host[i:], "]") {
		hostOnly = host[:i]
	}
	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == host || a == hostOnly {
			return true
		}
	}
	return false
}

// ClassifyMethod is the default access for a method (API-1).
func ClassifyMethod(m string) string {
	switch strings.ToUpper(m) {
	case "GET", "HEAD", "OPTIONS":
		return "read"
	}
	return "write"
}

var pathParamRe = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}|/:([A-Za-z_][A-Za-z0-9_]*)`)

// PathParams lists the {param} and :param names in a URL template.
func PathParams(u string) []string {
	u = varRe.ReplaceAllString(u, "")
	var out []string
	for _, m := range pathParamRe.FindAllStringSubmatch(u, -1) {
		if m[1] != "" {
			out = append(out, m[1])
		} else {
			out = append(out, m[2])
		}
	}
	return out
}

func fmtArg(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprint(t)
	default:
		if b, err := json.Marshal(t); err == nil && (strings.HasPrefix(string(b), "{") || strings.HasPrefix(string(b), "[")) {
			return string(b)
		}
		return fmt.Sprint(t)
	}
}

// built is a request ready to send, before auth.
type built struct {
	method  string
	url     string
	headers http.Header
	body    []byte
	access  string
	name    string
	reqID   string
}

func (p *Proxy) build(ctx context.Context, api store.API, c Call, res resolved) (built, *store.Request, error) {
	var r store.Request
	switch {
	case c.Draft != nil:
		r = *c.Draft
		if r.Access == "" {
			r.Access = ClassifyMethod(r.Method)
		}
	case c.RequestID != "":
		var err error
		r, err = p.store.Request(ctx, c.RequestID)
		if err != nil {
			return built{}, nil, &Error{Code: CodeNotFound, Message: "request not found; list requests with search_apis"}
		}
	default:
		pathOnly, rawQ, _ := strings.Cut(c.Path, "?")
		base := "{{baseUrl}}"
		if u, err := url.Parse(pathOnly); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			// An absolute URL from a Location or next link; the host
			// allowlist still applies.
			base, pathOnly = u.Scheme+"://"+u.Host, u.EscapedPath()
		}
		if reqs, err := p.store.Requests(ctx, api.ID); err == nil && base == "{{baseUrl}}" {
			if m, params := MatchRequest(reqs, c.Method, pathOnly); m != nil {
				// call("api", "METHOD /path") on a saved request uses its
				// headers, body type and access (TOOL-9).
				// The call spells out its own path, query and body; saved
				// example values don't apply.
				r = *m
				r.Query, r.Body = nil, ""
				c.Args = withPathArgs(c.Args, params)
			}
		}
		if r.ID == "" {
			r = store.Request{APIID: api.ID, Method: strings.ToUpper(c.Method), URL: base + pathOnly,
				Access: ClassifyRaw(c.Method, pathOnly, c.Args), Name: strings.ToUpper(c.Method) + " " + pathOnly, BodyType: "json"}
		}
		if rawQ != "" {
			if q, err := url.ParseQuery(rawQ); err == nil {
				keys := make([]string, 0, len(q))
				for k := range q {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					r.Query = append(r.Query, store.KV{Key: k, Value: q[k][0]})
				}
			}
		}
	}
	a := c.Args
	path := map[string]any{}
	query := map[string]any{}
	var body any
	hasBody := false
	for k, v := range a.Path {
		path[k] = v
	}
	for k, v := range a.Query {
		query[k] = v
	}
	if a.HasBody {
		body, hasBody = a.Body, true
	}
	// Spread flat args.
	if len(a.Flat) > 0 {
		params := map[string]bool{}
		for _, n := range PathParams(r.URL) {
			params[n] = true
		}
		qkeys := map[string]bool{}
		for _, q := range r.Query {
			qkeys[q.Key] = true
		}
		bodyArgs := map[string]any{}
		for k, v := range a.Flat {
			switch {
			case params[k]:
				path[k] = v
			case qkeys[k] || ClassifyMethod(r.Method) == "read" || strings.ToUpper(r.Method) == "DELETE":
				query[k] = v
			default:
				bodyArgs[k] = v
			}
		}
		if len(bodyArgs) > 0 && !hasBody && r.BodyType != "json" && r.BodyType != "none" && r.BodyType != "" {
			// Text, XML and form templates take arguments in {{name}} slots.
			tmpl := r.Body
			for k, v := range bodyArgs {
				val := fmtArg(v)
				if r.BodyType == "xml" {
					val = xmlEscape(val)
				}
				if strings.Contains(tmpl, "{{"+k+"}}") {
					tmpl = strings.ReplaceAll(tmpl, "{{"+k+"}}", val)
					delete(bodyArgs, k)
				}
			}
			if r.BodyType == "form" && len(bodyArgs) > 0 {
				form, _ := url.ParseQuery(tmpl)
				for k, v := range bodyArgs {
					form.Set(k, fmtArg(v))
				}
				tmpl = form.Encode()
				bodyArgs = nil
			}
			r.Body = tmpl
		}
		if len(bodyArgs) > 0 && !hasBody {
			// Merge over the saved JSON body template.
			base := map[string]any{}
			if r.BodyType == "json" && strings.TrimSpace(r.Body) != "" {
				_ = json.Unmarshal([]byte(res.expand(r.Body, map[string]bool{})), &base)
			}
			for k, v := range bodyArgs {
				base[k] = v
			}
			body, hasBody = base, true
		}
	}

	missing := map[string]bool{}
	u := res.expand(r.URL, missing)
	u = pathParamRe.ReplaceAllStringFunc(u, func(m string) string {
		sub := pathParamRe.FindStringSubmatch(m)
		name, prefix := sub[1], ""
		if name == "" {
			name, prefix = sub[2], "/"
		}
		if v, ok := path[name]; ok {
			return prefix + url.PathEscape(fmtArg(v))
		}
		missing["path:"+name] = true
		return m
	})
	pu, err := url.Parse(u)
	if err != nil || pu.Host == "" {
		if len(missing) > 0 {
			return built{}, &r, unresolvedErr(api, missing)
		}
		return built{}, &r, &Error{Code: CodeUpstream, Message: fmt.Sprintf("the URL %q isn't absolute; set baseUrl in the %s API's environment", r.URL, api.Name)}
	}
	q := pu.Query()
	for _, kv := range r.Query {
		if kv.Disabled {
			continue
		}
		if _, over := query[kv.Key]; over {
			continue
		}
		val := res.expand(kv.Value, missing)
		if val == "" {
			continue
		}
		q.Set(kv.Key, val)
	}
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if query[k] == nil {
			continue
		}
		q.Set(k, fmtArg(query[k]))
	}
	pu.RawQuery = q.Encode()

	h := http.Header{}
	for _, kv := range r.Headers {
		if !kv.Disabled && kv.Key != "" {
			h.Set(kv.Key, res.expand(kv.Value, missing))
		}
	}
	for k, v := range a.Headers {
		h.Set(k, v)
	}
	var b []byte
	switch {
	case hasBody:
		if s, ok := body.(string); ok {
			b = []byte(s)
		} else {
			b, _ = json.Marshal(body)
			if h.Get("Content-Type") == "" {
				h.Set("Content-Type", "application/json")
			}
		}
	case r.Body != "" && r.BodyType != "none":
		b = []byte(res.expand(r.Body, missing))
		if h.Get("Content-Type") == "" {
			switch r.BodyType {
			case "json":
				h.Set("Content-Type", "application/json")
			case "xml":
				h.Set("Content-Type", "text/xml; charset=utf-8")
			case "form":
				h.Set("Content-Type", "application/x-www-form-urlencoded")
			default:
				h.Set("Content-Type", "text/plain; charset=utf-8")
			}
		}
	}
	if len(missing) > 0 {
		return built{}, &r, unresolvedErr(api, missing)
	}
	access := r.EffectiveAccess()
	if c.Draft == nil && c.RequestID == "" && r.ID == "" {
		access = ClassifyRaw(r.Method, r.URL, c.Args)
	}
	// A GraphQL mutation writes, whatever request carries it.
	if strings.ToUpper(r.Method) == "POST" && IsGraphQLPath(pu.Path) && IsGraphQLMutation(string(b)) {
		access = "write"
	}
	name := api.Name + "." + r.Name
	return built{method: strings.ToUpper(r.Method), url: pu.String(), headers: h, body: b, access: access, name: name, reqID: r.ID}, &r, nil
}

func unresolvedErr(api store.API, missing map[string]bool) error {
	var vars, params []string
	for k := range missing {
		if strings.HasPrefix(k, "path:") {
			params = append(params, strings.TrimPrefix(k, "path:"))
		} else {
			vars = append(vars, "{{"+k+"}}")
		}
	}
	sort.Strings(vars)
	sort.Strings(params)
	var parts []string
	if len(params) > 0 {
		parts = append(parts, "missing path values: "+strings.Join(params, ", ")+"; pass them as arguments")
	}
	if len(vars) > 0 {
		parts = append(parts, "unresolved variables "+strings.Join(vars, ", ")+"; add them on the "+api.Name+" API's Environments tab in mcpDeck")
	}
	return &Error{Code: CodeUnresolved, Message: strings.Join(parts, "; ")}
}

// Do sends one request through every check, and records it.
func (p *Proxy) Do(ctx context.Context, c Call) (*Response, error) {
	start := p.clock.Now()
	api, err := p.store.API(ctx, c.APIID)
	if err != nil {
		return nil, &Error{Code: CodeNotFound, Message: "API not found"}
	}
	res, err := p.resolveVars(ctx, api, c.EnvID)
	if err != nil {
		return nil, err
	}
	rec := store.Call{ID: p.ids.New(), SessionID: c.SessionID, ParentID: c.ParentID, ActorKind: c.Actor.Kind,
		ActorID: c.Actor.ID, ActorName: c.Actor.Name, ToolkitID: c.ToolkitID, Kind: "upstream", StartedAt: start, APIID: api.ID}
	fail := func(status string, e error) (*Response, error) {
		rec.Status = status
		rec.Error = res.secrets.String(e.Error())
		rec.DurationMS = p.clock.Now().Sub(start).Milliseconds()
		_ = p.store.InsertCall(context.WithoutCancel(ctx), rec)
		return nil, e
	}
	// Sign in first, so {{login.*}} works in any template (AUTH-2).
	if pre := p.preAuth(ctx, api, c); usesSession(pre) {
		if err := p.ensureSession(ctx, api, pre, &res, false); err != nil {
			rec.Name = api.Name
			return fail("error", err)
		}
	}
	b, r, err := p.build(ctx, api, c, res)
	if r != nil {
		rec.Name = api.Name + "." + r.Name
		rec.Method = strings.ToUpper(r.Method)
		rec.RequestID = r.ID
	}
	if err != nil {
		return fail("error", err)
	}
	// Tools and agents never send the sign-in requests themselves (CRED-8).
	if c.Actor.Kind == "agent" && r != nil && isAuthRequest(api, r) {
		return fail("blocked", &Error{Code: CodeWriteBlocked, Message: b.name + " is how mcpDeck signs in to " + api.Name + "; tools can't call it. Call the API's other requests and mcpDeck signs in for them."})
	}
	rec.URL = res.secrets.String(b.url)
	rec.Input = res.secrets.String(string(MaskJSON(b.body)))

	// Access (ACC-2): a read grant never sends a write.
	if c.Access == "read" && b.access == "write" {
		why := b.method + " writes"
		if r != nil && r.AccessFlag {
			why = "it isn't confirmed as read (check access)"
		}
		tk := c.ToolkitName
		if tk == "" {
			tk = "this"
		}
		return fail("blocked", &Error{Code: CodeWriteBlocked, Message: fmt.Sprintf(
			"write blocked: %s (%s). This agent has read access to the %s toolkit. Use a read tool, or ask the user to give this agent read+write in mcpDeck.", b.name, why, tk)})
	}
	if c.Examples {
		if r == nil || len(r.Example) == 0 {
			return fail("error", &Error{Code: CodeNoExample, Message: fmt.Sprintf("%s has no saved example; send it once in mcpDeck and save the response, or test against the real API", b.name)})
		}
		out := &Response{CallID: rec.ID, Status: 200, StatusText: "200 OK (example)", Body: r.Example, Size: len(r.Example),
			Method: b.method, URL: rec.URL, FromExample: true, StartedAt: start, Headers: map[string][]string{"Content-Type": {"application/json"}}}
		out.Tokens = tokens.Count(string(r.Example))
		rec.Status, rec.HTTPStatus, rec.Name = "ok", 200, rec.Name+" (example)"
		rec.Output, rec.OutputBytes, rec.Tokens = truncate(string(r.Example)), out.Size, out.Tokens
		_ = p.store.InsertCall(context.WithoutCancel(ctx), rec)
		return out, nil
	}
	pu, _ := url.Parse(b.url)
	allowed := p.AllowedHosts(ctx, api)
	if !hostAllowed(pu.Host, allowed) {
		return fail("blocked", &Error{Code: CodeHostNotAllowed, Message: fmt.Sprintf(
			"host %s isn't in the allowed hosts of the %s API (%s). A person can add it in mcpDeck under the API's Settings.", pu.Host, api.Name, strings.Join(allowed, ", "))})
	}

	auth := effectiveAuth(api, r)
	cl, err := p.client(api, 30*time.Second, res)
	if err != nil {
		return fail("error", err)
	}
	send := func() (*http.Response, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, b.method, b.url, bytes.NewReader(b.body))
		if err != nil {
			return nil, nil, err
		}
		req.Header = b.headers.Clone()
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "mcpdeck")
		}
		if err := p.applyAuth(ctx, api, auth, req, res, b.body); err != nil {
			return nil, nil, err
		}
		hr, err := cl.Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer hr.Body.Close()
		body, err := io.ReadAll(io.LimitReader(hr.Body, 10<<20))
		return hr, body, err
	}
	hr, body, err := send()
	// A session the API no longer accepts: sign in again once (AUTH-2).
	if err == nil && usesSession(auth) && (hr.StatusCode == http.StatusUnauthorized || looksLikeLoginRedirect(hr)) {
		p.dropSession(api.ID)
		if serr := p.ensureSession(ctx, api, auth, &res, false); serr != nil {
			return fail("error", serr)
		}
		hr, body, err = send()
	}
	// Rate limits and brief outages: wait as told and retry reads (RESP-1).
	for attempt := 0; err == nil && attempt < 3 && (hr.StatusCode == http.StatusTooManyRequests || hr.StatusCode == http.StatusServiceUnavailable) && b.access == "read"; attempt++ {
		wait, ok := retryAfter(hr.Header, attempt, p.clock.Now())
		if !ok {
			break
		}
		select {
		case <-ctx.Done():
			return fail("error", ctx.Err())
		case <-time.After(wait):
		}
		hr, body, err = send()
	}
	if err != nil {
		var pe *Error
		if errors.As(err, &pe) {
			return fail("error", pe)
		}
		if isCertErr(err) {
			return fail("error", &Error{Code: CodeBadTLS, Message: res.secrets.String(fmt.Sprintf("%s: the server's certificate isn't trusted (%v). In mcpDeck, open the %s application's Settings → TLS and add its CA certificate, or turn off verification for this application.", b.name, err, api.Name))})
		}
		return fail("error", &Error{Code: CodeUpstream, Message: res.secrets.String(fmt.Sprintf("%s couldn't be reached: %v. Check the %s API's base URL and that the service is running.", b.name, err, api.Name))})
	}
	binary := isBinary(hr.Header.Get("Content-Type"), body)
	if binary {
		body = binaryFacts(hr.Header, body)
	}
	body = res.secrets.Bytes(body)
	out := &Response{CallID: rec.ID, Status: hr.StatusCode, StatusText: hr.Status, Headers: map[string][]string{},
		Body: body, Size: len(body), Method: b.method, URL: rec.URL, StartedAt: start, Binary: binary}
	for k, v := range hr.Header {
		if strings.EqualFold(k, "Set-Cookie") {
			out.Headers[k] = []string{"[masked]"}
			continue
		}
		vs := make([]string, len(v))
		for i := range v {
			vs[i] = res.secrets.String(v[i])
		}
		out.Headers[k] = vs
	}
	out.DurationMS = p.clock.Now().Sub(start).Milliseconds()
	out.Tokens = tokens.Count(string(body))
	rec.Status = "ok"
	if hr.StatusCode >= 400 {
		rec.Status = "error"
		rec.Error = hr.Status
	}
	rec.HTTPStatus = hr.StatusCode
	rec.DurationMS = out.DurationMS
	rec.Output = truncate(string(MaskJSON(body)))
	rec.OutputBytes = out.Size
	rec.Tokens = out.Tokens
	_ = p.store.InsertCall(context.WithoutCancel(ctx), rec)
	return out, nil
}

// effectiveAuth is a request's own auth, or its API's.
func effectiveAuth(api store.API, r *store.Request) store.Auth {
	if r != nil && r.Auth != nil && r.Auth.Type != "" && r.Auth.Type != "inherit" {
		return *r.Auth
	}
	return api.Auth
}

func truncate(s string) string {
	const max = 64 << 10
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// applyAuth adds credentials to a request. A session (login, oauth2) is
// already in res as {{login.*}}; its Apply headers are added here.
func (p *Proxy) applyAuth(ctx context.Context, api store.API, a store.Auth, req *http.Request, res resolved, body []byte) error {
	missing := map[string]bool{}
	apply := func(r resolved) {
		for _, kv := range a.Apply {
			if kv.Key != "" && !kv.Disabled {
				req.Header.Set(kv.Key, r.expand(kv.Value, missing))
			}
		}
	}
	switch a.Type {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+res.expand(a.Token, missing))
	case "apikey":
		k, v := res.expand(a.Key, missing), res.expand(a.Value, missing)
		if a.In == "query" {
			q := req.URL.Query()
			q.Set(k, v)
			req.URL.RawQuery = q.Encode()
		} else {
			req.Header.Set(k, v)
		}
	case "basic":
		req.SetBasicAuth(res.expand(a.Username, missing), res.expand(a.Password, missing))
	case "login":
		apply(res)
		if len(a.Steps) == 0 && len(a.Apply) == 0 {
			apply2 := normalizeLogin(a)
			for _, kv := range apply2.Apply {
				req.Header.Set(kv.Key, res.expand(kv.Value, missing))
			}
		}
	case "oauth2":
		if len(a.Apply) == 0 {
			req.Header.Set("Authorization", "Bearer "+res.expand("{{login.access_token}}", missing))
		}
		apply(res)
	case "hmac":
		vals, err := sign(a, req, body, res, p.clock.Now())
		if err != nil {
			return err
		}
		r := res
		r.vars = make(map[string]string, len(res.vars)+3)
		for k, v := range res.vars {
			r.vars[k] = v
		}
		for k, v := range vals {
			r.vars[k] = v
		}
		apply(r)
	}
	if len(missing) > 0 {
		return unresolvedErr(api, missing)
	}
	return nil
}

// isCertErr reports whether err is a TLS certificate verification failure.
func isCertErr(err error) bool {
	var ua x509.UnknownAuthorityError
	var hn x509.HostnameError
	var ci x509.CertificateInvalidError
	var tv *tls.CertificateVerificationError
	return errors.As(err, &ua) || errors.As(err, &hn) || errors.As(err, &ci) || errors.As(err, &tv)
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
