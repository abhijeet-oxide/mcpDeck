// Package browser opens the person's installed Edge or Chrome for the two
// things only a person can do in a web app: sign in (AUTH-5) and click
// through a task once so an agent can see its pages and forms (REC-1).
//
// It speaks the Chrome DevTools protocol to a browser started with its own
// profile folder, so the person's everyday browser profile is never touched.
// The browser makes its own requests only while a person drives it; tools and
// agents never use it. Cookies it returns go to internal/proxy and nowhere else.
package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Cookie is one browser cookie.
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Secure   bool    `json:"secure"`
	HTTPOnly bool    `json:"httpOnly"`
	Expires  float64 `json:"expires"` // Unix seconds; -1 for a session cookie
}

// Session is what a sign-in leaves: cookies and the browser's User-Agent,
// which some apps tie the session to.
type Session struct {
	Cookies   []Cookie
	UserAgent string
}

// Step is one page load or request seen while recording.
type Step struct {
	Method      string `json:"method"`
	URL         string `json:"url"`
	Type        string `json:"type"` // Document | XHR | Fetch
	Status      int    `json:"status"`
	Location    string `json:"location,omitempty"`
	ContentType string `json:"contentType,omitempty"` // of the request body
	PostData    string `json:"-"`                     // raw; summarized by the caller, never returned to agents
	MimeType    string `json:"mimeType,omitempty"`    // of the response
	// FormFields are the field names and input types (hidden, text, file…)
	// of the page form that sent this post, when one was seen.
	FormFields map[string]string `json:"formFields,omitempty"`
	// Page is what a page load showed: its forms and tables.
	Page *PageInfo `json:"page,omitempty"`
}

// formsJS describes a loaded page: its URL, its forms (name, method,
// absolute action, enctype, field names and types) and its tables' column
// headers. Names and types only, never values.
const formsJS = `JSON.stringify({u:location.href,f:[...document.forms].map(f=>({n:f.getAttribute('name')||f.id||'',m:(f.getAttribute('method')||'get').toUpperCase(),a:f.action,e:f.enctype,f:[...f.elements].filter(x=>x.name).map(x=>[x.name,(x.type||x.tagName).toLowerCase()])})),t:[...document.querySelectorAll('table')].map(t=>[...(t.querySelector('tr')?.children??[])].map(c=>c.textContent.trim()))})`

type pageForm struct {
	N string     `json:"n"`
	M string     `json:"m"`
	A string     `json:"a"`
	E string     `json:"e"`
	F [][]string `json:"f"`
}

type pageScan struct {
	U string     `json:"u"`
	F []pageForm `json:"f"`
	T [][]string `json:"t"`
}

// PageForm is one form on a recorded page.
type PageForm struct {
	Name    string            `json:"name"`
	Method  string            `json:"method"`
	Action  string            `json:"action"`
	Enctype string            `json:"enctype"`
	Fields  map[string]string `json:"fields"` // name → input type
}

// PageInfo is what a recorded page holds: its forms and table headers.
type PageInfo struct {
	Forms  []PageForm `json:"forms,omitempty"`
	Tables [][]string `json:"tables,omitempty"`
}

// Opener opens a browser for a person.
type Opener interface {
	// SignIn opens start and returns once signedIn(url) is true for the
	// page the person is on.
	SignIn(ctx context.Context, profile, start string, signedIn func(url string) bool) (Session, error)
	// Record opens start and returns the pages and requests seen until the
	// person closes the window or ctx ends (the person clicked Done in
	// mcpDeck); then it returns what it saw with ctx's error.
	Record(ctx context.Context, profile, start string) (Session, []Step, error)
}

// ErrClosed is returned when the person closes the window before signing in.
var ErrClosed = errors.New("the browser window was closed")

// ErrNoBrowser is returned when no Edge or Chrome is installed.
var ErrNoBrowser = errors.New("no Edge or Chrome found; install one, or set MCPDECK_BROWSER to its path")

// Chrome drives an installed Chromium-family browser.
type Chrome struct {
	// Exec is the browser binary; empty finds Edge or Chrome.
	Exec string
	// Dir holds one profile folder per name.
	Dir string
	// Headless runs without a window (tests).
	Headless bool
	// OnPage, when set, is called with each new page so a test can act as
	// the person.
	OnPage func(ctx context.Context, p *Page)
}

// Find returns the browser binary to use.
func Find() (string, error) {
	if p := os.Getenv("MCPDECK_BROWSER"); p != "" {
		return p, nil
	}
	var cands []string
	switch runtime.GOOS {
	case "windows":
		for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LocalAppData"} {
			base := os.Getenv(env)
			if base == "" {
				continue
			}
			cands = append(cands,
				filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
		}
	case "darwin":
		cands = []string{"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Chromium.app/Contents/MacOS/Chromium"}
	default:
		// Chrome first: Edge for Linux is less common and slower to start headless.
		for _, n := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "microsoft-edge-stable"} {
			if p, err := exec.LookPath(n); err == nil {
				cands = append(cands, p)
			}
		}
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", ErrNoBrowser
}

