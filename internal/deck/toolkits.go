package deck

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/tokens"
	"github.com/abhijeet-oxide/mcpdeck/internal/tool"
)

// ToolRow is one tool in a toolkit or the library.
type ToolRow struct {
	store.Tool
	Live      *store.ToolVersion `json:"live"`
	Latest    store.ToolVersion  `json:"latest"`
	Toolkits  []string           `json:"toolkits"`
	Calls7d   int                `json:"calls7d"`
	P50       int64              `json:"p50"`
	AvgTokens int                `json:"avgTokens"`
	Errors7d  int                `json:"errors7d"`
}

// ToolkitSummary is a card on the Toolkits page.
type ToolkitSummary struct {
	store.Toolkit
	Tools      int      `json:"tools"`
	Agents     int      `json:"agents"`
	Pending    int      `json:"pending"`
	Requests   int      `json:"requests"`
	ListTokens int      `json:"listTokens"`
	Endpoint   string   `json:"endpoint"`
	ScopeNames []string `json:"scopeNames"`
}

// GrantRow is a grant with its agent.
type GrantRow struct {
	store.Grant
	Agent       store.Agent `json:"agent"`
	ToolkitName string      `json:"toolkitName"`
	Connected   bool        `json:"connected"`
}

// ToolkitDetail is the toolkit page.
type ToolkitDetail struct {
	ToolkitSummary
	ToolRows      []ToolRow           `json:"toolRows"`
	Grants        []GrantRow          `json:"grants"`
	ToolRequests  []store.ToolRequest `json:"toolRequests"`
	PendingList   []store.ToolVersion `json:"pendingList"`
	BuildEndpoint string              `json:"buildEndpoint"`
}

func (d *Deck) toolRow(ctx context.Context, t store.Tool, since time.Time) ToolRow {
	row := ToolRow{Tool: t, Toolkits: []string{}}
	vs, _ := d.Store.Versions(ctx, t.ID)
	if len(vs) > 0 {
		row.Latest = vs[0]
	}
	for i := range vs {
		if vs[i].Status == "live" {
			v := vs[i]
			row.Live = &v
			break
		}
	}
	ids, _ := d.Store.ToolToolkits(ctx, t.ID)
	for _, id := range ids {
		if tk, err := d.Store.Toolkit(ctx, id); err == nil {
			row.Toolkits = append(row.Toolkits, tk.Name)
		}
	}
	calls, _ := d.Store.Calls(ctx, store.CallFilter{Kind: "tool", Name: t.Name, Since: since, Limit: 5000})
	var durs []int64
	tok := 0
	for _, c := range calls {
		if c.Name != t.Name {
			continue
		}
		row.Calls7d++
		durs = append(durs, c.DurationMS)
		tok += c.Tokens
		if c.Status != "ok" {
			row.Errors7d++
		}
	}
	if row.Calls7d > 0 {
		row.P50 = percentile(durs, 50)
		row.AvgTokens = tok / row.Calls7d
	}
	return row
}

func percentile(v []int64, p int) int64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]int64{}, v...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	idx := (len(s)*p + 99) / 100
	if idx < 1 {
		idx = 1
	}
	return s[idx-1]
}

// ListTokens is the standing cost of a grant's tools/list (SES-3).
func (d *Deck) ListTokens(ctx context.Context, tk store.Toolkit, access string, canBuild bool) int {
	defs, truncated := d.mcpToolDefs(ctx, tk, access)
	_ = canBuild
	b, _ := json.Marshal(map[string]any{"tools": append(defs, d.useExtras(truncated)...)})
	return tokens.Count(string(b))
}

