package deck

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/importer"
	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/tokens"
)

// APISummary is a row on the APIs page.
type APISummary struct {
	store.API
	Requests     int      `json:"requests"`
	Environments []string `json:"environments"`
	Tools        []string `json:"tools"`
	NeedsSecret  bool     `json:"needsSecret"`
	Flagged      int      `json:"flagged"`
}

// VariableView is a variable as the UI may see it: never a secret value (CRED-5).
type VariableView struct {
	ID        string `json:"id"`
	EnvID     string `json:"envId"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	Value     string `json:"value"`
	IsSet     bool   `json:"isSet"`
	UpdatedAt string `json:"updatedAt"`
}

// APIDetail is the API workspace.
type APIDetail struct {
	store.API
	Requests     []store.Request     `json:"requests"`
	Environments []store.Environment `json:"environments"`
	Variables    []VariableView      `json:"variables"`
	Hosts        []string            `json:"hosts"`
	Tools        []string            `json:"tools"`
}

var apiNameRe = regexp.MustCompile(`^[a-z][a-zA-Z0-9]{1,31}$`)

func checkAPIName(n string) error {
	if !apiNameRe.MatchString(n) {
		return badRequest("API name %q must start with a lowercase letter and use letters and digits only (it's how tools call it: %s.getAccount())", n, n)
	}
	switch n {
	case "call", "vars", "console", "input", "pm", "tool", "ApiError", "output":
		return badRequest("%q is reserved in tool code; choose another name", n)
	}
	return nil
}

// toolsUsingAPI lists live or pending tools whose code calls the API.
func (d *Deck) toolsUsingAPI(ctx context.Context, apiName string) []string {
	tools, _ := d.Store.Tools(ctx)
	var out []string
	for _, t := range tools {
		vs, _ := d.Store.Versions(ctx, t.ID)
		if len(vs) == 0 {
			continue
		}
		for _, a := range vs[0].APIs {
			if a == apiName {
				out = append(out, t.Name)
				break
			}
		}
	}
	return out
}

func (d *Deck) ListAPIs(ctx context.Context) ([]APISummary, error) {
	apis, err := d.Store.APIs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]APISummary, 0, len(apis))
	for _, a := range apis {
		reqs, _ := d.Store.Requests(ctx, a.ID)
		envs, _ := d.Store.Environments(ctx, a.ID)
		s := APISummary{API: a, Requests: len(reqs), Tools: d.toolsUsingAPI(ctx, a.Name), Environments: []string{}}
		for _, e := range envs {
			s.Environments = append(s.Environments, e.Name)
		}
		for _, r := range reqs {
			if r.AccessFlag {
				s.Flagged++
			}
		}
		vars, _ := d.Store.Variables(ctx, a.ID)
		for _, v := range vars {
			if v.Source == "secret" && v.Ciphertext == "" {
				s.NeedsSecret = true
			}
		}
		out = append(out, s)
	}
	return out, nil
}

func (d *Deck) GetAPI(ctx context.Context, id string) (*APIDetail, error) {
	a, err := d.Store.API(ctx, id)
	if err != nil {
		return nil, notFound("API")
	}
	reqs, _ := d.Store.Requests(ctx, id)
	for i := range reqs {
		normalize(&reqs[i])
	}
	sort.SliceStable(reqs, func(i, j int) bool { return reqs[i].Folder < reqs[j].Folder })
	envs, _ := d.Store.Environments(ctx, id)
	vars, _ := d.Store.Variables(ctx, id)
	out := &APIDetail{API: a, Requests: reqs, Environments: envs, Variables: []VariableView{},
		Hosts: d.Proxy.AllowedHosts(ctx, a), Tools: d.toolsUsingAPI(ctx, a.Name)}
	for _, v := range vars {
		vv := VariableView{ID: v.ID, EnvID: v.EnvID, Name: v.Name, Source: v.Source, UpdatedAt: v.UpdatedAt.Format("2006-01-02T15:04:05Z")}
		if v.Source == "secret" {
			vv.IsSet = v.Ciphertext != ""
		} else {
			vv.Value, vv.IsSet = v.Value, true
		}
		out.Variables = append(out.Variables, vv)
	}
	return out, nil
}

// CreateAPIInput starts an empty API.
type CreateAPIInput struct {
	Name    string     `json:"name"`
	Title   string     `json:"title"`
	BaseURL string     `json:"baseUrl"`
	Auth    store.Auth `json:"auth"`
	Secret  string     `json:"secret"` // value for the auth's secret variable, if any
}

func (d *Deck) CreateAPI(ctx context.Context, in CreateAPIInput) (*store.API, error) {
	if err := checkAPIName(in.Name); err != nil {
		return nil, err
	}
	if _, err := d.Store.APIByName(ctx, in.Name); err == nil {
		return nil, conflict("an API named %s already exists; import into it instead", in.Name)
	}
	now := d.Clock.Now()
	a := store.API{ID: d.IDs.New(), Name: in.Name, Title: in.Title, Source: "manual", Auth: in.Auth, CreatedAt: now}
	if a.Title == "" {
		a.Title = in.Name
	}
	if a.Auth.Type == "" {
		a.Auth.Type = "none"
	}
	env := store.Environment{ID: d.IDs.New(), APIID: a.ID, Name: "Default"}
	a.ActiveEnvID = env.ID
	if err := d.Store.PutAPI(ctx, a); err != nil {
		return nil, err
	}
	_ = d.Store.PutEnvironment(ctx, env)
	if in.BaseURL != "" {
		_ = d.Store.PutVariable(ctx, store.Variable{ID: d.IDs.New(), APIID: a.ID, EnvID: env.ID, Name: "baseUrl", Source: "literal", Value: strings.TrimSuffix(in.BaseURL, "/"), UpdatedAt: now})
	}
	for _, name := range secretVarsOf(a.Auth) {
		v := store.Variable{ID: d.IDs.New(), APIID: a.ID, EnvID: env.ID, Name: name, Source: "secret", UpdatedAt: now}
		if in.Secret != "" {
			v.Ciphertext, _ = d.Vault.Seal(in.Secret)
		}
		_ = d.Store.PutVariable(ctx, v)
		break
	}
	d.audit(ctx, audit.You, "api.create", a.Name, "")
	return &a, nil
}

var tmplVar = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.\-]+)\s*\}\}`)

