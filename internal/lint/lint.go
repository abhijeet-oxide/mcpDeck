// Package lint checks toolkits, tools and outputs against what makes an MCP
// server good for agents (GUIDE-1…4). Rules are deterministic; no model runs.
//
// Two levels, as the Guidelines page explains them:
//
//	must   breaks agents or clients; fix before connecting agents
//	avoid  works, but costs tokens or makes agents pick wrong
//	tip    an idea that usually helps
package lint

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/abhijeet-oxide/mcpdeck/internal/tokens"
)

// Levels.
const (
	Must  = "must"
	Avoid = "avoid"
	Tip   = "tip"
)

// Thresholds. Sources are listed in docs/mcp-guidelines.md.
const (
	// Cursor sends at most 40 tools; models choose worse well before that.
	MaxTools  = 40
	WarnTools = 15
	// Tokens every conversation pays for the tools/list of one toolkit.
	MaxListTokens  = 10000
	WarnListTokens = 4000
	// Claude Code truncates tool results over 25,000 tokens.
	MaxOutputTokens  = 25000
	WarnOutputTokens = 2000
	WarnFields       = 25
	WarnArrayItems   = 50
	WarnDepth        = 4
	WarnLongString   = 2000
	MaxDescription   = 1024
	MinDescription   = 25
	WarnParams       = 8
)

// Finding is one problem and how to fix it.
type Finding struct {
	Rule   string `json:"rule"`
	Level  string `json:"level"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
	Tool   string `json:"tool,omitempty"`
	Path   string `json:"path,omitempty"`
}

// Tool is what the linter needs to know about one tool.
type Tool struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	UseWhen      string          `json:"useWhen"`
	DontUseWhen  string          `json:"dontUseWhen"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	Source       string          `json:"source"`
	Calls        []string        `json:"calls"`
	Access       string          `json:"access"`
	OutputTokens int             `json:"outputTokens"` // largest seen in tests or calls, 0 if unknown
}

// Report is the result for a toolkit.
type Report struct {
	Tools      int       `json:"tools"`
	ListTokens int       `json:"listTokens"`
	Score      int       `json:"score"` // 0–100
	Findings   []Finding `json:"findings"`
}

var generic = map[string]bool{"new_tool": true, "tool": true, "test": true, "my_tool": true, "get_data": true, "run": true, "call_api": true, "do_it": true, "helper": true, "api": true, "request": true}

var vagueParams = map[string]string{"id": "account_id", "name": "customer_name", "q": "query", "data": "a named field", "value": "a named field", "type": "a specific name like report_type", "obj": "a named field", "input": "a named field"}

var rawReturn = regexp.MustCompile(`(?m)^\s*return\s+(res|response|result|data|r|out)\s*;?\s*$`)

// ListTokens is what one tool definition costs in tools/list.
func ListTokens(t Tool) int {
	def := map[string]any{"name": t.Name, "description": describe(t), "inputSchema": json.RawMessage(orEmpty(t.InputSchema))}
	b, _ := json.Marshal(def)
	return tokens.Count(string(b))
}

func orEmpty(b json.RawMessage) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage(`{"type":"object"}`)
	}
	return b
}

func describe(t Tool) string {
	d := t.Description
	if t.UseWhen != "" {
		d += " Use when: " + t.UseWhen + "."
	}
	if t.DontUseWhen != "" {
		d += " Don't use when: " + t.DontUseWhen + "."
	}
	return d
}