func (d *Deck) summary(ctx context.Context, tk store.Toolkit) ToolkitSummary {
	tools, _ := d.Store.ToolkitTools(ctx, tk.ID)
	grants, _ := d.Store.Grants(ctx, "", tk.ID)
	reqs, _ := d.Store.ToolRequests(ctx, tk.ID, "open")
	s := ToolkitSummary{Toolkit: tk, Tools: len(tools), Agents: len(grants), Requests: len(reqs), Endpoint: d.Endpoint(tk.Name, false), ScopeNames: []string{}}
	for _, t := range tools {
		vs, _ := d.Store.Versions(ctx, t.ID)
		for _, v := range vs {
			if v.Status == "pending" {
				s.Pending++
			}
		}
	}
	for _, id := range d.scopeIDs(ctx, tk) {
		if a, err := d.Store.API(ctx, id); err == nil {
			s.ScopeNames = append(s.ScopeNames, a.Name)
		}
	}
	sort.Strings(s.ScopeNames)
	s.ListTokens = d.ListTokens(ctx, tk, "read_write", false)
	return s
}

func (d *Deck) ListToolkits(ctx context.Context) ([]ToolkitSummary, error) {
	tks, err := d.Store.Toolkits(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ToolkitSummary, 0, len(tks))
	for _, tk := range tks {
		out = append(out, d.summary(ctx, tk))
	}
	return out, nil
}

func (d *Deck) toolkitByRef(ctx context.Context, ref string) (store.Toolkit, error) {
	if tk, err := d.Store.Toolkit(ctx, ref); err == nil {
		return tk, nil
	}
	tk, err := d.Store.ToolkitByName(ctx, ref)
	if err != nil {
		return tk, notFound("toolkit " + ref)
	}
	return tk, nil
}

// ToolkitByRef finds a toolkit by ID or name.
func (d *Deck) ToolkitByRef(ctx context.Context, ref string) (store.Toolkit, error) {
	return d.toolkitByRef(ctx, ref)
}

func (d *Deck) GetToolkit(ctx context.Context, ref string) (*ToolkitDetail, error) {
	tk, err := d.toolkitByRef(ctx, ref)
	if err != nil {
		return nil, err
	}
	out := &ToolkitDetail{ToolkitSummary: d.summary(ctx, tk), ToolRows: []ToolRow{}, Grants: []GrantRow{}, PendingList: []store.ToolVersion{},
		BuildEndpoint: d.Endpoint(tk.Name, true)}
	since := d.Clock.Now().Add(-7 * 24 * time.Hour)
	tools, _ := d.Store.ToolkitTools(ctx, tk.ID)
	for _, t := range tools {
		row := d.toolRow(ctx, t, since)
		out.ToolRows = append(out.ToolRows, row)
		vs, _ := d.Store.Versions(ctx, t.ID)
		for _, v := range vs {
			if v.Status == "pending" {
				out.PendingList = append(out.PendingList, v)
			}
		}
	}
	grants, _ := d.Store.Grants(ctx, "", tk.ID)
	for _, g := range grants {
		a, _ := d.Store.Agent(ctx, g.AgentID)
		out.Grants = append(out.Grants, GrantRow{Grant: g, Agent: a, ToolkitName: tk.Name, Connected: d.connected(a)})
	}
	out.ToolRequests, _ = d.Store.ToolRequests(ctx, tk.ID, "")
	return out, nil
}

// ToolkitInput creates or updates a toolkit.
type ToolkitInput struct {
	Name        *string      `json:"name"`
	Description *string      `json:"description"`
	BuildScope  *[]string    `json:"buildScope"` // API IDs or names
	Rules       *store.Rules `json:"rules"`
	Hints       *bool        `json:"hints"`
	HintTokens  *int         `json:"hintTokens"`
}

func (d *Deck) resolveScope(ctx context.Context, refs []string) ([]string, error) {
	out := []string{}
	for _, r := range refs {
		if a, err := d.Store.API(ctx, r); err == nil {
			out = append(out, a.ID)
			continue
		}
		a, err := d.Store.APIByName(ctx, r)
		if err != nil {
			return nil, badRequest("no API named %s", r)
		}
		out = append(out, a.ID)
	}
	return out, nil
}

