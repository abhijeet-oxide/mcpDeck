package deck

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/browser"
	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/tool"
)

// SetFilesFolder sets the folder an API's tools may upload from (HTTP-3).
// Person only; empty turns it off.
func (d *Deck) SetFilesFolder(ctx context.Context, id, dir string) (*store.API, error) {
	a, err := d.Store.API(ctx, id)
	if err != nil {
		return nil, notFound("API")
	}
	dir = strings.TrimSpace(dir)
	if dir != "" {
		if !filepath.IsAbs(dir) {
			return nil, badRequest("give the folder's full path, e.g. %s", filepath.Join(string(filepath.Separator)+"Users", "you", "Receipts"))
		}
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			return nil, badRequest("%s isn't a folder on this computer", dir)
		}
		dir = filepath.Clean(dir)
	}
	a.FilesDir = dir
	if err := d.Store.PutAPI(ctx, a); err != nil {
		return nil, err
	}
	what := "files folder off"
	if dir != "" {
		what = "files folder " + dir
	}
	d.audit(ctx, audit.You, "api.update", a.Name, what)
	return &a, nil
}

// RequestFilesFolder is an agent asking for the folder an API's tools may
// upload from: the person confirms it in mcpDeck, or nothing changes (HTTP-3).
func (d *Deck) RequestFilesFolder(ctx context.Context, who audit.Actor, id, dir string) (*store.API, error) {
	a, err := d.Store.API(ctx, id)
	if err != nil {
		return nil, notFound("API")
	}
	dir = strings.TrimSpace(dir)
	if !filepath.IsAbs(dir) {
		return nil, badRequest("give the folder's full path")
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil, badRequest("%s isn't a folder on this computer", dir)
	}
	title := "Let " + a.Name + " tools upload files from " + filepath.Clean(dir)
	fields := map[string]any{"application": a.Name, "folder": filepath.Clean(dir), "what": "Tools may list this folder and upload a file from it when you run them. Agents see file names only."}
	if err := d.AskConfirm(ctx, Caller{Actor: who}, title, fields); err != nil {
		return nil, errf(http.StatusForbidden, "not_confirmed", "%s", err.Error())
	}
	return d.SetFilesFolder(ctx, id, dir)
}

// ListFiles lists an API's files folder.
func (d *Deck) ListFiles(ctx context.Context, id string) ([]proxy.FileInfo, error) {
	return d.Proxy.ListFiles(ctx, id)
}

// SignIn opens the browser for the person to sign in to an API now (AUTH-5).
func (d *Deck) SignIn(ctx context.Context, id string) (map[string]any, error) {
	a, err := d.Store.API(ctx, id)
	if err != nil {
		return nil, notFound("API")
	}
	ctx, cancel := context.WithTimeout(ctx, d.Proxy.SigninTimeout)
	defer cancel()
	if err := d.Proxy.SignIn(ctx, id); err != nil {
		return nil, err
	}
	d.audit(ctx, audit.You, "api.signin", a.Name, "browser sign-in")
	return map[string]any{"signedIn": true, "api": a.Name}, nil
}

// RecordInput starts a recording (REC-1).
type RecordInput struct {
	API string `json:"api"` // name; created if new
	URL string `json:"url"`
}

// RecordedForm is a form post seen while recording.
type RecordedForm struct {
	ContentType string            `json:"contentType"`
	Fields      map[string]string `json:"fields"` // name → kind: (hidden), text, number, date, file …
}

// RecordedRequest is one request the recording added to the API.
type RecordedRequest struct {
	Name   string        `json:"name"`
	Method string        `json:"method"`
	Path   string        `json:"path"`
	Status int           `json:"status"`
	Goes   string        `json:"goesTo,omitempty"` // redirect target path
	Form   *RecordedForm `json:"form,omitempty"`
	Page   *RecordedPage `json:"page,omitempty"` // what a page load showed
}

