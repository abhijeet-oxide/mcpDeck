package deck

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// CallDetail is a call with its upstream calls.
type CallDetail struct {
	store.Call
	Children []store.Call `json:"children"`
}

func (d *Deck) Calls(ctx context.Context, f store.CallFilter) ([]store.Call, error) {
	return d.Store.Calls(ctx, f)
}

func (d *Deck) GetCall(ctx context.Context, id string) (*CallDetail, error) {
	c, err := d.Store.Call(ctx, id)
	if err != nil {
		return nil, notFound("call")
	}
	ch, _ := d.Store.Calls(ctx, store.CallFilter{ParentID: id, Limit: 500})
	sort.Slice(ch, func(i, j int) bool { return ch[i].StartedAt.Before(ch[j].StartedAt) })
	return &CallDetail{Call: c, Children: ch}, nil
}

// UsageRow is USE-1 for one tool, toolkit or agent.
type UsageRow struct {
	Key        string  `json:"key"`
	Label      string  `json:"label"`
	Calls      int     `json:"calls"`
	Errors     int     `json:"errors"`
	ErrorRate  float64 `json:"errorRate"`
	P50        int64   `json:"p50"`
	P95        int64   `json:"p95"`
	AvgTokens  int     `json:"avgTokens"`
	Tokens     int     `json:"tokens"`
	Upstream   float64 `json:"upstreamPerCall"`
	ListTokens int     `json:"listTokens"`
}

// Usage aggregates.
type Usage struct {
	Range    string     `json:"range"`
	Tools    []UsageRow `json:"tools"`
	Toolkits []UsageRow `json:"toolkits"`
	Agents   []UsageRow `json:"agents"`
	Daily    []DayPoint `json:"daily"`
	Encoder  string     `json:"encoder"`
}

// DayPoint is one bar in the usage chart.
type DayPoint struct {
	Day    string `json:"day"`
	Calls  int    `json:"calls"`
	Tokens int    `json:"tokens"`
	Errors int    `json:"errors"`
}

func rangeDur(r string) time.Duration {
	switch r {
	case "24h":
		return 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	}
	return 7 * 24 * time.Hour
}

func (d *Deck) Usage(ctx context.Context, rng string) (*Usage, error) {
	if rng == "" {
		rng = "7d"
	}
	since := d.Clock.Now().Add(-rangeDur(rng))
	calls, err := d.Store.Calls(ctx, store.CallFilter{Since: since, Limit: 100000})
	if err != nil {
		return nil, err
	}
	type acc struct {
		row  UsageRow
		durs []int64
		ups  int
	}
	tools, tks, agents := map[string]*acc{}, map[string]*acc{}, map[string]*acc{}
	upstreamByParent := map[string]int{}
	for _, c := range calls {
		if c.ParentID != "" {
			upstreamByParent[c.ParentID]++
		}
	}
	tkNames := map[string]string{}
	if list, err := d.Store.Toolkits(ctx); err == nil {
		for _, tk := range list {
			tkNames[tk.ID] = tk.Name
		}
	}
	days := map[string]*DayPoint{}
	add := func(m map[string]*acc, key, label string, c store.Call) {
		a, ok := m[key]
		if !ok {
			a = &acc{row: UsageRow{Key: key, Label: label}}
			m[key] = a
		}
		if c.Kind == "list" {
			if c.Tokens > a.row.ListTokens {
				a.row.ListTokens = c.Tokens
			}
			return
		}
		a.row.Calls++
		a.row.Tokens += c.Tokens
		a.durs = append(a.durs, c.DurationMS)
		a.ups += upstreamByParent[c.ID]
		if c.Status != "ok" {
			a.row.Errors++
		}
	}
	for _, c := range calls {
		if c.ParentID != "" || c.ActorKind != "agent" {
			continue
		}
		day := c.StartedAt.UTC().Format("2006-01-02")
		dp, ok := days[day]
		if !ok {
			dp = &DayPoint{Day: day}
			days[day] = dp
		}
		if c.Kind != "list" {
			dp.Calls++
			dp.Tokens += c.Tokens
			if c.Status != "ok" {
				dp.Errors++
			}
		}
		if c.Kind == "tool" {
			add(tools, c.Name, c.Name, c)
		}
		add(tks, c.ToolkitID, tkNames[c.ToolkitID], c)
		add(agents, c.ActorID, c.ActorName, c)
	}
	finish := func(m map[string]*acc) []UsageRow {
		out := []UsageRow{}
		for _, a := range m {
			r := a.row
			if r.Calls > 0 {
				r.ErrorRate = float64(r.Errors) / float64(r.Calls)
				r.AvgTokens = r.Tokens / r.Calls
				r.Upstream = float64(a.ups) / float64(r.Calls)
				r.P50, r.P95 = percentile(a.durs, 50), percentile(a.durs, 95)
			}
			out = append(out, r)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Calls > out[j].Calls })
		return out
	}
	u := &Usage{Range: rng, Tools: finish(tools), Toolkits: finish(tks), Agents: finish(agents), Daily: []DayPoint{}, Encoder: "cl100k_base"}
	n := int(rangeDur(rng) / (24 * time.Hour))
	if n < 1 {
		n = 1
	}
	for i := n - 1; i >= 0; i-- {
		day := d.Clock.Now().Add(-time.Duration(i) * 24 * time.Hour).UTC().Format("2006-01-02")
		if dp, ok := days[day]; ok {
			u.Daily = append(u.Daily, *dp)
		} else {
			u.Daily = append(u.Daily, DayPoint{Day: day})
		}
	}
	return u, nil
}