func (d *Deck) CreateToolkit(ctx context.Context, in ToolkitInput) (*store.Toolkit, error) {
	if in.Name == nil {
		return nil, badRequest("name the toolkit, like support")
	}
	name := Slug(*in.Name)
	if len(name) < 2 {
		return nil, badRequest("toolkit names use lowercase letters, digits and dashes, like support or finance-ops")
	}
	if _, err := d.Store.ToolkitByName(ctx, name); err == nil {
		return nil, conflict("a toolkit named %s already exists", name)
	}
	tk := store.Toolkit{ID: d.IDs.New(), Name: name, Rules: store.DefaultRules(), Hints: true, HintTokens: 2000, CreatedAt: d.Clock.Now(), BuildScope: []string{}}
	if in.Description != nil {
		tk.Description = strings.TrimSpace(*in.Description)
	}
	if in.BuildScope != nil {
		ids, err := d.resolveScope(ctx, *in.BuildScope)
		if err != nil {
			return nil, err
		}
		tk.BuildScope = ids
	}
	if in.Rules != nil {
		tk.Rules = *in.Rules
	}
	if err := d.Store.PutToolkit(ctx, tk); err != nil {
		return nil, err
	}
	d.audit(ctx, audit.You, "toolkit.create", tk.Name, "")
	return &tk, nil
}

func (d *Deck) UpdateToolkit(ctx context.Context, ref string, in ToolkitInput) (*store.Toolkit, error) {
	tk, err := d.toolkitByRef(ctx, ref)
	if err != nil {
		return nil, err
	}
	var changed []string
	if in.Description != nil {
		tk.Description = strings.TrimSpace(*in.Description)
		changed = append(changed, "description")
	}
	if in.BuildScope != nil {
		ids, err := d.resolveScope(ctx, *in.BuildScope)
		if err != nil {
			return nil, err
		}
		tk.BuildScope = ids
		changed = append(changed, "build list")
	}
	if in.Rules != nil {
		tk.Rules = *in.Rules
		changed = append(changed, "approval rules")
	}
	if in.Hints != nil {
		tk.Hints = *in.Hints
	}
	if in.HintTokens != nil && *in.HintTokens > 0 {
		tk.HintTokens = *in.HintTokens
	}
	if err := d.Store.PutToolkit(ctx, tk); err != nil {
		return nil, err
	}
	d.audit(ctx, audit.You, "toolkit.update", tk.Name, strings.Join(changed, ", "))
	d.toolsChanged([]string{tk.ID})
	return &tk, nil
}

func (d *Deck) DeleteToolkit(ctx context.Context, ref string) error {
	return d.DeleteToolkitForce(ctx, ref, false)
}

// DeleteToolkitForce deletes a toolkit; with force, agents' access to it is
// removed first (the person confirmed by typing its name in the UI).
func (d *Deck) DeleteToolkitForce(ctx context.Context, ref string, force bool) error {
	tk, err := d.toolkitByRef(ctx, ref)
	if err != nil {
		return err
	}
	grants, _ := d.Store.Grants(ctx, "", tk.ID)
	if len(grants) > 0 && !force {
		return conflict("%d agents use %s; remove their access first", len(grants), tk.Name)
	}
	for _, g := range grants {
		if err := d.DeleteGrant(ctx, g.ID); err != nil {
			return err
		}
	}
	tools, _ := d.Store.ToolkitTools(ctx, tk.ID)
	for _, t := range tools {
		_ = d.Store.RemoveToolFromToolkit(ctx, tk.ID, t.ID)
	}
	_ = d.Store.Delete(ctx, "toolkit", tk.ID)
	d.audit(ctx, audit.You, "toolkit.delete", tk.Name, "")
	return nil
}

// AddToolToToolkit puts a library tool in a toolkit (TOOL-11).
func (d *Deck) AddToolToToolkit(ctx context.Context, ref, toolID string) error {
	tk, err := d.toolkitByRef(ctx, ref)
	if err != nil {
		return err
	}
	t, err := d.toolByRef(ctx, toolID)
	if err != nil {
		return err
	}
	_ = d.Store.AddToolToToolkit(ctx, tk.ID, t.ID)
	d.audit(ctx, audit.You, "toolkit.add_tool", tk.Name, t.Name)
	d.toolsChanged([]string{tk.ID})
	return nil
}