// Toolkit lints a whole toolkit.
func Toolkit(tools []Tool) Report {
	r := Report{Tools: len(tools), Findings: []Finding{}}
	for _, t := range tools {
		r.ListTokens += ListTokens(t)
		r.Findings = append(r.Findings, ToolDef(t, len(tools))...)
	}
	switch {
	case len(tools) > MaxTools:
		r.Findings = append(r.Findings, Finding{Rule: "tool-count", Level: Must,
			Title:  fmt.Sprintf("%d tools in one toolkit", len(tools)),
			Detail: fmt.Sprintf("Some clients only send the first %d tools, and agents pick the wrong tool more often as the list grows.", MaxTools),
			Fix:    "Split it into toolkits by task (support, billing…), or merge tools that are steps of one job."})
	case len(tools) > WarnTools:
		r.Findings = append(r.Findings, Finding{Rule: "tool-count", Level: Avoid,
			Title:  fmt.Sprintf("%d tools in one toolkit", len(tools)),
			Detail: "Every tool is read on every turn. Past about 15, similar tools start to blur together.",
			Fix:    "Keep a toolkit to one job. Merge tools that are always called together."})
	}
	switch {
	case r.ListTokens > MaxListTokens:
		r.Findings = append(r.Findings, Finding{Rule: "list-tokens", Level: Must,
			Title:  fmt.Sprintf("tools/list costs %s tokens", fmtNum(r.ListTokens)),
			Detail: "Agents pay this in every conversation before doing any work.",
			Fix:    "Shorten descriptions, drop unused tools, or split the toolkit."})
	case r.ListTokens > WarnListTokens:
		r.Findings = append(r.Findings, Finding{Rule: "list-tokens", Level: Avoid,
			Title:  fmt.Sprintf("tools/list costs %s tokens", fmtNum(r.ListTokens)),
			Detail: "This is loaded into every conversation.",
			Fix:    "Trim long descriptions and schemas; keep examples short."})
	}
	// Endpoint wrappers: most tools call exactly one request.
	single := 0
	for _, t := range tools {
		if len(t.Calls) == 1 {
			single++
		}
	}
	if len(tools) >= 5 && single*10 >= len(tools)*8 {
		r.Findings = append(r.Findings, Finding{Rule: "endpoint-wrappers", Level: Avoid,
			Title:  fmt.Sprintf("%d of %d tools wrap a single request", single, len(tools)),
			Detail: "An agent then makes several calls and copies IDs between them, paying for every response.",
			Fix:    "Build tools around tasks: chain the requests (login → list → details) in one tool and return the answer."})
	}
	// Overlapping tools: two tools calling the same set of requests.
	seen := map[string]string{}
	for _, t := range tools {
		if len(t.Calls) == 0 {
			continue
		}
		c := append([]string{}, t.Calls...)
		sort.Strings(c)
		k := strings.Join(c, ",")
		if o, ok := seen[k]; ok {
			r.Findings = append(r.Findings, Finding{Rule: "overlap", Level: Avoid, Tool: t.Name,
				Title:  fmt.Sprintf("%s and %s call the same requests", o, t.Name),
				Detail: "Agents can't tell which to pick, and both cost tokens in tools/list.",
				Fix:    "Merge them, with an input that selects the variant."})
		} else {
			seen[k] = t.Name
		}
	}
	r.Score = score(r.Findings)
	return r
}

func score(fs []Finding) int {
	s := 100
	for _, f := range fs {
		switch f.Level {
		case Must:
			s -= 15
		case Avoid:
			s -= 5
		}
	}
	if s < 0 {
		s = 0
	}
	return s
}

// ToolDef lints one tool's definition and code. n is the toolkit size.
func ToolDef(t Tool, n int) []Finding {
	var out []Finding
	add := func(rule, level, title, detail, fix string) {
		out = append(out, Finding{Rule: rule, Level: level, Tool: t.Name, Title: title, Detail: detail, Fix: fix})
	}
	if generic[t.Name] {
		add("name", Must, "Generic name "+t.Name, "Agents choose tools by name first. A generic name says nothing about what it returns.",
			"Name it after the job, verb first: find_vulnerabilities, get_customer_summary.")
	} else if !strings.Contains(t.Name, "_") {
		add("name", Tip, "One-word name "+t.Name, "Verb + object names (list_images, get_account) are easier for agents to choose.", "Rename it to verb_object.")
	}
	desc := strings.TrimSpace(t.Description)
	switch {
	case desc == "":
		add("description", Must, "No description", "The description is how an agent decides to call the tool.", "Say what it returns, in one sentence.")
	case strings.HasPrefix(desc, "What it returns") || strings.HasPrefix(desc, "Returns the fields of "):
		add("description", Must, "Starter description left in", "The description still has the starter text, which doesn't say what this tool is for.", "Write what it returns and for which question.")
	case len(desc) < MinDescription:
		add("description", Avoid, "Very short description", "A few words rarely tell an agent when to use this tool rather than another.", "Say what it returns and what it needs, in one or two sentences.")
	case len(describe(t)) > MaxDescription:
		add("description", Avoid, fmt.Sprintf("Description is %d characters", len(describe(t))), "Long descriptions are paid on every turn by every agent.", "Keep it under 1,000 characters; move detail into parameter descriptions.")
	}
	if n > 3 && t.UseWhen == "" {
		add("use-when", Tip, "No “use when”", "With several tools, a short “use when” stops agents mixing them up.", "Add useWhen: the question this tool answers.")
	}
	if t.Access == "write" && t.DontUseWhen == "" {
		add("write-guard", Avoid, "Write tool without “don't use when”", "Write tools change data. Agents should know when not to call them.", "Add dontUseWhen, for example: “just looking something up”.")
	}
	// Input schema.
	var s struct {
		Properties map[string]map[string]any `json:"properties"`
		Required   []string                  `json:"required"`
	}
	_ = json.Unmarshal(orEmpty(t.InputSchema), &s)
	if len(s.Properties) > WarnParams {
		add("params", Avoid, fmt.Sprintf("%d input parameters", len(s.Properties)), "Agents fill many parameters badly.", "Bind fixed values as toolkit variables, and keep inputs to what changes per question.")
	}
	var undocumented []string
	for k, p := range s.Properties {
		if better, ok := vagueParams[k]; ok {
			out = append(out, Finding{Rule: "param-name", Level: Avoid, Tool: t.Name, Path: k,
				Title:  "Vague parameter name " + k,
				Detail: "An agent guesses what “" + k + "” refers to, and guesses wrong when it holds several IDs.",
				Fix:    "Use a specific name, like " + better + "."})
		}
		if d, _ := p["description"].(string); d == "" && p["enum"] == nil {
			undocumented = append(undocumented, k)
		}
	}
	if len(undocumented) > 0 {
		sort.Strings(undocumented)
		add("param-docs", Tip, "Parameters without a description: "+strings.Join(undocumented, ", "),
			"A one-line description with an example value cuts wrong inputs.", "Add description (and examples) to each parameter.")
	}
	if len(s.Properties) > 0 && len(s.Required) == 0 {
		add("required", Avoid, "No required parameters", "Agents call it with nothing and get an error or everything.", "List the parameters the tool can't work without in required.")
	}
	// Code.
	if rawReturn.MatchString(t.Source) && !strings.Contains(t.Source, "pick(") {
		add("raw-return", Avoid, "Returns a whole API response", "Every field of the upstream response is paid for in tokens on every call, most of it unused.",
			"Return only the fields the task needs: use pick(res, [...]) or pick fields in the flow builder's Output step.")
	}
	if t.OutputTokens > 0 {
		out = append(out, outputTokenFindings(t.Name, t.OutputTokens)...)
	}
	return out
}