func secretVarsOf(a store.Auth) []string {
	var out []string
	for _, s := range []string{a.Token, a.Value, a.Password} {
		for _, m := range tmplVar.FindAllStringSubmatch(s, -1) {
			out = append(out, m[1])
		}
	}
	return out
}

// UpdateAPIInput changes an API. Nil fields stay.
type UpdateAPIInput struct {
	Title          *string     `json:"title"`
	Description    *string     `json:"description"`
	AllowedHosts   *[]string   `json:"allowedHosts"`
	Auth           *store.Auth `json:"auth"`
	ActiveEnvID    *string     `json:"activeEnvId"`
	ScriptsTrusted *bool       `json:"scriptsTrusted"`
}

func (d *Deck) UpdateAPI(ctx context.Context, id string, in UpdateAPIInput) (*store.API, error) {
	a, err := d.Store.API(ctx, id)
	if err != nil {
		return nil, notFound("API")
	}
	var changed []string
	if in.Title != nil {
		a.Title = *in.Title
		changed = append(changed, "title")
	}
	if in.Description != nil {
		a.Description = *in.Description
	}
	if in.AllowedHosts != nil {
		hosts := []string{}
		for _, h := range *in.AllowedHosts {
			if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
				hosts = append(hosts, h)
			}
		}
		a.AllowedHosts = hosts
		changed = append(changed, "allowed hosts")
	}
	if in.Auth != nil {
		a.Auth = *in.Auth
		changed = append(changed, "auth")
		// Make sure the auth's secret variables exist in the active environment.
		vars, _ := d.Store.Variables(ctx, a.ID)
		for _, name := range secretVarsOf(a.Auth) {
			found := false
			for _, v := range vars {
				if v.Name == name {
					found = true
				}
			}
			if !found {
				_ = d.Store.PutVariable(ctx, store.Variable{ID: d.IDs.New(), APIID: a.ID, EnvID: a.ActiveEnvID, Name: name, Source: "secret", UpdatedAt: d.Clock.Now()})
			}
		}
		d.Proxy.ClearCookies(a.ID)
	}
	if in.ActiveEnvID != nil {
		if _, err := d.Store.Environment(ctx, *in.ActiveEnvID); err != nil {
			return nil, notFound("environment")
		}
		a.ActiveEnvID = *in.ActiveEnvID
	}
	if in.ScriptsTrusted != nil {
		a.ScriptsTrusted = *in.ScriptsTrusted
	}
	if err := d.Store.PutAPI(ctx, a); err != nil {
		return nil, err
	}
	if len(changed) > 0 {
		d.audit(ctx, audit.You, "api.update", a.Name, strings.Join(changed, ", "))
	}
	return &a, nil
}