func (d *Deck) RemoveToolFromToolkit(ctx context.Context, ref, toolID string) error {
	tk, err := d.toolkitByRef(ctx, ref)
	if err != nil {
		return err
	}
	t, err := d.toolByRef(ctx, toolID)
	if err != nil {
		return err
	}
	_ = d.Store.RemoveToolFromToolkit(ctx, tk.ID, t.ID)
	d.audit(ctx, audit.You, "toolkit.remove_tool", tk.Name, t.Name)
	d.toolsChanged([]string{tk.ID})
	return nil
}

func (d *Deck) toolByRef(ctx context.Context, ref string) (store.Tool, error) {
	if t, err := d.Store.Tool(ctx, ref); err == nil {
		return t, nil
	}
	t, err := d.Store.ToolByName(ctx, ref)
	if err != nil {
		return t, notFound("tool " + ref)
	}
	return t, nil
}

// Library lists every tool (All tools).
func (d *Deck) Library(ctx context.Context) ([]ToolRow, error) {
	tools, err := d.Store.Tools(ctx)
	if err != nil {
		return nil, err
	}
	since := d.Clock.Now().Add(-7 * 24 * time.Hour)
	out := make([]ToolRow, 0, len(tools))
	for _, t := range tools {
		out = append(out, d.toolRow(ctx, t, since))
	}
	return out, nil
}

// FieldUsage is FB-2 for one output field.
type FieldUsage struct {
	Path   string   `json:"path"`
	Used   []string `json:"used"`
	Unused []string `json:"unused"`
}

// ToolDetail is the tool page.
type ToolDetail struct {
	ToolRow
	Versions []store.ToolVersion `json:"versions"`
	Fields   []FieldUsage        `json:"fields"`
	Reports  []store.FieldReport `json:"reports"`
}

func (d *Deck) GetTool(ctx context.Context, ref string) (*ToolDetail, error) {
	t, err := d.toolByRef(ctx, ref)
	if err != nil {
		return nil, err
	}
	out := &ToolDetail{ToolRow: d.toolRow(ctx, t, d.Clock.Now().Add(-7*24*time.Hour)), Fields: []FieldUsage{}}
	out.Versions, _ = d.Store.Versions(ctx, t.ID)
	out.Reports, _ = d.Store.FieldReports(ctx, t.ID)
	since := d.Clock.Now().Add(-30 * 24 * time.Hour)
	byPath := map[string]*FieldUsage{}
	add := func(p, agent string, used bool) {
		f, ok := byPath[p]
		if !ok {
			f = &FieldUsage{Path: p, Used: []string{}, Unused: []string{}}
			byPath[p] = f
		}
		list := &f.Unused
		if used {
			list = &f.Used
		}
		for _, x := range *list {
			if x == agent {
				return
			}
		}
		*list = append(*list, agent)
	}
	for _, r := range out.Reports {
		if r.CreatedAt.Before(since) {
			continue
		}
		for _, p := range r.Used {
			add(p, r.AgentName, true)
		}
		for _, p := range r.Unused {
			add(p, r.AgentName, false)
		}
	}
	for _, f := range byPath {
		out.Fields = append(out.Fields, *f)
	}
	sort.Slice(out.Fields, func(i, j int) bool { return out.Fields[i].Path < out.Fields[j].Path })
	return out, nil
}