// Step is one onboarding step (UI-11).
type Step struct {
	Key    string `json:"key"`
	Done   bool   `json:"done"`
	Reason string `json:"reason,omitempty"`
}

// NeedsYou is one item waiting for a person.
type NeedsYou struct {
	Kind    string `json:"kind"` // review | request | blocked
	ID      string `json:"id"`
	Title   string `json:"title"`
	Detail  string `json:"detail"`
	Toolkit string `json:"toolkit"`
	Agent   string `json:"agent"`
	At      string `json:"at"`
	ToolID  string `json:"toolId,omitempty"`
}

// Home is the Home page (UI-1).
type Home struct {
	APIs           int          `json:"apis"`
	Requests       int          `json:"requests"`
	Toolkits       int          `json:"toolkits"`
	Tools          int          `json:"tools"`
	Agents         int          `json:"agents"`
	ActiveAgents   int          `json:"activeAgents"`
	CallsToday     int          `json:"callsToday"`
	TokensToday    int          `json:"tokensToday"`
	ErrorsToday    int          `json:"errorsToday"`
	BlockedToday   int          `json:"blockedToday"`
	NeedsYou       []NeedsYou   `json:"needsYou"`
	Recent         []store.Call `json:"recent"`
	Steps          []Step       `json:"steps"`
	HideOnboarding bool         `json:"hideOnboarding"`
	WelcomeSeen    bool         `json:"welcomeSeen"`
	Demo           bool         `json:"demo"`
	Daily          []DayPoint   `json:"daily"`
}