func (d *Deck) DeleteAPI(ctx context.Context, id string) error {
	a, err := d.Store.API(ctx, id)
	if err != nil {
		return notFound("API")
	}
	if tools := d.toolsUsingAPI(ctx, a.Name); len(tools) > 0 {
		return conflict("%s is used by %s; change or remove those tools first", a.Name, strings.Join(tools, ", "))
	}
	reqs, _ := d.Store.Requests(ctx, id)
	for _, r := range reqs {
		_ = d.Store.Delete(ctx, "request", r.ID)
	}
	vars, _ := d.Store.Variables(ctx, id)
	for _, v := range vars {
		_ = d.Store.Delete(ctx, "variable", v.ID)
	}
	envs, _ := d.Store.Environments(ctx, id)
	for _, e := range envs {
		_ = d.Store.Delete(ctx, "environment", e.ID)
	}
	_ = d.Store.Delete(ctx, "api", id)
	d.audit(ctx, audit.You, "api.delete", a.Name, "")
	return nil
}

// ImportSource is what to import.
type ImportSource struct {
	URL         string `json:"url"`
	Text        string `json:"text"`
	Environment string `json:"environment"` // Postman environment JSON
}

func (d *Deck) parseSource(ctx context.Context, src ImportSource) (*importer.Result, error) {
	data := []byte(src.Text)
	if strings.TrimSpace(src.URL) != "" {
		b, ct, err := d.Proxy.Fetch(ctx, strings.TrimSpace(src.URL))
		if err != nil {
			return nil, badRequest("%s", err.Error())
		}
		if importer.IsHTML(ct, b) {
			spec, script := importer.FindSpecURL(src.URL, b)
			if spec == "" && script != "" {
				if js, _, err := d.Proxy.Fetch(ctx, script); err == nil {
					spec, _ = importer.FindSpecURL(script, js)
				}
			}
			if spec == "" {
				return nil, badRequest(importer.NoSpecFound)
			}
			b, _, err = d.Proxy.Fetch(ctx, spec)
			if err != nil {
				return nil, badRequest("found the spec at %s but couldn't fetch it: %s", spec, err.Error())
			}
		}
		data = b
	}
	var res *importer.Result
	var err error
	if strings.TrimSpace(src.Environment) != "" {
		res, err = importer.Postman(data, []byte(src.Environment))
	} else {
		res, err = importer.Detect(data)
	}
	if err != nil {
		return nil, badRequest("%s", err.Error())
	}
	return res, nil
}

// PreviewImport parses a source without saving anything (UI-14).
func (d *Deck) PreviewImport(ctx context.Context, src ImportSource) (*importer.Result, error) {
	return d.parseSource(ctx, src)
}

// ImportInput applies an import into a new or existing API.
type ImportInput struct {
	ImportSource
	APIID string `json:"apiId"`
	Name  string `json:"name"` // for a new API
	Skip  []int  `json:"skip"` // indexes of requests not to import
}