func outputTokenFindings(tool string, n int) []Finding {
	switch {
	case n > MaxOutputTokens:
		return []Finding{{Rule: "output-tokens", Level: Must, Tool: tool, Title: fmt.Sprintf("Output is %s tokens", fmtNum(n)),
			Detail: "Claude Code cuts tool results over 25,000 tokens, so the agent sees a partial answer.",
			Fix:    "Filter and page: add a limit input, return only needed fields."}}
	case n > WarnOutputTokens:
		return []Finding{{Rule: "output-tokens", Level: Avoid, Tool: tool, Title: fmt.Sprintf("Output is %s tokens", fmtNum(n)),
			Detail: "Large results fill the agent's context and slow every later step.",
			Fix:    "Aim for under 2,000 tokens: drop fields, summarise lists, add a limit."}}
	}
	return nil
}

var (
	sensitive = regexp.MustCompile(`(?i)(password|passwd|token|secret|authorization|api[-_]?key|cookie)`)
	uuidRe    = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	b64Re     = regexp.MustCompile(`^[A-Za-z0-9+/=_-]{200,}$`)
)

// FieldStat is the size of one field path in an output.
type FieldStat struct {
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	Tokens int    `json:"tokens"`
	Type   string `json:"type"`
	Empty  int    `json:"empty"`
	Count  int    `json:"count"`
}

// OutputReport analyses one response or tool output.
type OutputReport struct {
	Tokens   int         `json:"tokens"`
	Bytes    int         `json:"bytes"`
	Fields   int         `json:"fields"`
	Depth    int         `json:"depth"`
	Largest  []FieldStat `json:"largest"`
	Findings []Finding   `json:"findings"`
}

