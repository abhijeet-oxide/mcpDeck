package deck

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/tokens"
	"github.com/abhijeet-oxide/mcpdeck/internal/tool"
)

// ToolDef is an MCP tool definition.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	ReadOnly    bool            `json:"-"`
	Destructive bool            `json:"-"`
	Idempotent  bool            `json:"-"`
	Title       string          `json:"-"`
}

// listLimit and listTop implement ACC-3.
const (
	listLimit = 25
	listTop   = 15
)

// MCPToolDefs are a toolkit's live tools as a grant sees them (MCP-2, ACC-2, ACC-3).
func (d *Deck) MCPToolDefs(ctx context.Context, tk store.Toolkit, access string) []ToolDef {
	defs, _ := d.mcpToolDefs(ctx, tk, access)
	return defs
}

// Truncated reports whether the toolkit lists only its most used tools.
func (d *Deck) Truncated(ctx context.Context, tk store.Toolkit, access string) bool {
	_, t := d.mcpToolDefs(ctx, tk, access)
	return t
}

func (d *Deck) mcpToolDefs(ctx context.Context, tk store.Toolkit, access string) ([]ToolDef, bool) {
	tools, _ := d.Store.ToolkitTools(ctx, tk.ID)
	type ranked struct {
		def   ToolDef
		calls int
	}
	var all []ranked
	since := d.Clock.Now().Add(-30 * 24 * time.Hour)
	for _, t := range tools {
		v, err := d.Store.LiveVersion(ctx, t.ID)
		if err != nil {
			continue
		}
		if access == "read" && v.Access == "write" {
			continue
		}
		desc := v.Description
		if v.UseWhen != "" {
			desc += "\nUse when: " + v.UseWhen
		}
		if v.DontUseWhen != "" {
			desc += "\nDon't use when: " + v.DontUseWhen
		}
		n := 0
		if len(tools) > listLimit {
			cs, _ := d.Store.Calls(ctx, store.CallFilter{Kind: "tool", Name: t.Name, Since: since, Limit: 10000})
			n = len(cs)
		}
		def := ToolDef{Name: v.Name, Description: desc, InputSchema: v.InputSchema, ReadOnly: v.Access == "read", Destructive: v.Access == "write"}
		// Declared hints may narrow, never widen, what mcpDeck derived (LINT-1).
		if a := d.annotations(v); a != nil {
			def.Title = a.Title
			def.Idempotent = a.IdempotentHint != nil && *a.IdempotentHint
			if def.Destructive && a.DestructiveHint != nil && !*a.DestructiveHint && !hasDelete(d.callMethods(ctx, v.Calls)) {
				def.Destructive = false
			}
		}
		all = append(all, ranked{def: def, calls: n})
	}
	truncated := len(all) > listLimit
	if truncated {
		sort.SliceStable(all, func(i, j int) bool { return all[i].calls > all[j].calls })
		all = all[:listTop]
	}
	out := make([]ToolDef, 0, len(all))
	for _, r := range all {
		out = append(out, r.def)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, truncated
}

func schema(s string) json.RawMessage { return json.RawMessage(s) }

// useExtras are on every use endpoint (MCP-3: request_tool, report_field_usage).
// use_tool is only there when the list is cut to the most used tools (ACC-3).
func (d *Deck) useExtras(truncated bool) []ToolDef {
	out := []ToolDef{
		{Name: "search_tools", ReadOnly: true, Description: "Find tools in this toolkit by intent when none of the listed ones fits, including ones not listed. One line each.",
			InputSchema: schema(`{"type":"object","properties":{"query":{"type":"string","description":"what you need, in plain words"}},"required":["query"]}`)},
		{Name: "use_tool", Description: "Call a tool found with search_tools that isn't in the tool list.",
			InputSchema: schema(`{"type":"object","properties":{"name":{"type":"string"},"input":{"type":"object"}},"required":["name"]}`)},
		{Name: "request_tool", Description: "Ask for a tool you need but don't have (missing API, needs write access, or 3+ calls for one answer). A person or a building agent makes it.",
			InputSchema: schema(`{"type":"object","properties":{"description":{"type":"string","description":"what the tool should return and its inputs"},"tried":{"type":"string","description":"tools you tried and why they didn't fit"}},"required":["description"]}`)},
		{Name: "report_field_usage", ReadOnly: true, Description: "After using a tool's output, say which fields you used and which you didn't, so the tool can be made smaller. Changes nothing by itself.",
			InputSchema: schema(`{"type":"object","properties":{"tool":{"type":"string"},"used":{"type":"array","items":{"type":"string"},"description":"paths like issues[].key"},"unused":{"type":"array","items":{"type":"string"}},"note":{"type":"string"}},"required":["tool"]}`)},
	}
	if !truncated {
		var kept []ToolDef
		for _, t := range out {
			if t.Name != "use_tool" {
				kept = append(kept, t)
			}
		}
		return kept
	}
	return out
}

// BuildToolDefs are on the builder endpoint (MCP-3).
func BuildToolDefs() []ToolDef {
	return []ToolDef{
		{Name: "search_tools", ReadOnly: true, Description: "Find existing tools in this toolkit before building one.",
			InputSchema: schema(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)},
		{Name: "list_tool_requests", ReadOnly: true, Description: "Open tool requests in this toolkit. Pass a request's id as fulfils when saving the tool for it.",
			InputSchema: schema(`{"type":"object","properties":{}}`)},
		{Name: "search_apis", ReadOnly: true, Description: "Find API requests by intent in this toolkit's build list. One line each: crm.getAccount({ id }) → GetAccountResponse · GET /accounts/{id} · read",
			InputSchema: schema(`{"type":"object","properties":{"query":{"type":"string","description":"intent, e.g. overdue invoices; empty lists everything"},"api":{"type":"string"}}}`)},
		{Name: "describe_api", ReadOnly: true, Description: "TypeScript signature, response type and a masked example for one request, e.g. crm.getAccount.",
			InputSchema: schema(`{"type":"object","properties":{"request":{"type":"string","description":"api.request, e.g. crm.getAccount"},"detail":{"type":"boolean"}},"required":["request"]}`)},
		{Name: "run_api", Description: "Send one request through mcpDeck and see a compact response (long lists trimmed with a count).",
			InputSchema: schema(`{"type":"object","properties":{"request":{"type":"string","description":"api.request"},"args":{"type":"object","description":"flat args, or { path, query, headers, body }"},"detail":{"type":"boolean"}},"required":["request"]}`)},
		{Name: "run_script", Description: "Run TypeScript in the tool sandbox with the API clients (crm.getAccount(...), call(), vars). Statements with a return, or a full tool file.",
			InputSchema: schema(`{"type":"object","properties":{"code":{"type":"string"},"input":{"type":"object"},"detail":{"type":"boolean"}},"required":["code"]}`)},
		{Name: "test_tool", Description: "Run a tool file with an input. examples: true answers from saved examples without network.",
			InputSchema: schema(`{"type":"object","properties":{"source":{"type":"string"},"input":{"type":"object"},"examples":{"type":"boolean"},"detail":{"type":"boolean"}},"required":["source"]}`)},
		{Name: "save_tool", Description: "Save a tool file as a new version with tests. Returns version, status (live or pending) and which approval rules passed.",
			InputSchema: schema(`{"type":"object","properties":{"source":{"type":"string"},"tests":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"},"input":{"type":"object"},"script":{"type":"string","description":"pm.test(\"name\", () => pm.expect(output.x).to.equal(1))"}},"required":["name","input"]}},"fulfils":{"type":"string","description":"tool request id"}},"required":["source","tests"]}`)},
	}
}

// UseInstructions is the initialize instructions text for a toolkit (TK-3).
// It tells the main agent to hand tool building to a subagent so API
// exploration never fills its own context.
func (d *Deck) UseInstructions(tk store.Toolkit, g store.Grant, client string) string {
	var b strings.Builder
	about := tk.Description
	if about == "" {
		about = "tools built from the company's APIs"
	}
	fmt.Fprintf(&b, "mcpDeck toolkit %q: %s. Credentials are handled by mcpDeck; never ask for keys.\n", tk.Name, about)
	b.WriteString("Pick a listed tool by its Use when / Don't use when lines. Nothing fits: search_tools.\n")
	if g.CanBuild {
		agent := "mcpdeck-" + tk.Name + "-builder"
		switch client {
		case "claude-code":
			fmt.Fprintf(&b, "Still nothing: don't explore APIs here. Delegate to the %s subagent (Agent tool) with one paragraph: the question, inputs, fields needed. It builds, tests and saves the tool, then replies with its name and status; call it when live.\n", agent)
		case "vscode":
			fmt.Fprintf(&b, "Still nothing: don't explore APIs here. Run the %s agent as a subagent (runSubagent) with one paragraph: the question, inputs, fields needed. It replies with the tool's name and status.\n", agent)
		default:
			b.WriteString("Still nothing: in a separate chat or subagent connected to this toolkit's /build endpoint, build the tool; here, call request_tool if you can't.\n")
		}
	} else {
		b.WriteString("Still nothing: call request_tool with what you need. Don't call APIs another way.\n")
	}
	b.WriteString("After a large response, call report_field_usage with the fields you used.")
	return b.String()
}

// BuildInstructions is the builder endpoint's loop in five lines (TK-3).
func BuildInstructions(tk store.Toolkit) string {
	return fmt.Sprintf(`mcpDeck builder for toolkit %q. Build one small tool, then stop and report back.
1. search_tools and list_tool_requests: reuse or fulfil, don't duplicate.
2. search_apis "<intent>" then describe_api api.request for signatures and examples.
3. run_api / run_script to try calls; return only fields the task needs, use Promise.all for independent calls.
4. test_tool, then save_tool with at least one pm.test test (and fulfils if a request exists).
5. Reply in two lines: tool name, status (live or pending and why), what it returns.`, tk.Name)
}

// AgentSession is an authenticated MCP caller.
type AgentSession struct {
	Agent     store.Agent
	Grant     store.Grant
	Toolkit   store.Toolkit
	SessionID string
	Mode      string // use | build
}

func (s AgentSession) caller(parent string) Caller {
	tk := s.Toolkit
	g := s.Grant
	return Caller{Actor: audit.Actor{Kind: "agent", ID: s.Agent.ID, Name: s.Agent.Name}, Grant: &g, Toolkit: &tk, SessionID: s.SessionID, ParentID: parent}
}

// AgentCall runs one MCP tools/call and records it (SES-2, SES-3).
func (d *Deck) AgentCall(ctx context.Context, s AgentSession, name string, raw json.RawMessage) (string, bool) {
	start := d.Clock.Now()
	callID := d.IDs.New()
	var args map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &args)
	}
	if args == nil {
		args = map[string]any{}
	}
	kind := "tool"
	var text string
	var isErr bool
	var versionID string
	switch {
	case s.Mode == "build":
		kind = "build"
		text, isErr = d.buildCall(ctx, s, callID, name, args)
	case name == "search_tools":
		kind = "build"
		text = d.searchTools(ctx, s.Toolkit, s.Grant.Access, str(args["query"]))
	case name == "request_tool":
		kind = "build"
		text, isErr = d.requestTool(ctx, s, str(args["description"]), str(args["tried"]))
	case name == "report_field_usage":
		kind = "build"
		text, isErr = d.reportFields(ctx, s, args)
	case name == "use_tool" && d.Truncated(ctx, s.Toolkit, s.Grant.Access):
		in, _ := json.Marshal(args["input"])
		text, isErr, versionID = d.runPublished(ctx, s, callID, str(args["name"]), in)
	default:
		text, isErr, versionID = d.runPublished(ctx, s, callID, name, raw)
	}
	tok := tokens.Count(text)
	if !isErr && kind == "tool" && s.Toolkit.Hints && s.Toolkit.HintTokens > 0 && tok > s.Toolkit.HintTokens && !strings.Contains(text, `"_mcpdeck":{`) {
		text += fmt.Sprintf("\n\nLarge response (%d tokens). If you used only some fields, call report_field_usage.", tok)
		tok = tokens.Count(text)
	}
	status := "ok"
	if isErr {
		status = "error"
		if strings.Contains(text, "write blocked") {
			status = "blocked"
		}
	}
	in := string(proxy.MaskJSON(raw))
	_ = d.Store.InsertCall(context.WithoutCancel(ctx), store.Call{ID: callID, SessionID: s.SessionID, ActorKind: "agent", ActorID: s.Agent.ID, ActorName: s.Agent.Name,
		ToolkitID: s.Toolkit.ID, Kind: kind, Name: name, Status: status, Error: map[bool]string{true: firstLine(text), false: ""}[isErr],
		StartedAt: start, DurationMS: d.Clock.Now().Sub(start).Milliseconds(), Input: in, Output: clip(text, 64<<10), OutputBytes: len(text), Tokens: tok, VersionID: versionID})
	return text, isErr
}