// RecordedPage is a page's forms and tables, so an agent knows what to
// read with html.forms and html.tables. Names and types, never values.
type RecordedPage struct {
	Forms  []string   `json:"forms,omitempty"`  // "claimForm POST /expenses/claims: _state(hidden), amount(text)…"
	Tables [][]string `json:"tables,omitempty"` // column headers
}

// pageForms reads a page's forms as tools will (html.forms), with actions
// made absolute against the page's address.
func pageForms(pageURL, page string) []tool.Form {
	base, _ := url.Parse(pageURL)
	forms := tool.ParseForms(page)
	for i := range forms {
		if base != nil {
			if u, err := base.Parse(forms[i].Action); err == nil {
				forms[i].Action = u.String()
			}
		}
	}
	return forms
}

// recordPage summarizes a page from its HTML: its forms and tables.
func recordPage(pageURL, page string) *RecordedPage {
	if page == "" {
		return nil
	}
	out := &RecordedPage{Tables: tool.TableHeaders(page)}
	if len(out.Tables) == 0 {
		out.Tables = nil
	}
	for _, f := range pageForms(pageURL, page) {
		path := f.Action
		if u, err := url.Parse(f.Action); err == nil {
			path, _ = templatePath(u.Path)
		}
		names := []string{}
		for _, fl := range f.Fields {
			switch fl.Type {
			case "submit", "button", "reset", "image":
				continue
			}
			names = append(names, fl.Name+"("+fl.Type+")")
		}
		sort.Strings(names)
		name := f.Name
		if name == "" {
			name = f.ID
		}
		if name == "" {
			name = "(unnamed)"
		}
		enc := ""
		if strings.Contains(f.Enctype, "multipart") {
			enc = " multipart"
		}
		out.Forms = append(out.Forms, fmt.Sprintf("%s %s %s%s: %s", name, f.Method, path, enc, strings.Join(names, ", ")))
	}
	if len(out.Forms) == 0 && len(out.Tables) == 0 {
		return nil
	}
	return out
}

// mergePage adds the forms and tables of another visit to the same page.
func mergePage(a, b *RecordedPage) *RecordedPage {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	have := map[string]bool{}
	for _, f := range a.Forms {
		have[f] = true
	}
	for _, f := range b.Forms {
		if !have[f] {
			a.Forms = append(a.Forms, f)
		}
	}
	if len(a.Tables) == 0 {
		a.Tables = b.Tables
	}
	return a
}

// RecordResult is what an agent gets back: short, no cookies or values.
type RecordResult struct {
	API      string            `json:"api"`
	SignedIn bool              `json:"signedIn"`
	Requests []RecordedRequest `json:"requests"`
	Next     string            `json:"next"`
}

var idSeg = regexp.MustCompile(`^(\d+|[0-9a-fA-F-]{16,})$`)