func (d *Deck) Home(ctx context.Context) (*Home, error) {
	h := &Home{NeedsYou: []NeedsYou{}, Recent: []store.Call{}, Demo: d.Demo}
	apis, _ := d.Store.APIs(ctx)
	h.APIs = len(apis)
	reqs, _ := d.Store.Requests(ctx, "")
	h.Requests = len(reqs)
	tks, _ := d.Store.Toolkits(ctx)
	h.Toolkits = len(tks)
	tools, _ := d.Store.Tools(ctx)
	h.Tools = len(tools)
	agents, _ := d.Store.Agents(ctx)
	for _, a := range agents {
		if a.RevokedAt == nil {
			h.Agents++
			if d.connected(a) {
				h.ActiveAgents++
			}
		}
	}
	tkName := map[string]string{}
	for _, tk := range tks {
		tkName[tk.ID] = tk.Name
	}
	y, m, dd := d.Clock.Now().Date()
	calls, _ := d.Store.Calls(ctx, store.CallFilter{Since: time.Date(y, m, dd, 0, 0, 0, 0, time.UTC), Limit: 100000})
	for _, c := range calls {
		if c.ActorKind != "agent" {
			continue
		}
		if c.Status == "blocked" {
			h.BlockedToday++
		}
		if c.ParentID != "" || c.Kind == "list" {
			continue
		}
		h.CallsToday++
		h.TokensToday += c.Tokens
		if c.Status == "error" {
			h.ErrorsToday++
		}
	}
	pend, _ := d.Store.PendingVersions(ctx)
	for _, v := range pend {
		h.NeedsYou = append(h.NeedsYou, NeedsYou{Kind: "review", ID: v.ID, ToolID: v.ToolID, Title: v.Name + " v" + strconv.Itoa(v.N), Detail: firstLine(v.Description),
			Toolkit: tkName[v.ToolkitID], Agent: v.CreatedByName, At: v.CreatedAt.Format(time.RFC3339)})
	}
	open, _ := d.Store.ToolRequests(ctx, "", "open")
	for _, r := range open {
		h.NeedsYou = append(h.NeedsYou, NeedsYou{Kind: "request", ID: r.ID, Title: "Tool request", Detail: r.Description, Toolkit: tkName[r.ToolkitID], Agent: r.AgentName, At: r.CreatedAt.Format(time.RFC3339)})
	}
	blocked, _ := d.Store.Calls(ctx, store.CallFilter{Status: "blocked", Since: d.Clock.Now().Add(-7 * 24 * time.Hour), Limit: 20})
	seen := map[string]bool{}
	for _, c := range blocked {
		if c.ActorKind != "agent" || seen[c.ActorID+c.Name] {
			continue
		}
		seen[c.ActorID+c.Name] = true
		h.NeedsYou = append(h.NeedsYou, NeedsYou{Kind: "blocked", ID: c.ID, Title: c.ActorName + " was blocked calling " + c.Name, Detail: c.Error,
			Toolkit: tkName[c.ToolkitID], Agent: c.ActorName, At: c.StartedAt.Format(time.RFC3339)})
	}
	h.Recent, _ = d.Store.Calls(ctx, store.CallFilter{TopLevel: true, Limit: 8})
	filtered := h.Recent[:0]
	for _, c := range h.Recent {
		if c.Kind != "list" {
			filtered = append(filtered, c)
		}
	}
	h.Recent = filtered
	if u, err := d.Usage(ctx, "7d"); err == nil {
		h.Daily = u.Daily
	}
	hasLive := false
	for _, t := range tools {
		if _, err := d.Store.LiveVersion(ctx, t.ID); err == nil {
			hasLive = true
		}
	}
	sent, _ := d.Store.Calls(ctx, store.CallFilter{Kind: "upstream", Limit: 1})
	hasExample := false
	for _, r := range reqs {
		if len(r.Example) > 0 && string(r.Example) != "null" {
			hasExample = true
			break
		}
	}
	// The journey a new user is guided through (UI-11). Steps can be done in
	// any order; Reason names what a step needs when it can't start yet.
	h.Steps = []Step{
		{Key: "api", Done: h.APIs > 0},
		{Key: "send", Done: len(sent) > 0},
		{Key: "example", Done: hasExample},
		{Key: "toolkit", Done: h.Toolkits > 0},
		{Key: "tool", Done: hasLive},
		{Key: "agent", Done: h.Agents > 0},
	}
	needs := map[string]string{"send": "Add an application first", "example": "Send a request first", "tool": "Create a toolkit first", "agent": "Create a toolkit first"}
	done := map[string]bool{}
	for _, st := range h.Steps {
		done[st.Key] = st.Done
	}
	prereq := map[string]string{"send": "api", "example": "send", "tool": "toolkit", "agent": "toolkit"}
	for i, st := range h.Steps {
		if p, ok := prereq[st.Key]; ok && !st.Done && !done[p] {
			h.Steps[i].Reason = needs[st.Key]
		}
	}
	h.HideOnboarding = d.Store.Setting(ctx, "hideOnboarding", "") == "true"
	h.WelcomeSeen = d.Store.Setting(ctx, "welcomeSeen", "") == "true"
	return h, nil
}