// launch starts the browser with its own profile and connects to it.
func (b *Chrome) launch(ctx context.Context, profile, start string) (*Conn, func(), error) {
	bin := b.Exec
	if bin == "" {
		var err error
		if bin, err = Find(); err != nil {
			return nil, nil, err
		}
	}
	dir := filepath.Join(b.Dir, safeName(profile))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, err
	}
	portFile := filepath.Join(dir, "DevToolsActivePort")
	_ = os.Remove(portFile)
	args := []string{"--user-data-dir=" + dir, "--remote-debugging-port=0", "--no-first-run", "--no-default-browser-check", "--new-window"}
	if b.Headless {
		args = append(args, "--headless=new", "--no-sandbox", "--disable-gpu")
	}
	args = append(args, start)
	cmd := exec.Command(bin, args...)
	errOut := &tailBuffer{max: 2048}
	cmd.Stderr = errOut
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("starting the browser (%s): %w", bin, err)
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	kill := func() {
		select {
		case <-exited:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
		}
	}
	var wsURL string
	deadline := time.Now().Add(30 * time.Second)
	for wsURL == "" {
		if b, err := os.ReadFile(portFile); err == nil {
			lines := strings.Split(strings.TrimSpace(string(b)), "\n")
			if len(lines) == 2 {
				if _, err := strconv.Atoi(strings.TrimSpace(lines[0])); err == nil {
					wsURL = "ws://127.0.0.1:" + strings.TrimSpace(lines[0]) + strings.TrimSpace(lines[1])
				}
			}
		}
		if wsURL != "" {
			break
		}
		select {
		case <-exited:
			return nil, nil, fmt.Errorf("the browser (%s) exited at start; if it was already open with this profile, close that window and try again%s", bin, errOut.say())
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return nil, nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			return nil, nil, fmt.Errorf("the browser (%s) didn't open its debugging port within 30 seconds%s", bin, errOut.say())
		}
	}
	ws, err := dialWS(wsURL, 5*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, nil, err
	}
	c := newConn(ws)
	stop := func() {
		_, _ = c.Call(context.Background(), "Browser.close", nil, "")
		c.Close()
		kill()
	}
	return c, stop, nil
}

// tailBuffer keeps the last bytes a browser wrote to stderr, for errors.
type tailBuffer struct {
	mu  sync.Mutex
	max int
	b   []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, p...)
	if len(t.b) > t.max {
		t.b = t.b[len(t.b)-t.max:]
	}
	return len(p), nil
}

func (t *tailBuffer) say() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s := strings.TrimSpace(string(t.b)); s != "" {
		return ". It said: " + s
	}
	return ""
}

func safeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "default"
	}
	return b.String()
}

type targetInfo struct {
	TargetID string `json:"targetId"`
	Type     string `json:"type"`
	URL      string `json:"url"`
}

// pages follows the browser's page targets: their URLs, and a session on
// each for Network events when record is set.
type pages struct {
	c      *Conn
	record bool
	watch  bool // attach to pages to read their loaded address (sign-in)
	onPage func(ctx context.Context, p *Page)
	ctx    context.Context

	mu       sync.Mutex
	urls     map[string]string // targetId → URL
	sessions map[string]string // sessionId → targetId
	open     atomic.Int32
}

func (ps *pages) attach(t targetInfo) {
	if t.Type != "page" {
		return
	}
	ps.mu.Lock()
	_, known := ps.urls[t.TargetID]
	ps.urls[t.TargetID] = t.URL
	ps.mu.Unlock()
	if known {
		return
	}
	ps.open.Add(1)
	if !ps.record && !ps.watch && ps.onPage == nil {
		return
	}
	go func() {
		res, err := ps.c.Call(ps.ctx, "Target.attachToTarget", map[string]any{"targetId": t.TargetID, "flatten": true}, "")
		if err != nil {
			return
		}
		var a struct {
			SessionID string `json:"sessionId"`
		}
		_ = json.Unmarshal(res, &a)
		if ps.record {
			_, _ = ps.c.Call(ps.ctx, "Network.enable", map[string]any{"maxPostDataSize": 1 << 20}, a.SessionID)
			_, _ = ps.c.Call(ps.ctx, "Page.enable", nil, a.SessionID)
		}
		ps.mu.Lock()
		ps.sessions[a.SessionID] = t.TargetID
		ps.mu.Unlock()
		if ps.onPage != nil {
			go ps.onPage(ps.ctx, &Page{c: ps.c, session: a.SessionID})
		}
	}()
}