// Record opens the browser at a URL for the person to click through a task
// once, then adds one request per distinct page or post seen on the API's
// hosts and returns their shapes (REC-1).
func (d *Deck) Record(ctx context.Context, in RecordInput) (*RecordResult, error) {
	b := d.Proxy.Browser()
	if b == nil {
		return nil, badRequest("recording needs mcpDeck running on your computer with Edge or Chrome installed")
	}
	u, err := url.Parse(strings.TrimSpace(in.URL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, badRequest("give the page to start from as an http or https URL")
	}
	name := strings.TrimSpace(in.API)
	a, err := d.Store.APIByName(ctx, name)
	if errors.Is(err, store.ErrNotFound) {
		created, cerr := d.CreateAPI(ctx, CreateAPIInput{Name: name, Title: u.Host, BaseURL: u.Scheme + "://" + u.Host,
			Auth: store.Auth{Type: "browser", StartURL: u.String()}})
		if cerr != nil {
			return nil, cerr
		}
		a = *created
		a.AllowedHosts = []string{strings.ToLower(u.Host)}
		_ = d.Store.PutAPI(ctx, a)
	} else if err != nil {
		return nil, err
	}
	rctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	d.mu.Lock()
	if d.recording == nil {
		d.recording = map[string]context.CancelFunc{}
	}
	if _, busy := d.recording[a.Name]; busy {
		d.mu.Unlock()
		return nil, conflict("%s is already being recorded; finish it in the browser, or click Done in mcpDeck", a.Name)
	}
	d.recording[a.Name] = cancel
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.recording, a.Name)
		d.mu.Unlock()
	}()
	sess, steps, err := b.Record(rctx, a.ID, u.String())
	if err != nil && len(steps) == 0 {
		return nil, badRequest("recording didn't finish: %v", err)
	}
	res := &RecordResult{API: a.Name, Requests: []RecordedRequest{}}
	if len(sess.Cookies) > 0 {
		res.SignedIn = d.Proxy.UseBrowserSession(ctx, a, sess) > 0
	}
	allowed := d.Proxy.AllowedHosts(ctx, a)
	existing, _ := d.Store.Requests(ctx, a.ID)
	byKey := map[string]bool{}
	for _, r := range existing {
		byKey[r.Method+" "+r.URL] = true
	}
	// Which page form sent each post: its field types (hidden, file…).
	fieldsBy := map[string]map[string]string{}
	for _, st := range steps {
		for _, f := range pageForms(st.URL, st.HTML) {
			m := map[string]string{}
			for _, fl := range f.Fields {
				m[fl.Name] = fl.Type
			}
			fieldsBy[f.Method+" "+f.Action] = m
		}
	}
	seen := map[string]int{}
	for _, st := range steps {
		su, err := url.Parse(st.URL)
		if err != nil || !hostIn(su.Host, allowed) || signInPath(su.Path) {
			continue
		}
		path, params := templatePath(su.Path)
		key := st.Method + " " + path
		if i, ok := seen[key]; ok {
			if f := recordForm(st, fieldsBy[st.Method+" "+st.URL]); f != nil && res.Requests[i].Form == nil {
				res.Requests[i].Form = f
			}
			res.Requests[i].Page = mergePage(res.Requests[i].Page, recordPage(st.URL, st.HTML))
			// A page first seen redirecting to sign-in, then loaded.
			if r := &res.Requests[i]; r.Status >= 300 && r.Status < 400 && st.Status == 200 {
				r.Status, r.Goes = 200, ""
			}
			continue
		}
		rr := RecordedRequest{Name: requestName(st.Method, path, params), Method: st.Method, Path: path, Status: st.Status, Form: recordForm(st, fieldsBy[st.Method+" "+st.URL]), Page: recordPage(st.URL, st.HTML)}
		// Where a post sends the person; a redirect to sign-in isn't the agent's concern.
		if lu, err := su.Parse(st.Location); err == nil && st.Location != "" && hostIn(lu.Host, allowed) && !signInPath(lu.Path) {
			rr.Goes, _ = templatePath(lu.Path)
		}
		seen[key] = len(res.Requests)
		res.Requests = append(res.Requests, rr)
		if byKey[st.Method+" {{baseUrl}}"+path] {
			continue
		}
		req := store.Request{Name: rr.Name, Title: rr.Name, Method: st.Method, URL: "{{baseUrl}}" + path, Access: proxy.ClassifyMethod(st.Method)}
		if st.Method != "GET" && st.Method != "HEAD" {
			req.BodyType = "form"
		}
		if _, err := d.SaveRequest(ctx, a.ID, req); err != nil {
			return nil, err
		}
	}
	res.Next = "Each request is a method on " + a.Name + " (" + a.Name + ".<name>), already signed in. Write tools that: fetch a page (it returns HTML text); read it with " +
		"html.tables(page) (rows keyed by the headers above) or html.forms(page).find(f => f.name === \"<form>\"); post { ...form.values, ...input } to the form's request " +
		"(hidden fields come from the page each time; never hard-code them); use <request>.raw(...) for a post and read headers.location to find the page it went to. " +
		"For a file field pass files.attach(fileName) (a file in the folder set with set_files_folder). A tool that submits sets confirm: true and awaits confirm({ title, fields }) before its first post. " +
		"Read tools need a test; save with save_tool, then create_toolkit if needed and export_toolkit to share."
	d.audit(ctx, audit.You, "api.record", a.Name, fmt.Sprintf("%d requests", len(res.Requests)))
	return res, nil
}