// Settings are the editable settings (RUN-7).
var settingKeys = map[string]bool{"theme": true, "toolTimeoutSeconds": true, "retentionBodiesDays": true, "retentionCallsDays": true, "hideOnboarding": true, "welcomeSeen": true}

// SettingsView is Settings with read-only facts.
type SettingsView struct {
	Values   map[string]string `json:"values"`
	Port     int               `json:"port"`
	DataDir  string            `json:"dataDir"`
	Keychain string            `json:"keychain"`
	KeyFile  string            `json:"keyFile,omitempty"`
	Demo     bool              `json:"demo"`
	Version  string            `json:"version"`
}

func (d *Deck) Settings(ctx context.Context, version string) (*SettingsView, error) {
	all, err := d.Store.Settings(ctx)
	if err != nil {
		return nil, err
	}
	vals := map[string]string{"theme": "system", "toolTimeoutSeconds": "10", "retentionBodiesDays": "30", "retentionCallsDays": "365", "hideOnboarding": "false", "welcomeSeen": "false"}
	for k, v := range all {
		if settingKeys[k] {
			vals[k] = v
		}
	}
	return &SettingsView{Values: vals, Port: d.Port, DataDir: d.DataDir, Keychain: d.Vault.Backend, KeyFile: d.Vault.KeyFile, Demo: d.Demo, Version: version}, nil
}

func (d *Deck) SetSettings(ctx context.Context, in map[string]string) error {
	for k, v := range in {
		if !settingKeys[k] {
			return badRequest("unknown setting %s", k)
		}
		if k == "toolTimeoutSeconds" || k == "retentionBodiesDays" || k == "retentionCallsDays" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 || n > 36500 {
				return badRequest("%s must be a positive number", k)
			}
		}
		if err := d.Store.SetSetting(ctx, k, v); err != nil {
			return err
		}
	}
	d.audit(ctx, audit.You, "settings.update", "", "")
	return nil
}

// Prune applies retention (SES-5).
func (d *Deck) Prune(ctx context.Context) error {
	bodies, _ := strconv.Atoi(d.Store.Setting(ctx, "retentionBodiesDays", "30"))
	rows, _ := strconv.Atoi(d.Store.Setting(ctx, "retentionCallsDays", "365"))
	now := d.Clock.Now()
	return d.Store.PruneCalls(ctx, now.Add(-time.Duration(bodies)*24*time.Hour), now.Add(-time.Duration(rows)*24*time.Hour))
}

// AuditLog returns the newest events.
func (d *Deck) AuditLog(ctx context.Context, limit int) ([]store.AuditEvent, error) {
	return d.Store.AuditEvents(ctx, false, limit)
}

// VerifyAudit checks the hash chain (AUD-2).
func (d *Deck) VerifyAudit(ctx context.Context) (audit.Result, error) { return d.Audit.Verify(ctx) }

// CloseRequest closes a tool request without a tool.
func (d *Deck) CloseRequest(ctx context.Context, id string) error {
	r, err := d.Store.ToolRequest(ctx, id)
	if err != nil {
		return notFound("tool request")
	}
	r.State = "closed"
	_ = d.Store.PutToolRequest(ctx, r)
	d.audit(ctx, audit.You, "tool_request.close", r.ID, "")
	return nil
}