func (ps *pages) sessionIDs() []string {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	out := make([]string, 0, len(ps.sessions))
	for k := range ps.sessions {
		out = append(out, k)
	}
	return out
}

func (ps *pages) urlList() []string {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	out := make([]string, 0, len(ps.urls))
	for _, u := range ps.urls {
		out = append(out, u)
	}
	return out
}

func (b *Chrome) start(ctx context.Context, profile, start string, record, watch bool) (*Conn, *pages, func(), error) {
	first := start
	if record {
		// Open blank, turn on network capture, then go: the first page counts too.
		first = "about:blank"
	}
	c, stop, err := b.launch(ctx, profile, first)
	if err != nil {
		return nil, nil, nil, err
	}
	ps := &pages{c: c, record: record, watch: watch, onPage: b.OnPage, ctx: ctx, urls: map[string]string{}, sessions: map[string]string{}}
	c.handle(func(func(event)) func(event) {
		return func(ev event) {
			switch ev.Method {
			case "Target.targetCreated", "Target.targetInfoChanged":
				var p struct {
					TargetInfo targetInfo `json:"targetInfo"`
				}
				_ = json.Unmarshal(ev.Params, &p)
				ps.attach(p.TargetInfo)
			case "Target.targetDestroyed":
				var p struct {
					TargetID string `json:"targetId"`
				}
				_ = json.Unmarshal(ev.Params, &p)
				ps.mu.Lock()
				if _, ok := ps.urls[p.TargetID]; ok {
					delete(ps.urls, p.TargetID)
					ps.open.Add(-1)
				}
				ps.mu.Unlock()
			}
		}
	})
	if _, err := c.Call(ctx, "Target.setDiscoverTargets", map[string]any{"discover": true}, ""); err != nil {
		stop()
		return nil, nil, nil, err
	}
	if record {
		var sid string
		for i := 0; i < 100 && sid == ""; i++ {
			ps.mu.Lock()
			for k := range ps.sessions {
				sid = k
			}
			ps.mu.Unlock()
			if sid == "" {
				time.Sleep(50 * time.Millisecond)
			}
		}
		if sid == "" {
			stop()
			return nil, nil, nil, errors.New("the browser opened no page to record")
		}
		if _, err := c.Call(ctx, "Page.navigate", map[string]any{"url": start}, sid); err != nil {
			stop()
			return nil, nil, nil, err
		}
	}
	return c, ps, stop, nil
}

func (b *Chrome) session(ctx context.Context, c *Conn) (Session, error) {
	var s Session
	res, err := c.Call(ctx, "Storage.getCookies", nil, "")
	if err != nil {
		return s, err
	}
	var cs struct {
		Cookies []Cookie `json:"cookies"`
	}
	if err := json.Unmarshal(res, &cs); err != nil {
		return s, err
	}
	s.Cookies = cs.Cookies
	if v, err := c.Call(ctx, "Browser.getVersion", nil, ""); err == nil {
		var bv struct {
			UserAgent string `json:"userAgent"`
		}
		_ = json.Unmarshal(v, &bv)
		s.UserAgent = strings.Replace(bv.UserAgent, "HeadlessChrome", "Chrome", 1)
	}
	return s, nil
}

// SignIn implements Opener.
func (b *Chrome) SignIn(ctx context.Context, profile, start string, signedIn func(string) bool) (Session, error) {
	c, ps, stop, err := b.start(ctx, profile, start, false, true)
	if err != nil {
		return Session{}, err
	}
	defer stop()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	seen := false
	for {
		select {
		case <-ctx.Done():
			return Session{}, ctx.Err()
		case <-c.done:
			return Session{}, ErrClosed
		case <-tick.C:
		}
		urls := ps.urlList()
		if len(urls) > 0 {
			seen = true
		} else if seen {
			return Session{}, ErrClosed
		}
		// The URL a tab reports can be the one it was asked for, before the
		// app redirects to its sign-in page; trust only a loaded document's
		// own address.
		for _, sid := range ps.sessionIDs() {
			v, err := (&Page{c: c, session: sid}).Eval(ctx, `document.readyState === "complete" ? location.href : ""`)
			if err != nil {
				continue
			}
			var href string
			if json.Unmarshal(v, &href) == nil && href != "" && signedIn(href) {
				return b.session(ctx, c)
			}
		}
	}
}