// ImportReport says what changed (CLI-1: adds and updates, never deletes).
type ImportReport struct {
	APIID       string   `json:"apiId"`
	APIName     string   `json:"apiName"`
	Added       []string `json:"added"`
	Updated     []string `json:"updated"`
	Credentials []string `json:"credentials"`
	Warnings    []string `json:"warnings"`
}

func (d *Deck) Import(ctx context.Context, in ImportInput) (*ImportReport, error) {
	res, err := d.parseSource(ctx, in.ImportSource)
	if err != nil {
		return nil, err
	}
	now := d.Clock.Now()
	var a store.API
	if in.APIID != "" {
		a, err = d.Store.API(ctx, in.APIID)
		if err != nil {
			return nil, notFound("API")
		}
	} else {
		name := in.Name
		if name == "" {
			name = res.SuggestName
		}
		if existing, err := d.Store.APIByName(ctx, name); err == nil {
			a = existing
		} else {
			if err := checkAPIName(name); err != nil {
				return nil, err
			}
			a = store.API{ID: d.IDs.New(), Name: name, Title: res.Title, Description: res.Description, Source: res.Kind, Auth: res.Auth, CreatedAt: now}
			if a.Auth.Type == "" {
				a.Auth.Type = "none"
			}
			env := store.Environment{ID: d.IDs.New(), APIID: a.ID, Name: "Default"}
			if len(res.Environments) > 0 {
				env.Name = res.Environments[0]
			}
			a.ActiveEnvID = env.ID
			if err := d.Store.PutAPI(ctx, a); err != nil {
				return nil, err
			}
			_ = d.Store.PutEnvironment(ctx, env)
			d.audit(ctx, audit.You, "api.create", a.Name, "import from "+res.Kind)
		}
	}
	rep := &ImportReport{APIID: a.ID, APIName: a.Name, Added: []string{}, Updated: []string{}, Credentials: []string{}, Warnings: res.Warnings}
	existing, _ := d.Store.Requests(ctx, a.ID)
	byName := map[string]store.Request{}
	for _, r := range existing {
		byName[r.Name] = r
	}
	skip := map[int]bool{}
	for _, i := range in.Skip {
		skip[i] = true
	}
	for i, r := range res.Requests {
		if skip[i] {
			continue
		}
		r.APIID = a.ID
		r.UpdatedAt = now
		if old, ok := byName[r.Name]; ok {
			r.ID, r.CreatedAt = old.ID, old.CreatedAt
			if len(r.Example) == 0 {
				r.Example = old.Example
			}
			rep.Updated = append(rep.Updated, r.Name)
		} else {
			r.ID, r.CreatedAt = d.IDs.New(), now
			rep.Added = append(rep.Added, r.Name)
		}
		if len(r.Example) > 0 {
			r.Example = proxy.MaskJSON(r.Example)
		}
		normalize(&r)
		if err := d.Store.PutRequest(ctx, r); err != nil {
			return nil, err
		}
	}
	// Variables: environments by name, then values.
	envs, _ := d.Store.Environments(ctx, a.ID)
	envByName := map[string]string{}
	for _, e := range envs {
		envByName[e.Name] = e.ID
	}
	vars, _ := d.Store.Variables(ctx, a.ID)
	for _, v := range res.Vars {
		envID := a.ActiveEnvID
		if v.Env != "" {
			id, ok := envByName[v.Env]
			if !ok {
				e := store.Environment{ID: d.IDs.New(), APIID: a.ID, Name: v.Env}
				_ = d.Store.PutEnvironment(ctx, e)
				envByName[v.Env], id = e.ID, e.ID
			}
			envID = id
		}
		var cur *store.Variable
		for i := range vars {
			if vars[i].Name == v.Name && (vars[i].EnvID == envID || vars[i].EnvID == "") {
				cur = &vars[i]
				break
			}
		}
		if cur == nil {
			cur = &store.Variable{ID: d.IDs.New(), APIID: a.ID, EnvID: envID, Name: v.Name}
		}
		cur.UpdatedAt = now
		if v.Secret {
			cur.Source, cur.Value = "secret", ""
			if v.Value != "" {
				cur.Ciphertext, _ = d.Vault.Seal(v.Value)
				rep.Credentials = append(rep.Credentials, v.Name)
			}
		} else {
			if v.Value == "" && cur.Value != "" {
				continue
			}
			cur.Source, cur.Value = "literal", v.Value
		}
		_ = d.Store.PutVariable(ctx, *cur)
	}
	d.audit(ctx, audit.You, "api.import", a.Name, fmt.Sprintf("%d added, %d updated", len(rep.Added), len(rep.Updated)))
	if len(rep.Credentials) > 0 {
		d.audit(ctx, audit.You, "credential.set", a.Name, "from import: "+strings.Join(rep.Credentials, ", "))
	}
	return rep, nil
}