// RecordList records a tools/list and its standing token cost (SES-3).
func (d *Deck) RecordList(ctx context.Context, s AgentSession, payload []byte) {
	_ = d.Store.InsertCall(context.WithoutCancel(ctx), store.Call{ID: d.IDs.New(), SessionID: s.SessionID, ActorKind: "agent", ActorID: s.Agent.ID, ActorName: s.Agent.Name,
		ToolkitID: s.Toolkit.ID, Kind: "list", Name: "tools/list", Status: "ok", StartedAt: d.Clock.Now(), OutputBytes: len(payload), Tokens: tokens.Count(string(payload))})
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func (d *Deck) runPublished(ctx context.Context, s AgentSession, callID, name string, raw json.RawMessage) (string, bool, string) {
	t, err := d.Store.ToolByName(ctx, name)
	if err != nil {
		return fmt.Sprintf("no tool named %s in the %s toolkit; call search_tools to find one", name, s.Toolkit.Name), true, ""
	}
	ids, _ := d.Store.ToolToolkits(ctx, t.ID)
	in := false
	for _, id := range ids {
		if id == s.Toolkit.ID {
			in = true
		}
	}
	v, verr := d.Store.LiveVersion(ctx, t.ID)
	if !in || verr != nil {
		return fmt.Sprintf("%s isn't available in the %s toolkit; call search_tools", name, s.Toolkit.Name), true, ""
	}
	if s.Grant.Access == "read" && v.Access == "write" {
		return fmt.Sprintf("write blocked: %s writes and this agent has read access to the %s toolkit. Use a read tool, or ask the user to give this agent read+write in mcpDeck.", name, s.Toolkit.Name), true, v.ID
	}
	var input any = map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		_ = json.Unmarshal(raw, &input)
	}
	if err := tool.ValidateInput(v.InputSchema, input); err != nil {
		return err.Error(), true, v.ID
	}
	env := d.env(ctx, s.caller(callID), false)
	env.Clients = d.clients(ctx, nil) // published tools may use any API they were approved with
	r := tool.Run(ctx, v.Source, input, env)
	if r.Error != "" {
		msg := r.Error
		if r.ErrorCode == "" && !strings.Contains(msg, "mcpDeck") {
			msg = fmt.Sprintf("%s failed: %s. Check the input; if it keeps failing, tell the user the tool needs fixing in mcpDeck.", name, msg)
		}
		return msg, true, v.ID
	}
	out, _ := fitOutput(r.OutputJSON, OutputBudget)
	return out, false, v.ID
}