// clients returns the APIs (and their requests) a run may use. With scope,
// only those API IDs.
func (d *Deck) clients(ctx context.Context, scope []string) []tool.Client {
	apis, _ := d.Store.APIs(ctx)
	allowed := map[string]bool{}
	for _, id := range scope {
		allowed[id] = true
	}
	var out []tool.Client
	for _, a := range apis {
		if scope != nil && !allowed[a.ID] {
			continue
		}
		reqs, _ := d.Store.Requests(ctx, a.ID)
		// Sign-in requests run only inside mcpDeck, never from a tool (CRED-8).
		skip := map[string]bool{}
		for _, id := range a.Auth.AuthRequestIDs() {
			skip[id] = true
		}
		kept := reqs[:0:0]
		for _, r := range reqs {
			if !skip[r.ID] {
				kept = append(kept, r)
			}
		}
		out = append(out, tool.Client{API: a, Requests: kept})
	}
	return out
}

// scopeIDs is the toolkit's build scope: its list, or the APIs its tools use (TK-4).
func (d *Deck) scopeIDs(ctx context.Context, tk store.Toolkit) []string {
	if len(tk.BuildScope) > 0 {
		return tk.BuildScope
	}
	seen := map[string]bool{}
	out := []string{}
	tools, _ := d.Store.ToolkitTools(ctx, tk.ID)
	for _, t := range tools {
		vs, _ := d.Store.Versions(ctx, t.ID)
		if len(vs) == 0 {
			continue
		}
		for _, name := range vs[0].APIs {
			if a, err := d.Store.APIByName(ctx, name); err == nil && !seen[a.ID] {
				seen[a.ID] = true
				out = append(out, a.ID)
			}
		}
	}
	return out
}

// TypesFor is the .d.ts for a toolkit's editor (all APIs, so people can reach any).
func (d *Deck) TypesFor(ctx context.Context, toolkitRef string) (string, error) {
	return tool.DTS(d.clients(ctx, nil)), nil
}

// Caller identifies who runs or saves a tool.
type Caller struct {
	Actor     audit.Actor
	Grant     *store.Grant // nil for you
	Toolkit   *store.Toolkit
	SessionID string
	ParentID  string
}

func (c Caller) byAgent() bool { return c.Actor.Kind == "agent" }

func (c Caller) proxyBase() proxy.Call {
	b := proxy.Call{Actor: proxy.Actor{Kind: c.Actor.Kind, ID: c.Actor.ID, Name: c.Actor.Name}, SessionID: c.SessionID, ParentID: c.ParentID}
	if c.Grant != nil {
		b.Access = c.Grant.Access
	}
	if c.Toolkit != nil {
		b.ToolkitID, b.ToolkitName = c.Toolkit.ID, c.Toolkit.Name
	}
	return b
}

func (d *Deck) env(ctx context.Context, c Caller, examples bool) tool.Env {
	var scope []string
	if c.byAgent() && c.Toolkit != nil {
		scope = d.scopeIDs(ctx, *c.Toolkit)
	}
	base := c.proxyBase()
	base.Examples = examples
	return tool.Env{Clients: d.clients(ctx, scope), Sender: d.Proxy, Base: base, Timeout: d.toolTimeout(ctx)}
}

func (d *Deck) toolTimeout(ctx context.Context) time.Duration {
	var sec int
	if _, err := fmt.Sscan(d.Store.Setting(ctx, "toolTimeoutSeconds", "10"), &sec); err != nil || sec <= 0 {
		sec = 10
	}
	return time.Duration(sec) * time.Second
}

// TestInput runs unsaved tool code (TOOL-7).
type TestInput struct {
	Source   string `json:"source"`
	Input    any    `json:"input"`
	Examples bool   `json:"examples"`
}

func (d *Deck) TestTool(ctx context.Context, c Caller, in TestInput) (tool.Result, error) {
	def, err := tool.Load(in.Source)
	if err != nil {
		return tool.Result{Error: err.Error(), Console: []string{}, Calls: []tool.Upstream{}}, nil
	}
	if err := tool.ValidateInput(def.InputSchema, in.Input); err != nil {
		return tool.Result{Error: err.Error(), Console: []string{}, Calls: []tool.Upstream{}}, nil
	}
	return tool.Run(ctx, in.Source, in.Input, d.env(ctx, c, in.Examples)), nil
}