func normalize(r *store.Request) {
	if r.Headers == nil {
		r.Headers = []store.KV{}
	}
	if r.Query == nil {
		r.Query = []store.KV{}
	}
}

// SaveRequest creates or updates a request.
func (d *Deck) SaveRequest(ctx context.Context, apiID string, r store.Request) (*store.Request, error) {
	a, err := d.Store.API(ctx, apiID)
	if err != nil {
		return nil, notFound("API")
	}
	r.Name = importer.MethodName(r.Name)
	r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
	if r.Method == "" {
		r.Method = "GET"
	}
	if r.URL == "" {
		return nil, badRequest("a request needs a URL, like {{baseUrl}}/accounts/{id}")
	}
	reqs, _ := d.Store.Requests(ctx, apiID)
	now := d.Clock.Now()
	isNew := r.ID == ""
	for _, o := range reqs {
		if o.Name == r.Name && o.ID != r.ID {
			return nil, conflict("%s already has a request named %s", a.Name, r.Name)
		}
	}
	if isNew {
		r.ID, r.CreatedAt = d.IDs.New(), now
		if r.Access == "" {
			importer.Classify(&r)
		}
	} else {
		old, err := d.Store.Request(ctx, r.ID)
		if err != nil || old.APIID != apiID {
			return nil, notFound("request")
		}
		r.CreatedAt = old.CreatedAt
		if r.Example == nil {
			r.Example = old.Example
		}
		if r.ResponseSchema == nil {
			r.ResponseSchema = old.ResponseSchema
		}
		if r.Access == "" {
			r.Access, r.AccessReason, r.AccessFlag = old.Access, old.AccessReason, old.AccessFlag
		}
	}
	if r.Access != "read" && r.Access != "write" {
		return nil, badRequest("access must be read or write")
	}
	if r.Headers == nil {
		r.Headers = []store.KV{}
	}
	if r.Query == nil {
		r.Query = []store.KV{}
	}
	if r.BodyType == "" {
		r.BodyType = "none"
	}
	r.APIID, r.UpdatedAt = apiID, now
	if err := d.Store.PutRequest(ctx, r); err != nil {
		return nil, err
	}
	act := "request.update"
	if isNew {
		act = "request.create"
	}
	d.audit(ctx, audit.You, act, a.Name+"."+r.Name, r.Access)
	if !isNew {
		d.notifyToolkitsUsingAPI(ctx, a.Name)
	}
	return &r, nil
}

// SetAccess confirms or overrides a request's access (API-1).
func (d *Deck) SetAccess(ctx context.Context, id, access, reason string) (*store.Request, error) {
	r, err := d.Store.Request(ctx, id)
	if err != nil {
		return nil, notFound("request")
	}
	if access != "read" && access != "write" {
		return nil, badRequest("access must be read or write")
	}
	r.Access, r.AccessFlag = access, false
	r.AccessReason = reason
	if reason == "" {
		r.AccessReason = "set by you"
	}
	r.UpdatedAt = d.Clock.Now()
	_ = d.Store.PutRequest(ctx, r)
	a, _ := d.Store.API(ctx, r.APIID)
	d.audit(ctx, audit.You, "request.access", a.Name+"."+r.Name, access)
	d.notifyToolkitsUsingAPI(ctx, a.Name)
	return &r, nil
}