func (d *Deck) searchTools(ctx context.Context, tk store.Toolkit, access, q string) string {
	tools, _ := d.Store.ToolkitTools(ctx, tk.ID)
	words := strings.Fields(strings.ToLower(q))
	type hit struct {
		line  string
		score int
	}
	var hits []hit
	for _, t := range tools {
		v, err := d.Store.LiveVersion(ctx, t.ID)
		if err != nil || (access == "read" && v.Access == "write") {
			continue
		}
		hay := strings.ToLower(v.Name + " " + v.Description + " " + v.UseWhen + " " + strings.Join(v.Calls, " "))
		score := 0
		for _, w := range words {
			if strings.Contains(hay, w) {
				score++
			}
		}
		if len(words) == 0 || score > 0 {
			var props map[string]any
			var sch map[string]any
			_ = json.Unmarshal(v.InputSchema, &sch)
			props, _ = sch["properties"].(map[string]any)
			var in []string
			for k := range props {
				in = append(in, k)
			}
			sort.Strings(in)
			hits = append(hits, hit{fmt.Sprintf("%s({ %s }) · %s · %s", v.Name, strings.Join(in, ", "), v.Access, firstLine(v.Description)), score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) == 0 {
		return "No tool matches. If you can build, delegate to the builder; otherwise call request_tool."
	}
	var lines []string
	for i, h := range hits {
		if i == 20 {
			break
		}
		lines = append(lines, h.line)
	}
	return strings.Join(lines, "\n")
}

func (d *Deck) requestTool(ctx context.Context, s AgentSession, desc, tried string) (string, bool) {
	if strings.TrimSpace(desc) == "" {
		return "description is empty: say what the tool should return and its inputs", true
	}
	r := store.ToolRequest{ID: d.IDs.New(), ToolkitID: s.Toolkit.ID, AgentID: s.Agent.ID, AgentName: s.Agent.Name, SessionID: s.SessionID,
		Description: clip(desc, 2000), Tried: clip(tried, 2000), State: "open", CreatedAt: d.Clock.Now()}
	_ = d.Store.PutToolRequest(ctx, r)
	d.audit(ctx, audit.Actor{Kind: "agent", ID: s.Agent.ID, Name: s.Agent.Name}, "tool_request.create", s.Toolkit.Name, clip(desc, 200))
	return fmt.Sprintf("Request %s created in %s. A person sees it on mcpDeck's Home; tell the user you asked for it.", r.ID, s.Toolkit.Name), false
}

func strs(v any) []string {
	arr, _ := v.([]any)
	out := []string{}
	for _, x := range arr {
		if s, ok := x.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (d *Deck) reportFields(ctx context.Context, s AgentSession, args map[string]any) (string, bool) {
	t, err := d.Store.ToolByName(ctx, str(args["tool"]))
	if err != nil {
		return "no tool named " + str(args["tool"]), true
	}
	v, _ := d.Store.LiveVersion(ctx, t.ID)
	r := store.FieldReport{ID: d.IDs.New(), ToolID: t.ID, Version: v.N, AgentID: s.Agent.ID, AgentName: s.Agent.Name, SessionID: s.SessionID,
		Used: strs(args["used"]), Unused: strs(args["unused"]), Note: clip(str(args["note"]), 500), CreatedAt: d.Clock.Now()}
	_ = d.Store.PutFieldReport(ctx, r)
	return "Thanks, recorded. Nothing changes until a person approves.", false
}

// findRequest resolves "api.request" inside a toolkit's build scope (TK-4).
func (d *Deck) findRequest(ctx context.Context, tk store.Toolkit, ref string) (store.API, store.Request, error) {
	apiName, reqName, ok := strings.Cut(strings.TrimSpace(ref), ".")
	if !ok {
		return store.API{}, store.Request{}, fmt.Errorf("name a request as api.request, e.g. crm.getAccount; search_apis lists them")
	}
	a, err := d.Store.APIByName(ctx, apiName)
	if err != nil {
		return a, store.Request{}, fmt.Errorf("no API named %s; search_apis lists what this toolkit can reach", apiName)
	}
	inScope := false
	for _, id := range d.scopeIDs(ctx, tk) {
		if id == a.ID {
			inScope = true
		}
	}
	if !inScope {
		return a, store.Request{}, fmt.Errorf("out of build scope: %s isn't in the %s toolkit's build list. Ask the user to add it in mcpDeck (toolkit Settings › Build list), or call request_tool", apiName, tk.Name)
	}
	reqs, _ := d.Store.Requests(ctx, a.ID)
	for _, r := range reqs {
		if r.Name == reqName {
			return a, r, nil
		}
	}
	return a, store.Request{}, fmt.Errorf("%s has no request named %s; search_apis lists them", apiName, reqName)
}

func (d *Deck) buildCall(ctx context.Context, s AgentSession, callID, name string, args map[string]any) (string, bool) {
	detail, _ := args["detail"].(bool)
	switch name {
	case "search_tools":
		return d.searchTools(ctx, s.Toolkit, "read_write", str(args["query"])), false
	case "list_tool_requests":
		reqs, _ := d.Store.ToolRequests(ctx, s.Toolkit.ID, "open")
		if len(reqs) == 0 {
			return "No open tool requests.", false
		}
		var lines []string
		for _, r := range reqs {
			lines = append(lines, fmt.Sprintf("%s · from %s · %s", r.ID, r.AgentName, firstLine(r.Description)))
		}
		return strings.Join(lines, "\n"), false
	case "search_apis":
		return d.searchAPIs(ctx, s.Toolkit, str(args["query"]), str(args["api"])), false
	case "describe_api":
		a, r, err := d.findRequest(ctx, s.Toolkit, str(args["request"]))
		if err != nil {
			return err.Error(), true
		}
		out := tool.Describe(a, r)
		if len(r.Example) > 0 {
			var v any
			_ = json.Unmarshal(r.Example, &v)
			out += "\nExample (masked):\n" + Compact(v, detail)
		} else {
			out += "\nNo saved example; use run_api to see a real response."
		}
		return out, false
	case "run_api":
		a, r, err := d.findRequest(ctx, s.Toolkit, str(args["request"]))
		if err != nil {
			return err.Error(), true
		}
		c := s.caller(callID).proxyBase()
		c.APIID, c.RequestID = a.ID, r.ID
		if m, ok := args["args"].(map[string]any); ok {
			c.Args = toArgs(m)
		}
		res, err := d.Proxy.Do(ctx, c)
		if err != nil {
			return err.Error(), true
		}
		head := fmt.Sprintf("%s · %d bytes · %d tokens · %d ms\n", res.StatusText, res.Size, res.Tokens, res.DurationMS)
		return head + Compact(res.JSON(), detail), res.Status >= 400
	case "run_script":
		r := d.RunScript(ctx, s.caller(callID), str(args["code"]), args["input"])
		return runSummary(r, detail), r.Error != ""
	case "test_tool":
		r, _ := d.TestTool(ctx, s.caller(callID), TestInput{Source: str(args["source"]), Input: args["input"], Examples: args["examples"] == true})
		return runSummary(r, detail), r.Error != ""
	case "save_tool":
		var tests []store.ToolTest
		if arr, ok := args["tests"].([]any); ok {
			for _, x := range arr {
				m, _ := x.(map[string]any)
				in, _ := json.Marshal(m["input"])
				if string(in) == "null" {
					in = []byte("{}")
				}
				tests = append(tests, store.ToolTest{Name: str(m["name"]), Input: in, Script: str(m["script"])})
			}
		}
		res, err := d.SaveTool(ctx, s.Toolkit.ID, s.caller(callID), SaveInput{Source: str(args["source"]), Tests: tests, Fulfils: str(args["fulfils"])})
		if err != nil {
			return err.Error(), true
		}
		var lines []string
		lines = append(lines, res.Message)
		for _, c := range res.Checks {
			mark := "pass"
			if !c.Passed {
				mark = "FAIL"
			}
			line := mark + " · " + c.Label
			if c.Detail != "" {
				line += " · " + c.Detail
			}
			lines = append(lines, line)
		}
		for _, w := range res.Warnings {
			lines = append(lines, "warning · "+w)
		}
		return strings.Join(lines, "\n"), false
	}
	return fmt.Sprintf("unknown tool %s on the builder endpoint", name), true
}

func toArgs(m map[string]any) proxy.Args {
	var a proxy.Args
	structured := len(m) > 0
	for k := range m {
		if k != "path" && k != "query" && k != "headers" && k != "body" {
			structured = false
		}
	}
	if !structured {
		a.Flat = m
		return a
	}
	a.Path, _ = m["path"].(map[string]any)
	a.Query, _ = m["query"].(map[string]any)
	if h, ok := m["headers"].(map[string]any); ok {
		a.Headers = map[string]string{}
		for k, v := range h {
			a.Headers[k] = fmt.Sprint(v)
		}
	}
	if b, ok := m["body"]; ok {
		a.Body, a.HasBody = b, true
	}
	return a
}

func runSummary(r tool.Result, detail bool) string {
	var b strings.Builder
	if r.Error != "" {
		b.WriteString("error: " + r.Error + "\n")
	} else {
		fmt.Fprintf(&b, "output · %d bytes · %d tokens (API responses: %d bytes, %d tokens) · %d ms\n", r.OutputBytes, r.OutputTokens, r.UpstreamBytes, r.UpstreamTokens, r.DurationMS)
		b.WriteString(Compact(r.Output, detail) + "\n")
	}
	for _, c := range r.Calls {
		line := fmt.Sprintf("call %s · %d · %d ms · %d tokens", c.Name, c.Status, c.DurationMS, c.Tokens)
		if c.Error != "" {
			line = fmt.Sprintf("call %s · error: %s", c.Name, c.Error)
		}
		b.WriteString(line + "\n")
	}
	for i, l := range r.Console {
		if i == 20 {
			break
		}
		b.WriteString("console: " + clip(l, 300) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (d *Deck) searchAPIs(ctx context.Context, tk store.Toolkit, q, apiFilter string) string {
	words := strings.Fields(strings.ToLower(q))
	type hit struct {
		line  string
		score int
	}
	var hits []hit
	for _, id := range d.scopeIDs(ctx, tk) {
		a, err := d.Store.API(ctx, id)
		if err != nil || (apiFilter != "" && a.Name != apiFilter) {
			continue
		}
		reqs, _ := d.Store.Requests(ctx, a.ID)
		for _, r := range reqs {
			hay := strings.ToLower(a.Name + " " + a.Title + " " + r.Name + " " + r.Title + " " + r.URL + " " + r.Folder + " " + r.Description)
			score := 0
			for _, w := range words {
				if strings.Contains(hay, w) {
					score++
				}
			}
			if len(words) == 0 || score > 0 {
				hits = append(hits, hit{tool.Signature(a, r), score})
			}
		}
	}
	if len(hits) == 0 {
		if len(d.scopeIDs(ctx, tk)) == 0 {
			return fmt.Sprintf("The %s toolkit has no APIs in its build list. Ask the user to add some in mcpDeck (toolkit Settings › Build list).", tk.Name)
		}
		return "No request matches; try other words or an empty query to list everything."
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	var lines []string
	for i, h := range hits {
		if i == 30 {
			lines = append(lines, fmt.Sprintf("… %d more; narrow the query", len(hits)-30))
			break
		}
		lines = append(lines, h.line)
	}
	return strings.Join(lines, "\n")
}

// Compact renders JSON for agents: short lists and strings unless detail.
func Compact(v any, detail bool) string {
	if !detail {
		v = trim(v, 0)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func trim(v any, depth int) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			out[k] = trim(x, depth+1)
		}
		return out
	case []any:
		if len(t) > 3 {
			out := make([]any, 0, 4)
			for _, x := range t[:3] {
				out = append(out, trim(x, depth+1))
			}
			return append(out, fmt.Sprintf("… %d more items (detail: true for all)", len(t)-3))
		}
		out := make([]any, 0, len(t))
		for _, x := range t {
			out = append(out, trim(x, depth+1))
		}
		return out
	case string:
		if utf8.RuneCountInString(t) > 240 {
			r := []rune(t)
			return string(r[:240]) + "…"
		}
	}
	return v
}

// UseExtrasFor are the extra tools on the use endpoint.
func (d *Deck) UseExtrasFor(truncated bool) []ToolDef { return d.useExtras(truncated) }

func hasDelete(ms []string) bool {
	for _, m := range ms {
		if m == "DELETE" {
			return true
		}
	}
	return false
}