// Recordings lists the APIs being recorded now.
func (d *Deck) Recordings() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := []string{}
	for k := range d.recording {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// StopRecording ends a recording, as closing its window does (REC-1).
func (d *Deck) StopRecording(api string) error {
	d.mu.Lock()
	stop, ok := d.recording[api]
	d.mu.Unlock()
	if !ok {
		return notFound("recording of " + api)
	}
	stop()
	return nil
}

func hostIn(h string, allowed []string) bool {
	for _, a := range allowed {
		if strings.EqualFold(a, h) {
			return true
		}
	}
	return false
}

func signInPath(p string) bool {
	p = strings.ToLower(p)
	for _, w := range []string{"login", "signin", "sign-in", "/sso", "saml", "oauth", "logout"} {
		if strings.Contains(p, w) {
			return true
		}
	}
	return false
}

// templatePath turns ID-looking segments into {id} params.
func templatePath(p string) (string, []string) {
	segs := strings.Split(p, "/")
	var params []string
	for i, s := range segs {
		if idSeg.MatchString(s) {
			n := "id"
			if len(params) > 0 {
				n = fmt.Sprintf("id%d", len(params)+1)
			}
			segs[i] = "{" + n + "}"
			params = append(params, n)
		}
	}
	return strings.Join(segs, "/"), params
}

// requestName is a camelCase name like getExpensesClaims or postClaimsReceipt.
func requestName(method, path string, params []string) string {
	var words []string
	for _, s := range strings.Split(path, "/") {
		if s == "" || strings.HasPrefix(s, "{") {
			continue
		}
		words = append(words, strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })...)
	}
	if len(words) > 4 {
		words = words[len(words)-4:]
	}
	b := strings.ToLower(method)
	for _, w := range words {
		b += strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	if len(params) > 0 {
		b += "ById"
	}
	return b
}

// recordForm summarizes a post's fields by kind, never by value.
func recordForm(st browser.Step, formFields map[string]string) *RecordedForm {
	if st.Method == "GET" || st.Method == "HEAD" {
		return nil
	}
	ct, params, _ := mime.ParseMediaType(st.ContentType)
	f := &RecordedForm{ContentType: ct, Fields: map[string]string{}}
	values := url.Values{}
	if ct == "application/x-www-form-urlencoded" {
		values, _ = url.ParseQuery(st.PostData)
	}
	if len(formFields) > 0 {
		// The page's own form says which fields are hidden or files.
		for name, typ := range formFields {
			switch typ {
			case "submit", "button", "reset", "image":
				continue
			case "hidden":
				f.Fields[name] = "(hidden: take it from the page)"
			case "file":
				f.Fields[name] = "file"
			case "password":
				f.Fields[name] = "(secret, left out)"
			default:
				if v, ok := values[name]; ok {
					f.Fields[name] = kindOf(name, v[0])
				} else {
					f.Fields[name] = typ
				}
			}
		}
		for name, v := range values {
			if _, ok := f.Fields[name]; !ok {
				f.Fields[name] = kindOf(name, v[0]) + " (set by the page's script)"
			}
		}
		return f
	}
	switch ct {
	case "application/x-www-form-urlencoded":
		q, _ := url.ParseQuery(st.PostData)
		for k, v := range q {
			f.Fields[k] = kindOf(k, v[0])
		}
	case "multipart/form-data":
		// Part headers only: Content-Disposition names, and a filename for files.
		re := regexp.MustCompile(`(?i)content-disposition: form-data; name="([^"]+)"(; filename="([^"]*)")?`)
		for _, m := range re.FindAllStringSubmatch(st.PostData, -1) {
			if m[2] != "" {
				f.Fields[m[1]] = "file"
			} else {
				f.Fields[m[1]] = "(hidden or text)"
			}
		}
		_ = params
	default:
		f.Fields["(body)"] = ct
	}
	return f
}