func (d *Deck) DeleteRequest(ctx context.Context, id string) error {
	r, err := d.Store.Request(ctx, id)
	if err != nil {
		return notFound("request")
	}
	a, _ := d.Store.API(ctx, r.APIID)
	full := a.Name + "." + r.Name
	tools, _ := d.Store.Tools(ctx)
	for _, t := range tools {
		if v, err := d.Store.LiveVersion(ctx, t.ID); err == nil {
			for _, c := range v.Calls {
				if c == full {
					return conflict("%s is used by the tool %s; change the tool first", full, t.Name)
				}
			}
		}
	}
	_ = d.Store.Delete(ctx, "request", id)
	d.audit(ctx, audit.You, "request.delete", full, "")
	return nil
}

// SaveExample stores a response as the request's example, masked (EX-1).
func (d *Deck) SaveExample(ctx context.Context, id string, body json.RawMessage) (*store.Request, error) {
	r, err := d.Store.Request(ctx, id)
	if err != nil {
		return nil, notFound("request")
	}
	if !json.Valid(body) {
		return nil, badRequest("examples must be JSON")
	}
	r.Example = proxy.MaskJSON(body)
	r.UpdatedAt = d.Clock.Now()
	_ = d.Store.PutRequest(ctx, r)
	return &r, nil
}

// SendInput is a send from the runner (RNR-1).
type SendInput struct {
	APIID     string         `json:"apiId"`
	RequestID string         `json:"requestId"`
	Draft     *store.Request `json:"draft"`
	EnvID     string         `json:"envId"`
}

// SendResult is the response pane.
type SendResult struct {
	*proxy.Response
	Body   string `json:"body"`
	Error  string `json:"error,omitempty"`
	Code   string `json:"code,omitempty"`
	Tokens int    `json:"tokens"`
}

func (d *Deck) Send(ctx context.Context, in SendInput) (*SendResult, error) {
	c := proxy.Call{APIID: in.APIID, RequestID: in.RequestID, Draft: in.Draft, EnvID: in.EnvID, Actor: proxy.Actor{Kind: "you", Name: "You"}}
	if in.Draft != nil {
		c.RequestID = ""
	}
	res, err := d.Proxy.Do(ctx, c)
	if err != nil {
		out := &SendResult{Error: err.Error()}
		var pe *proxy.Error
		if ok := asProxyErr(err, &pe); ok {
			out.Code, out.Error = pe.Code, pe.Message
		}
		return out, nil
	}
	return &SendResult{Response: res, Body: string(res.Body), Tokens: tokens.Count(string(res.Body))}, nil
}

func asProxyErr(err error, pe **proxy.Error) bool {
	e, ok := err.(*proxy.Error)
	if ok {
		*pe = e
	}
	return ok
}

// History lists your sends to an API, newest first (RNR-2).
func (d *Deck) History(ctx context.Context, apiID string) ([]store.Call, error) {
	calls, err := d.Store.Calls(ctx, store.CallFilter{Kind: "upstream", Limit: 500})
	if err != nil {
		return nil, err
	}
	out := []store.Call{}
	for _, c := range calls {
		if c.APIID == apiID && c.ParentID == "" && c.ActorKind == "you" {
			out = append(out, c)
		}
	}
	return out, nil
}

// Environments and variables.

func (d *Deck) CreateEnvironment(ctx context.Context, apiID, name string) (*store.Environment, error) {
	if _, err := d.Store.API(ctx, apiID); err != nil {
		return nil, notFound("API")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, badRequest("name the environment, like Staging")
	}
	e := store.Environment{ID: d.IDs.New(), APIID: apiID, Name: name}
	if err := d.Store.PutEnvironment(ctx, e); err != nil {
		return nil, err
	}
	// Copy variable names (not secret values) from the active environment.
	a, _ := d.Store.API(ctx, apiID)
	vars, _ := d.Store.Variables(ctx, apiID)
	for _, v := range vars {
		if v.EnvID == a.ActiveEnvID {
			nv := store.Variable{ID: d.IDs.New(), APIID: apiID, EnvID: e.ID, Name: v.Name, Source: v.Source, UpdatedAt: d.Clock.Now()}
			if v.Source != "secret" {
				nv.Value = v.Value
			}
			_ = d.Store.PutVariable(ctx, nv)
		}
	}
	d.audit(ctx, audit.You, "environment.create", a.Name+"/"+name, "")
	return &e, nil
}