// Output analyses a value an agent would receive (GUIDE-3).
func Output(v any) OutputReport {
	b, _ := json.Marshal(v)
	rep := OutputReport{Bytes: len(b), Tokens: tokens.Count(string(b)), Findings: []Finding{}, Largest: []FieldStat{}}
	stats := map[string]*FieldStat{}
	var maxArr int
	var maxArrPath string
	var longStrings, uuids, blobs, secrets []string
	var walk func(x any, path string, depth int)
	walk = func(x any, path string, depth int) {
		if depth > rep.Depth {
			rep.Depth = depth
		}
		switch t := x.(type) {
		case map[string]any:
			for k, c := range t {
				p := k
				if path != "" {
					p = path + "." + k
				}
				st := stats[p]
				if st == nil {
					st = &FieldStat{Path: p}
					stats[p] = st
				}
				cb, _ := json.Marshal(c)
				st.Bytes += len(cb)
				st.Count++
				st.Type = kind(c)
				if _, obj := c.(map[string]any); !obj && sensitive.MatchString(k) {
					secrets = appendOnce(secrets, p)
				}
				if isEmpty(c) {
					st.Empty++
				}
				walk(c, p, depth+1)
			}
		case []any:
			if len(t) > maxArr {
				maxArr, maxArrPath = len(t), path
			}
			for _, c := range t {
				walk(c, path+"[]", depth)
			}
		case string:
			switch {
			case b64Re.MatchString(t):
				blobs = appendOnce(blobs, path)
			case len(t) > WarnLongString:
				longStrings = appendOnce(longStrings, path)
			case uuidRe.MatchString(t):
				uuids = appendOnce(uuids, path)
			}
		}
	}
	walk(v, "", 0)
	leaves := 0
	var all []FieldStat
	empty := []string{}
	for _, s := range stats {
		if s.Type != "object" && s.Type != "array" {
			leaves++
		}
		if s.Count > 0 && s.Empty == s.Count && s.Type != "object" {
			empty = append(empty, s.Path)
		}
		s.Tokens = s.Bytes / 4
		all = append(all, *s)
	}
	rep.Fields = leaves
	sort.Slice(all, func(i, j int) bool {
		return all[i].Bytes > all[j].Bytes || all[i].Bytes == all[j].Bytes && all[i].Path < all[j].Path
	})
	for _, s := range all {
		if s.Type == "object" || s.Type == "array" {
			continue
		}
		rep.Largest = append(rep.Largest, s)
		if len(rep.Largest) == 8 {
			break
		}
	}
	rep.Findings = append(rep.Findings, outputTokenFindings("", rep.Tokens)...)
	if leaves > WarnFields {
		rep.Findings = append(rep.Findings, Finding{Rule: "field-count", Level: Avoid, Title: fmt.Sprintf("%d fields", leaves),
			Detail: "Agents rarely need more than a handful of fields to answer a question.",
			Fix:    "Untick fields in the Output step, or return pick(res, [...]) with the ones the task needs."})
	}
	if maxArr > WarnArrayItems {
		rep.Findings = append(rep.Findings, Finding{Rule: "unbounded-list", Level: Avoid, Path: maxArrPath,
			Title:  fmt.Sprintf("List of %d items at %s", maxArr, orDefault(maxArrPath, "the top level")),
			Detail: "Long lists cost tokens per item and grow with the data.",
			Fix:    "Add a limit input with a small default (10–25), filter by what the agent asked, or return counts."})
	}
	if rep.Depth > WarnDepth {
		rep.Findings = append(rep.Findings, Finding{Rule: "deep-nesting", Level: Tip, Title: fmt.Sprintf("Nested %d levels deep", rep.Depth),
			Detail: "Deep objects cost brackets and keys at every level.", Fix: "Flatten to the fields the agent uses: owner.name → ownerName."})
	}
	if len(empty) > 0 {
		sort.Strings(empty)
		rep.Findings = append(rep.Findings, Finding{Rule: "empty-fields", Level: Tip, Title: fmt.Sprintf("%d fields are always empty", len(empty)),
			Detail: "Empty: " + strings.Join(first(empty, 6), ", ") + ".", Fix: "Drop fields that are always null or empty."})
	}
	if len(secrets) > 0 {
		rep.Findings = append(rep.Findings, Finding{Rule: "credential-in-output", Level: Must, Path: secrets[0], Title: "Credentials in the output",
			Detail: "Fields named like credentials (" + strings.Join(first(secrets, 3), ", ") + ") would be sent to the agent and kept in its context.",
			Fix:    "Never return tokens or passwords. Use them inside the tool only, or set the application's auth so mcpDeck adds them."})
	}
	if len(blobs) > 0 {
		rep.Findings = append(rep.Findings, Finding{Rule: "blob", Level: Avoid, Path: blobs[0], Title: "Encoded data in the output",
			Detail: "Base64 or token-like strings at " + strings.Join(first(blobs, 3), ", ") + " are expensive and meaningless to an agent.",
			Fix:    "Leave them out, or return a short reference instead."})
	}
	if len(longStrings) > 0 {
		rep.Findings = append(rep.Findings, Finding{Rule: "long-text", Level: Tip, Path: longStrings[0], Title: "Long text fields",
			Detail: "Over 2,000 characters at " + strings.Join(first(longStrings, 3), ", ") + ".",
			Fix:    "Return a summary or the first few hundred characters, with a separate tool for the full text."})
	}
	if len(uuids) >= 3 {
		rep.Findings = append(rep.Findings, Finding{Rule: "opaque-ids", Level: Tip, Title: "Many opaque IDs",
			Detail: "UUIDs at " + strings.Join(first(uuids, 3), ", ") + " mean little to an agent and are easy to copy wrong.",
			Fix:    "Return names alongside IDs, and keep only IDs another tool needs."})
	}
	return rep
}

func kind(x any) string {
	switch x.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case float64, int:
		return "number"
	case bool:
		return "boolean"
	}
	return "null"
}

func isEmpty(x any) bool {
	switch t := x.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	}
	return false
}

func appendOnce(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

func first(s []string, n int) []string {
	if len(s) > n {
		return append(s[:n:n], "…")
	}
	return s
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func fmtNum(n int) string {
	s := fmt.Sprint(n)
	if n < 1000 {
		return s
	}
	return s[:len(s)-3] + "," + s[len(s)-3:]
}