var (
	numRe  = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)
)

func kindOf(name, v string) string {
	switch {
	case proxy.IsSensitiveKey(name):
		return "(secret, left out)"
	case strings.HasPrefix(name, "_") || strings.Contains(strings.ToLower(name), "state") || strings.Contains(strings.ToLower(name), "csrf") || len(v) >= 16 && !strings.Contains(v, " "):
		return "(hidden: take it from the page)"
	case v == "":
		return "empty"
	case dateRe.MatchString(v):
		return "date"
	case numRe.MatchString(v):
		return "number"
	case len(v) <= 12 && !strings.Contains(v, " "):
		return "word: " + v
	}
	return "text"
}

// Confirmation is one write waiting for the person (CONF-1).
type Confirmation struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Fields  any       `json:"fields"`
	Tool    string    `json:"tool,omitempty"`
	Agent   string    `json:"agent"`
	Toolkit string    `json:"toolkit,omitempty"`
	At      time.Time `json:"at"`
	answer  chan bool
}

type confirmations struct {
	mu    sync.Mutex
	items map[string]*Confirmation
}

func (d *Deck) confirms() *confirmations {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.conf == nil {
		d.conf = &confirmations{items: map[string]*Confirmation{}}
	}
	return d.conf
}

// AskConfirm waits for the person to confirm a write (CONF-1).
func (d *Deck) AskConfirm(ctx context.Context, c Caller, title string, fields any) error {
	cs := d.confirms()
	item := &Confirmation{ID: d.IDs.New(), Title: title, Fields: fields, Agent: c.Actor.Name, At: d.Clock.Now(), answer: make(chan bool, 1)}
	if c.Toolkit != nil {
		item.Toolkit = c.Toolkit.Name
	}
	cs.mu.Lock()
	cs.items[item.ID] = item
	cs.mu.Unlock()
	defer func() {
		cs.mu.Lock()
		delete(cs.items, item.ID)
		cs.mu.Unlock()
	}()
	if d.OpenConfirm != nil {
		d.OpenConfirm(item.ID)
	}
	t := time.NewTimer(ConfirmWait)
	defer t.Stop()
	select {
	case ok := <-item.answer:
		d.audit(ctx, audit.You, "tool.confirm", title, map[bool]string{true: "confirmed", false: "declined"}[ok]+" for "+item.Agent)
		if !ok {
			return errors.New("the user declined " + title + "; nothing was sent. Ask them what to change")
		}
		return nil
	case <-t.C:
		return errors.New("the user didn't confirm " + title + " within 5 minutes; nothing was sent. Ask them to confirm in mcpDeck and call again")
	case <-ctx.Done():
		return errors.New("stopped before the user confirmed " + title + "; nothing was sent")
	}
}

// ConfirmWait is how long a confirmation waits (CONF-1).
var ConfirmWait = 5 * time.Minute

// Confirmations lists writes waiting for the person.
func (d *Deck) Confirmations() []Confirmation {
	cs := d.confirms()
	cs.mu.Lock()
	defer cs.mu.Unlock()
	out := []Confirmation{}
	for _, c := range cs.items {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

// DecideConfirmation confirms or declines a waiting write. Person only.
func (d *Deck) DecideConfirmation(id string, ok bool) error {
	cs := d.confirms()
	cs.mu.Lock()
	item := cs.items[id]
	if item != nil {
		delete(cs.items, id)
	}
	cs.mu.Unlock()
	if item == nil {
		return notFound("confirmation (it may have timed out)")
	}
	item.answer <- ok
	return nil
}