func (d *Deck) DeleteEnvironment(ctx context.Context, id string) error {
	e, err := d.Store.Environment(ctx, id)
	if err != nil {
		return notFound("environment")
	}
	a, _ := d.Store.API(ctx, e.APIID)
	if a.ActiveEnvID == id {
		return conflict("%s is the active environment; switch to another first", e.Name)
	}
	vars, _ := d.Store.Variables(ctx, e.APIID)
	for _, v := range vars {
		if v.EnvID == id {
			_ = d.Store.Delete(ctx, "variable", v.ID)
		}
	}
	_ = d.Store.Delete(ctx, "environment", id)
	d.audit(ctx, audit.You, "environment.delete", a.Name+"/"+e.Name, "")
	return nil
}

// SetVariableInput sets a variable. A secret's value is write-only.
type SetVariableInput struct {
	ID     string `json:"id"`
	EnvID  string `json:"envId"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Value  string `json:"value"`
}

var varNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.\-]{0,63}$`)

func (d *Deck) SetVariable(ctx context.Context, apiID string, in SetVariableInput) (*VariableView, error) {
	a, err := d.Store.API(ctx, apiID)
	if err != nil {
		return nil, notFound("API")
	}
	if !varNameRe.MatchString(in.Name) {
		return nil, badRequest("variable names use letters, digits, _ . -")
	}
	if in.Source == "" {
		in.Source = "literal"
	}
	if in.Source != "literal" && in.Source != "secret" && in.Source != "os_env" {
		return nil, badRequest("source must be literal, secret or os_env")
	}
	v := store.Variable{ID: in.ID, APIID: apiID, EnvID: in.EnvID, Name: in.Name}
	if in.ID != "" {
		old, err := d.Store.Variable(ctx, in.ID)
		if err != nil || old.APIID != apiID {
			return nil, notFound("variable")
		}
		v = old
		v.Name = in.Name
	} else {
		v.ID = d.IDs.New()
	}
	v.Source = in.Source
	v.UpdatedAt = d.Clock.Now()
	if in.Source == "secret" {
		v.Value = ""
		if in.Value != "" {
			v.Ciphertext, err = d.Vault.Seal(in.Value)
			if err != nil {
				return nil, err
			}
		}
		d.audit(ctx, audit.You, "credential.set", a.Name+"."+v.Name, "value replaced")
	} else {
		v.Value, v.Ciphertext = in.Value, ""
	}
	if err := d.Store.PutVariable(ctx, v); err != nil {
		return nil, err
	}
	d.Proxy.ClearCookies(apiID)
	return &VariableView{ID: v.ID, EnvID: v.EnvID, Name: v.Name, Source: v.Source, Value: map[bool]string{true: "", false: v.Value}[v.Source == "secret"],
		IsSet: v.Source != "secret" || v.Ciphertext != ""}, nil
}

func (d *Deck) DeleteVariable(ctx context.Context, id string) error {
	v, err := d.Store.Variable(ctx, id)
	if err != nil {
		return notFound("variable")
	}
	_ = d.Store.Delete(ctx, "variable", id)
	if v.Source == "secret" {
		a, _ := d.Store.API(ctx, v.APIID)
		d.audit(ctx, audit.You, "credential.delete", a.Name+"."+v.Name, "")
	}
	return nil
}

func (d *Deck) notifyToolkitsUsingAPI(ctx context.Context, apiName string) {
	tks, _ := d.Store.Toolkits(ctx)
	var ids []string
	for _, tk := range tks {
		ids = append(ids, tk.ID)
	}
	_ = apiName
	d.toolsChanged(ids)
}