// Record implements Opener.
func (b *Chrome) Record(ctx context.Context, profile, start string) (Session, []Step, error) {
	c, ps, stop, err := b.start(ctx, profile, start, true, false)
	if err != nil {
		return Session{}, nil, err
	}
	defer stop()
	var mu sync.Mutex
	var steps []Step
	byID := map[string]int{}
	var last Session
	snap := make(chan struct{}, 1)
	forms := map[string]map[string]string{} // METHOD action → field types
	pages := map[string]*PageInfo{}         // URL → what the page held
	c.handle(func(prev func(event)) func(event) {
		return func(ev event) {
			prev(ev)
			switch ev.Method {
			case "Page.loadEventFired", "Page.domContentEventFired":
				sid := ev.SessionID
				go func() {
					v, err := (&Page{c: c, session: sid}).Eval(ctx, formsJS)
					if err != nil {
						return
					}
					var raw string
					var scan pageScan
					if json.Unmarshal(v, &raw) != nil || json.Unmarshal([]byte(raw), &scan) != nil {
						return
					}
					info := &PageInfo{Tables: scan.T}
					mu.Lock()
					for _, f := range scan.F {
						m := map[string]string{}
						for _, x := range f.F {
							if len(x) == 2 {
								m[x[0]] = x[1]
							}
						}
						forms[f.M+" "+f.A] = m
						info.Forms = append(info.Forms, PageForm{Name: f.N, Method: f.M, Action: f.A, Enctype: f.E, Fields: m})
					}
					// A page seen more than once (before and after a post) keeps
					// every form and table it showed.
					if prev := pages[scan.U]; prev != nil {
						have := map[string]bool{}
						for _, f := range prev.Forms {
							have[f.Method+" "+f.Name+" "+f.Action] = true
						}
						for _, f := range info.Forms {
							if !have[f.Method+" "+f.Name+" "+f.Action] {
								prev.Forms = append(prev.Forms, f)
							}
						}
						if len(prev.Tables) == 0 {
							prev.Tables = info.Tables
						}
						info = prev
					}
					pages[scan.U] = info
					mu.Unlock()
				}()
			case "Network.requestWillBeSent":
				var p struct {
					RequestID string `json:"requestId"`
					Type      string `json:"type"`
					Request   struct {
						URL      string            `json:"url"`
						Method   string            `json:"method"`
						Headers  map[string]string `json:"headers"`
						PostData string            `json:"postData"`
					} `json:"request"`
					RedirectResponse *struct {
						Status  int               `json:"status"`
						Headers map[string]string `json:"headers"`
					} `json:"redirectResponse"`
				}
				if json.Unmarshal(ev.Params, &p) != nil {
					return
				}
				mu.Lock()
				defer mu.Unlock()
				key := ev.SessionID + "/" + p.RequestID
				if i, ok := byID[key]; ok && p.RedirectResponse != nil {
					steps[i].Status = p.RedirectResponse.Status
					steps[i].Location = header(p.RedirectResponse.Headers, "Location")
				}
				if p.Type != "Document" && p.Type != "XHR" && p.Type != "Fetch" {
					return
				}
				byID[key] = len(steps)
				steps = append(steps, Step{Method: p.Request.Method, URL: p.Request.URL, Type: p.Type,
					ContentType: header(p.Request.Headers, "Content-Type"), PostData: p.Request.PostData, FormFields: forms[p.Request.Method+" "+p.Request.URL]})
			case "Network.responseReceived":
				var p struct {
					RequestID string `json:"requestId"`
					Response  struct {
						Status   int    `json:"status"`
						MimeType string `json:"mimeType"`
					} `json:"response"`
				}
				if json.Unmarshal(ev.Params, &p) != nil {
					return
				}
				mu.Lock()
				if i, ok := byID[ev.SessionID+"/"+p.RequestID]; ok {
					steps[i].Status, steps[i].MimeType = p.Response.Status, p.Response.MimeType
				}
				mu.Unlock()
				select {
				case snap <- struct{}{}:
				default:
				}
			}
		}
	})
	seen := false
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	finish := func() (Session, []Step, error) {
		mu.Lock()
		defer mu.Unlock()
		out := append([]Step(nil), steps...)
		for i := range out {
			if out[i].Method == "GET" && out[i].Type == "Document" {
				out[i].Page = pages[out[i].URL]
			}
		}
		return last, out, nil
	}
	for {
		select {
		case <-ctx.Done():
			// The person clicked Done: keep the latest cookies before closing.
			fctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if s, err := b.session(fctx, c); err == nil {
				mu.Lock()
				last = s
				mu.Unlock()
			}
			cancel()
			s, st, _ := finish()
			return s, st, ctx.Err()
		case <-c.done:
			return finish()
		case <-snap:
			// Keep the latest cookies: once the window closes they're gone.
			if s, err := b.session(ctx, c); err == nil {
				mu.Lock()
				last = s
				mu.Unlock()
			}
		case <-tick.C:
			if ps.open.Load() > 0 {
				seen = true
			} else if seen {
				return finish()
			}
		}
	}
}

func header(h map[string]string, name string) string {
	for k, v := range h {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return ""
}