// RunScript runs exploratory code (run_script).
func (d *Deck) RunScript(ctx context.Context, c Caller, src string, input any) tool.Result {
	return tool.RunScript(ctx, src, input, d.env(ctx, c, false))
}

// SaveInput saves a tool file as a new version.
type SaveInput struct {
	Source  string           `json:"source"`
	Tests   []store.ToolTest `json:"tests"`
	Fulfils string           `json:"fulfils"`
	Note    string           `json:"note"`
}

// SaveResult says what happened.
type SaveResult struct {
	Version  store.ToolVersion `json:"version"`
	Status   string            `json:"status"`
	Checks   []store.Check     `json:"checks"`
	Warnings []string          `json:"warnings"`
	Message  string            `json:"message"`
}

// SaveTool creates version N+1 of a tool in a toolkit (VER-1, AB-1…AB-6).
func (d *Deck) SaveTool(ctx context.Context, toolkitRef string, c Caller, in SaveInput) (*SaveResult, error) {
	tk, err := d.toolkitByRef(ctx, toolkitRef)
	if err != nil {
		return nil, err
	}
	c.Toolkit = &tk
	def, err := tool.Load(in.Source)
	if err != nil {
		return nil, badRequest("%s", err.Error())
	}
	all := d.clients(ctx, nil)
	calls, apis := tool.UsedCalls(in.Source, all)
	access := "read"
	byFull := map[string]store.Request{}
	for _, cl := range all {
		for _, r := range cl.Requests {
			byFull[cl.API.Name+"."+r.Name] = r
		}
	}
	for _, k := range calls {
		if r, ok := byFull[k]; ok {
			if r.EffectiveAccess() == "write" {
				access = "write"
			}
			if strings.ToUpper(r.Method) == "POST" && proxy.IsGraphQLPath(strings.TrimPrefix(r.URL, "{{baseUrl}}")) && proxy.IsGraphQLMutation(in.Source) {
				access = "write"
			}
		} else if parts := strings.Fields(k); len(parts) >= 3 {
			// A raw call: by method, except a GraphQL POST whose code
			// sends no mutation (TOOL-9).
			if proxy.ClassifyMethod(parts[1]) == "write" && (!proxy.IsGraphQLPath(parts[2]) || strings.ToUpper(parts[1]) != "POST" || proxy.IsGraphQLMutation(in.Source)) {
				access = "write"
			}
		}
	}
	existing, terr := d.Store.ToolByName(ctx, def.Name)
	if terr == nil && c.byAgent() {
		ids, _ := d.Store.ToolToolkits(ctx, existing.ID)
		in := false
		for _, id := range ids {
			if id == tk.ID {
				in = true
			}
		}
		if !in {
			return nil, conflict("a tool named %s already exists in another toolkit; pick another name or ask the user to add it to %s", def.Name, tk.Name)
		}
	}
	now := d.Clock.Now()
	v := store.ToolVersion{ID: d.IDs.New(), Source: in.Source, Name: def.Name, Description: strings.TrimSpace(def.Description),
		UseWhen: def.UseWhen, DontUseWhen: def.DontUseWhen, InputSchema: def.InputSchema, Access: access,
		APIs: apis, Calls: calls, Tests: in.Tests, CreatedByKind: c.Actor.Kind, CreatedByID: c.Actor.ID, CreatedByName: c.Actor.Name,
		SessionID: c.SessionID, ToolkitID: tk.ID, Fulfils: in.Fulfils, Note: in.Note, CreatedAt: now}
	if v.APIs == nil {
		v.APIs = []string{}
	}
	if v.Calls == nil {
		v.Calls = []string{}
	}
	if v.Tests == nil {
		v.Tests = []store.ToolTest{}
	}
	var warnings []string
	for _, t := range in.Tests {
		var x any
		if err := json.Unmarshal(t.Input, &x); err != nil {
			return nil, badRequest("test %q has an input that isn't JSON", t.Name)
		}
		if err := tool.ValidateInput(def.InputSchema, x); err != nil {
			return nil, badRequest("test %q: %s", t.Name, err.Error())
		}
	}
	// Tests of tools that write run against saved examples, never the real API.
	env := d.env(ctx, c, access == "write")
	env.Clients = all
	v.TestResults = tool.RunTests(ctx, in.Source, in.Tests, env)
	// Duplicate check (AB-5).
	var dups []string
	if len(calls) > 0 {
		lib, _ := d.Store.Tools(ctx)
		for _, t := range lib {
			if t.Name == def.Name {
				continue
			}
			lv, err := d.Store.LiveVersion(ctx, t.ID)
			if err == nil && strings.Join(lv.Calls, ",") == strings.Join(calls, ",") {
				dups = append(dups, t.Name)
			}
		}
	}
	if len(dups) > 0 {
		warnings = append(warnings, "uses the same requests as "+strings.Join(dups, ", "))
	}
	scope := map[string]bool{}
	for _, id := range d.scopeIDs(ctx, tk) {
		if a, err := d.Store.API(ctx, id); err == nil {
			scope[a.Name] = true
		}
	}
	checks, status := tool.Evaluate(tool.ApprovalInput{Version: v, Rules: tk.Rules, ByAgent: c.byAgent(), Duplicates: dups,
		InScope: func(a string) bool { return !c.byAgent() || scope[a] }})
	v.Checks, v.Status = checks, status
	var t store.Tool
	if terr == nil {
		t = existing
	} else {
		t = store.Tool{ID: d.IDs.New(), Name: def.Name, CreatedAt: now}
		if err := d.Store.PutTool(ctx, t); err != nil {
			return nil, err
		}
	}
	_ = d.Store.AddToolToToolkit(ctx, tk.ID, t.ID)
	v.ToolID = t.ID
	n, err := d.Store.NextVersion(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	v.N = n
	if status == "live" {
		v.DecidedAt = &now
	}
	if err := d.Store.AddVersion(ctx, v); err != nil {
		return nil, err
	}
	d.audit(ctx, c.Actor, "tool.save", fmt.Sprintf("%s v%d", t.Name, v.N), status)
	res := &SaveResult{Version: v, Status: status, Checks: checks, Warnings: warnings}
	if status == "live" {
		d.publish(ctx, v)
		res.Message = fmt.Sprintf("%s v%d is live for every agent of %s.", t.Name, v.N, tk.Name)
	} else {
		var held []string
		for _, ch := range checks {
			if !ch.Passed {
				held = append(held, ch.Label+map[bool]string{true: ": " + ch.Detail, false: ""}[ch.Detail != ""])
			}
		}
		if !tk.Rules.AutoApprove {
			held = append(held, "auto-approval is off for this toolkit")
		}
		res.Message = fmt.Sprintf("%s v%d is pending review by a person. Held by: %s.", t.Name, v.N, strings.Join(held, "; "))
	}
	if in.Fulfils != "" {
		if r, err := d.Store.ToolRequest(ctx, in.Fulfils); err == nil {
			r.FulfilledBy = v.ID
			if status == "live" {
				r.State = "closed"
			}
			_ = d.Store.PutToolRequest(ctx, r)
		}
	}
	return res, nil
}

// publish retires older live versions and tells agents (AB-7).
func (d *Deck) publish(ctx context.Context, v store.ToolVersion) {
	vs, _ := d.Store.Versions(ctx, v.ToolID)
	for _, o := range vs {
		if o.ID != v.ID && o.Status == "live" {
			_ = d.Store.SetVersionStatus(ctx, o.ID, "retired", d.Clock.Now())
		}
	}
	reqs, _ := d.Store.ToolRequests(ctx, "", "open")
	for _, r := range reqs {
		if r.FulfilledBy == v.ID {
			r.State = "closed"
			_ = d.Store.PutToolRequest(ctx, r)
		}
	}
	ids, _ := d.Store.ToolToolkits(ctx, v.ToolID)
	d.toolsChanged(ids)
}

// Decide approves or rejects a pending version. Only a person can (AB-3, ACC-6).
func (d *Deck) Decide(ctx context.Context, versionID string, approve bool) (*store.ToolVersion, error) {
	v, err := d.Store.Version(ctx, versionID)
	if err != nil {
		return nil, notFound("version")
	}
	if v.Status != "pending" {
		return nil, conflict("v%d of %s is %s, not pending", v.N, v.Name, v.Status)
	}
	status := "rejected"
	if approve {
		status = "live"
	}
	if err := d.Store.SetVersionStatus(ctx, v.ID, status, d.Clock.Now()); err != nil {
		return nil, err
	}
	v.Status = status
	d.audit(ctx, audit.You, map[bool]string{true: "tool.approve", false: "tool.reject"}[approve], fmt.Sprintf("%s v%d", v.Name, v.N), "")
	if approve {
		d.publish(ctx, v)
	}
	return &v, nil
}

// Rollback makes a new version equal to an older one (VER-4).
func (d *Deck) Rollback(ctx context.Context, versionID string) (*SaveResult, error) {
	old, err := d.Store.Version(ctx, versionID)
	if err != nil {
		return nil, notFound("version")
	}
	tk := old.ToolkitID
	if tk == "" {
		ids, _ := d.Store.ToolToolkits(ctx, old.ToolID)
		if len(ids) == 0 {
			return nil, conflict("the tool isn't in any toolkit")
		}
		tk = ids[0]
	}
	return d.SaveTool(ctx, tk, Caller{Actor: audit.You}, SaveInput{Source: old.Source, Tests: old.Tests, Note: fmt.Sprintf("roll back to v%d", old.N)})
}

// DeleteTool removes a tool and its versions from the library.
func (d *Deck) DeleteTool(ctx context.Context, ref string) error {
	t, err := d.toolByRef(ctx, ref)
	if err != nil {
		return err
	}
	ids, _ := d.Store.ToolToolkits(ctx, t.ID)
	for _, id := range ids {
		_ = d.Store.RemoveToolFromToolkit(ctx, id, t.ID)
	}
	vs, _ := d.Store.Versions(ctx, t.ID)
	for _, v := range vs {
		if v.Status == "live" || v.Status == "pending" {
			_ = d.Store.SetVersionStatus(ctx, v.ID, "retired", d.Clock.Now())
		}
	}
	_ = d.Store.Delete(ctx, "tool", t.ID)
	d.audit(ctx, audit.You, "tool.delete", t.Name, "")
	d.toolsChanged(ids)
	return nil
}

// AgentView is what an agent with a given grant receives from a toolkit's
// use endpoint: the initialize instructions and tools/list (TK-5).
type AgentView struct {
	Toolkit      string    `json:"toolkit"`
	Endpoint     string    `json:"endpoint"`
	Access       string    `json:"access"`
	CanBuild     bool      `json:"canBuild"`
	Client       string    `json:"client"`
	Instructions string    `json:"instructions"`
	Tools        []ToolDef `json:"tools"`
	Tokens       int       `json:"tokens"`
}

// AgentView builds the preview from the same functions the MCP server uses.
func (d *Deck) AgentView(ctx context.Context, ref, access string, canBuild bool, client string) (*AgentView, error) {
	tk, err := d.toolkitByRef(ctx, ref)
	if err != nil {
		return nil, err
	}
	if access != "read" && access != "read_write" {
		access = "read"
	}
	defs, truncated := d.mcpToolDefs(ctx, tk, access)
	defs = append(defs, d.useExtras(truncated)...)
	// MCP servers list tools by name; show them in the order a client gets them.
	sort.SliceStable(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	b, _ := json.Marshal(map[string]any{"tools": defs})
	g := store.Grant{Access: access, CanBuild: canBuild}
	return &AgentView{Toolkit: tk.Name, Endpoint: d.Endpoint(tk.Name, false), Access: access, CanBuild: canBuild, Client: client,
		Instructions: d.UseInstructions(tk, g, client), Tools: defs, Tokens: tokens.Count(string(b))}, nil
}
