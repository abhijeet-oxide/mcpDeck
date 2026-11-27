package deck

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/tool"
)

// BundleFormat names the toolkit file format (EXP-1).
const BundleFormat = "mcpdeck.toolkit/1"

// Bundle is a toolkit file: everything another person needs to run a
// toolkit's tools, without anything of the person who made it (EXP-1).
type Bundle struct {
	Format   string       `json:"format"`
	Exported time.Time    `json:"exported"`
	Toolkit  BundleKit    `json:"toolkit"`
	APIs     []BundleAPI  `json:"apis"`
	Tools    []BundleTool `json:"tools"`
}

// BundleKit is the toolkit's own settings.
type BundleKit struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Rules       store.Rules `json:"rules"`
}

// BundleAPI is one application: requests, auth settings and plain variables.
// Login steps name their requests by name, not ID.
type BundleAPI struct {
	Name         string          `json:"name"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	AllowedHosts []string        `json:"allowedHosts"`
	Auth         store.Auth      `json:"auth"`
	TLS          store.TLS       `json:"tls"`
	Variables    []BundleVar     `json:"variables"`
	Requests     []store.Request `json:"requests"`
}

// BundleVar is a variable; secrets and OS variables travel without a value.
type BundleVar struct {
	Name   string `json:"name"`
	Source string `json:"source"` // literal | secret | os_env
	Value  string `json:"value,omitempty"`
}

// BundleTool is a tool's live version.
type BundleTool struct {
	Name   string           `json:"name"`
	Source string           `json:"source"`
	Tests  []store.ToolTest `json:"tests"`
}

func reqNames(reqs []store.Request) map[string]string {
	m := map[string]string{}
	for _, r := range reqs {
		m[r.ID] = r.Name
	}
	return m
}

func remapAuth(a store.Auth, m map[string]string) store.Auth {
	a.LoginRequestID, a.RefreshRequestID = m[a.LoginRequestID], m[a.RefreshRequestID]
	steps := make([]store.LoginStep, len(a.Steps))
	for i, s := range a.Steps {
		s.RequestID = m[s.RequestID]
		steps[i] = s
	}
	a.Steps = steps
	return a
}

// ExportToolkit makes a toolkit file (EXP-1).
func (d *Deck) ExportToolkit(ctx context.Context, ref string) (*Bundle, error) {
	tk, err := d.toolkitByRef(ctx, ref)
	if err != nil {
		return nil, err
	}
	b := &Bundle{Format: BundleFormat, Exported: d.Clock.Now().UTC(), Toolkit: BundleKit{Name: tk.Name, Description: tk.Description, Rules: tk.Rules},
		APIs: []BundleAPI{}, Tools: []BundleTool{}}
	apis := map[string]bool{}
	tools, _ := d.Store.ToolkitTools(ctx, tk.ID)
	for _, t := range tools {
		v, err := d.Store.LiveVersion(ctx, t.ID)
		if err != nil {
			continue
		}
		tests := v.Tests
		if tests == nil {
			tests = []store.ToolTest{}
		}
		b.Tools = append(b.Tools, BundleTool{Name: v.Name, Source: v.Source, Tests: tests})
		for _, a := range v.APIs {
			apis[a] = true
		}
	}
	for _, id := range tk.BuildScope {
		if a, err := d.Store.API(ctx, id); err == nil {
			apis[a.Name] = true
		}
	}
	all, _ := d.Store.APIs(ctx)
	for _, a := range all {
		if !apis[a.Name] {
			continue
		}
		reqs, _ := d.Store.Requests(ctx, a.ID)
		m := reqNames(reqs)
		ba := BundleAPI{Name: a.Name, Title: a.Title, Description: a.Description, AllowedHosts: d.Proxy.AllowedHosts(ctx, a),
			Auth: remapAuth(a.Auth, m), TLS: a.TLS, Variables: []BundleVar{}, Requests: []store.Request{}}
		vars, _ := d.Store.Variables(ctx, a.ID)
		seen := map[string]bool{}
		for _, v := range vars {
			// Active environment wins; one value per name.
			if (v.EnvID != "" && v.EnvID != a.ActiveEnvID) || seen[v.Name] {
				continue
			}
			seen[v.Name] = true
			bv := BundleVar{Name: v.Name, Source: v.Source}
			if v.Source == "literal" {
				bv.Value = v.Value
			}
			ba.Variables = append(ba.Variables, bv)
		}
		for _, r := range reqs {
			r.ID, r.APIID = "", ""
			r.Example, r.Responses, r.PathParams = nil, nil, nil
			if r.Auth != nil {
				ra := remapAuth(*r.Auth, m)
				r.Auth = &ra
			}
			ba.Requests = append(ba.Requests, r)
		}
		b.APIs = append(b.APIs, ba)
	}
	d.audit(ctx, audit.You, "toolkit.export", tk.Name, fmt.Sprintf("%d tools, %d applications", len(b.Tools), len(b.APIs)))
	return b, nil
}

// ImportResult says what an import created.
type ImportResult struct {
	Toolkit  string         `json:"toolkit"`
	APIs     []string       `json:"apis"`     // created
	Kept     []string       `json:"kept"`     // already here, left as they were
	Tools    []ImportedTool `json:"tools"`    // created
	Secrets  []string       `json:"secrets"`  // "api.name" to fill in
	SignIn   []string       `json:"signIn"`   // applications that sign in through a browser
	FilesFor []string       `json:"filesFor"` // applications whose tools upload files: pick a folder
	Message  string         `json:"message"`
}

// ImportedTool is one tool and whether it waits for approval.
type ImportedTool struct {
	Name   string `json:"name"`
	Status string `json:"status"` // live | pending | exists
	Access string `json:"access"`
}

// ImportToolkit creates what a toolkit file describes, never overwriting:
// secrets empty, scripts untrusted, write tools pending a person (EXP-1).
func (d *Deck) ImportToolkit(ctx context.Context, b Bundle) (*ImportResult, error) {
	if b.Format != BundleFormat {
		return nil, badRequest("this isn't an mcpDeck toolkit file (format %q, want %s)", b.Format, BundleFormat)
	}
	res := &ImportResult{APIs: []string{}, Kept: []string{}, Tools: []ImportedTool{}, Secrets: []string{}, SignIn: []string{}, FilesFor: []string{}}
	now := d.Clock.Now()
	for _, ba := range b.APIs {
		if err := checkAPIName(ba.Name); err != nil {
			return nil, err
		}
		if _, err := d.Store.APIByName(ctx, ba.Name); err == nil {
			res.Kept = append(res.Kept, ba.Name)
			continue
		} else if !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
		a := store.API{ID: d.IDs.New(), Name: ba.Name, Title: ba.Title, Description: ba.Description, Source: "toolkit file",
			AllowedHosts: ba.AllowedHosts, TLS: ba.TLS, CreatedAt: now}
		if a.Title == "" {
			a.Title = a.Name
		}
		env := store.Environment{ID: d.IDs.New(), APIID: a.ID, Name: "Default"}
		a.ActiveEnvID = env.ID
		ids := map[string]string{}
		var reqs []store.Request
		for _, r := range ba.Requests {
			r.ID, r.APIID, r.CreatedAt, r.UpdatedAt = d.IDs.New(), a.ID, now, now
			ids[r.Name] = r.ID
			reqs = append(reqs, r)
		}
		for i := range reqs {
			if reqs[i].Auth != nil {
				ra := remapAuth(*reqs[i].Auth, ids)
				reqs[i].Auth = &ra
			}
		}
		a.Auth = tidyAuth(remapAuth(ba.Auth, ids))
		if err := d.checkAuthNames(a, reqs); err != nil {
			return nil, err
		}
		if err := d.Store.PutAPI(ctx, a); err != nil {
			return nil, err
		}
		_ = d.Store.PutEnvironment(ctx, env)
		for _, r := range reqs {
			if err := d.Store.PutRequest(ctx, r); err != nil {
				return nil, err
			}
		}
		for _, v := range ba.Variables {
			sv := store.Variable{ID: d.IDs.New(), APIID: a.ID, EnvID: env.ID, Name: v.Name, Source: v.Source, UpdatedAt: now}
			switch v.Source {
			case "literal":
				sv.Value = v.Value
			case "secret":
				res.Secrets = append(res.Secrets, a.Name+"."+v.Name)
			case "os_env":
				sv.Value = v.Value
			default:
				continue
			}
			_ = d.Store.PutVariable(ctx, sv)
		}
		if a.Auth.Type == "browser" {
			res.SignIn = append(res.SignIn, a.Name)
		}
		res.APIs = append(res.APIs, a.Name)
		d.audit(ctx, audit.You, "api.create", a.Name, "from a toolkit file")
	}
	name := Slug(b.Toolkit.Name)
	tk, err := d.Store.ToolkitByName(ctx, name)
	if errors.Is(err, store.ErrNotFound) {
		desc := b.Toolkit.Description
		created, cerr := d.CreateToolkit(ctx, ToolkitInput{Name: &name, Description: &desc})
		if cerr != nil {
			return nil, cerr
		}
		tk = *created
	} else if err != nil {
		return nil, err
	}
	res.Toolkit = tk.Name
	all := d.clients(ctx, nil)
	for _, bt := range b.Tools {
		def, err := tool.Load(bt.Source)
		if err != nil {
			return nil, badRequest("tool %s in the file doesn't load: %s", bt.Name, err.Error())
		}
		if _, err := d.Store.ToolByName(ctx, def.Name); err == nil {
			res.Tools = append(res.Tools, ImportedTool{Name: def.Name, Status: "exists"})
			continue
		}
		calls, apis := tool.UsedCalls(bt.Source, all)
		access := d.accessOf(ctx, bt.Source, calls)
		status := "live"
		if access == "write" {
			status = "pending" // a person approves imported write tools (AB-3)
		}
		if strings.Contains(bt.Source, "files.") {
			for _, a := range apis {
				res.FilesFor = appendOnce(res.FilesFor, a)
			}
		}
		t := store.Tool{ID: d.IDs.New(), Name: def.Name, CreatedAt: now}
		if err := d.Store.PutTool(ctx, t); err != nil {
			return nil, err
		}
		_ = d.Store.AddToolToToolkit(ctx, tk.ID, t.ID)
		tests := bt.Tests
		if tests == nil {
			tests = []store.ToolTest{}
		}
		if apis == nil {
			apis = []string{}
		}
		if calls == nil {
			calls = []string{}
		}
		v := store.ToolVersion{ID: d.IDs.New(), ToolID: t.ID, N: 1, Source: bt.Source, Name: def.Name, Description: strings.TrimSpace(def.Description),
			UseWhen: def.UseWhen, DontUseWhen: def.DontUseWhen, InputSchema: def.InputSchema, Access: access, APIs: apis, Calls: calls,
			Tests: tests, TestResults: []store.TestResult{}, Checks: []store.Check{{Rule: "import", Label: "Imported from a toolkit file", Passed: status == "live"}},
			Status: status, CreatedByKind: "you", CreatedByID: "import", CreatedByName: "Toolkit file", ToolkitID: tk.ID, Note: "imported", CreatedAt: now}
		if status == "live" {
			v.DecidedAt = &now
		}
		if err := d.Store.AddVersion(ctx, v); err != nil {
			return nil, err
		}
		res.Tools = append(res.Tools, ImportedTool{Name: def.Name, Status: status, Access: access})
	}
	d.toolsChanged([]string{tk.ID})
	d.audit(ctx, audit.You, "toolkit.import", tk.Name, fmt.Sprintf("%d applications, %d tools", len(res.APIs), len(res.Tools)))
	var next []string
	for _, t := range res.Tools {
		if t.Status == "pending" {
			next = append(next, "approve "+t.Name+" in Review")
		}
	}
	if len(res.Secrets) > 0 {
		next = append(next, "fill in "+strings.Join(res.Secrets, ", "))
	}
	if len(res.FilesFor) > 0 {
		next = append(next, "pick a files folder for "+strings.Join(res.FilesFor, ", "))
	}
	if len(res.SignIn) > 0 {
		next = append(next, "sign in to "+strings.Join(res.SignIn, ", ")+" when the browser opens")
	}
	res.Message = "Imported the " + tk.Name + " toolkit."
	if len(next) > 0 {
		res.Message += " Next: " + strings.Join(next, "; ") + "."
	}
	return res, nil
}

func appendOnce(xs []string, x string) []string {
	for _, y := range xs {
		if y == x {
			return xs
		}
	}
	return append(xs, x)
}

// checkAuthNames checks a login chain's steps name requests in the file.
func (d *Deck) checkAuthNames(a store.API, reqs []store.Request) error {
	known := map[string]bool{"": true}
	for _, r := range reqs {
		known[r.ID] = true
	}
	ids := []string{a.Auth.LoginRequestID, a.Auth.RefreshRequestID}
	for _, s := range a.Auth.Steps {
		ids = append(ids, s.RequestID)
	}
	for _, id := range ids {
		if !known[id] {
			return badRequest("the %s application's sign-in uses a request that isn't in the file", a.Name)
		}
	}
	return nil
}
