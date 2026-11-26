package lint

// Source is a published document a rule is based on.
type Source struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Rule documents one check. docs/lint-rules.md is generated from Rules
// (go run ./tools/lintdoc), and a test keeps the two in step.
type Rule struct {
	ID       string   `json:"id"`
	Severity string   `json:"severity"` // the highest severity the rule reports
	Section  string   `json:"section"`
	Title    string   `json:"title"`
	Why      string   `json:"why"`
	Check    string   `json:"check"`
	Fix      string   `json:"fix"`
	Sources  []string `json:"sources"`
}

// Sources the rules cite.
var Sources = []Source{
	{"mcp-tools", "MCP specification 2025-11-25: Tools", "https://modelcontextprotocol.io/specification/2025-11-25/server/tools"},
	{"mcp-schema", "MCP specification 2025-11-25: Schema reference (ToolAnnotations)", "https://modelcontextprotocol.io/specification/2025-11-25/schema"},
	{"mcp-security", "MCP specification 2025-11-25: Security best practices", "https://modelcontextprotocol.io/specification/2025-11-25/basic/security_best_practices"},
	{"anthropic-tools", "Anthropic: Writing effective tools for agents", "https://www.anthropic.com/engineering/writing-tools-for-agents"},
	{"anthropic-code", "Anthropic: Code execution with MCP", "https://www.anthropic.com/engineering/code-execution-with-mcp"},
	{"cloudflare-code-mode", "Cloudflare: Code Mode", "https://blog.cloudflare.com/code-mode/"},
	{"cwe-798", "CWE-798: Use of Hard-coded Credentials", "https://cwe.mitre.org/data/definitions/798.html"},
}

// Sections, in the order the docs show them.
var Sections = []string{
	"Valid and safe",
	"Tools for tasks, not endpoints",
	"Names and descriptions",
	"Inputs agents can fill",
	"Return only what the task needs",
}

// Rules is every check the linter runs.
var Rules = []Rule{
	{ID: "schema-valid", Severity: Block, Section: "Valid and safe", Title: "Input schema is valid",
		Why:     "The spec requires inputSchema to be a JSON Schema object. Clients reject or misread anything else.",
		Check:   "inputSchema is an object with type \"object\"; properties is an object; required lists existing properties; types and enums are well formed.",
		Fix:     "Use { type: \"object\", properties: {...}, required: [...] }.",
		Sources: []string{"mcp-tools"}},
	{ID: "secret-in-definition", Severity: Block, Section: "Valid and safe", Title: "No credentials in the tool",
		Why:     "A token in the description, schema or code is sent to every agent or stored in every version.",
		Check:   "Bearer tokens, AWS, GitHub, Slack and OpenAI-style keys, JWTs and private keys in the definition or code.",
		Fix:     "Store it as a secret variable on the application and use {{name}} or the application's auth.",
		Sources: []string{"cwe-798", "mcp-security"}},
	{ID: "secret-input", Severity: Block, Section: "Valid and safe", Title: "Agents don't pass credentials",
		Why:     "An input named password or api_key puts a credential in the agent's context and transcript.",
		Check:   "Input parameters named password, secret, api_key, access_token, client_secret, private_key and similar.",
		Fix:     "Set the application's auth (login steps, OAuth, API key) so mcpDeck adds credentials; agents never see them.",
		Sources: []string{"mcp-security"}},
	{ID: "credential-in-output", Severity: Block, Section: "Valid and safe", Title: "No credentials in the output",
		Why:     "Anything a tool returns stays in the agent's context and transcript.",
		Check:   "Output fields named like credentials (password, access_token, api_key, cookie…) with a value.",
		Fix:     "Use credentials inside the tool only, or set the application's auth.",
		Sources: []string{"mcp-tools", "mcp-security"}},
	{ID: "write-annotation", Severity: Block, Section: "Valid and safe", Title: "Write tools are annotated as writes",
		Why:     "Clients use readOnlyHint and destructiveHint to decide when to ask the user. A write that claims to be read-only skips that prompt.",
		Check:   "A tool that calls a write request declares readOnlyHint: true, or calls DELETE and declares destructiveHint: false.",
		Fix:     "Remove the hint; mcpDeck sets readOnlyHint and destructiveHint from the requests the tool calls.",
		Sources: []string{"mcp-schema", "mcp-tools"}},
	{ID: "name-format", Severity: Warn, Section: "Valid and safe", Title: "Tool name uses allowed characters",
		Why:     "The spec says names SHOULD be 1 to 128 characters of letters, digits, _, - and .",
		Check:   "Name length and characters.",
		Fix:     "Rename to snake_case, for example list_findings.",
		Sources: []string{"mcp-tools"}},
	{ID: "endpoint-wrappers", Severity: Warn, Section: "Tools for tasks, not endpoints", Title: "Tools wrap tasks, not single endpoints",
		Why:     "An agent that chains login, list and details itself pays for every response and can copy the wrong ID.",
		Check:   "80% or more of a toolkit's tools (5 or more) call one request each.",
		Fix:     "Chain the requests in one tool and return the answer.",
		Sources: []string{"anthropic-tools"}},
	{ID: "overlap", Severity: Warn, Section: "Tools for tasks, not endpoints", Title: "No overlapping tools",
		Why:     "Agents can't tell overlapping tools apart, and both cost tokens.",
		Check:   "Two tools call the same set of requests, or have near-identical descriptions.",
		Fix:     "Merge them, with an input that selects the variant.",
		Sources: []string{"anthropic-tools"}},
	{ID: "catch-all", Severity: Warn, Section: "Tools for tasks, not endpoints", Title: "No catch-all request tool",
		Why:     "A tool that takes a method, path or URL lets an agent call anything, and its access can't be checked ahead of time.",
		Check:   "Inputs named method with path, url or endpoint.",
		Fix:     "Write one tool per task. Agents with build access already have search and execute for exploring.",
		Sources: []string{"anthropic-tools", "cloudflare-code-mode"}},
	{ID: "tool-count", Severity: Warn, Section: "Tools for tasks, not endpoints", Title: "Few tools per toolkit",
		Why:     "Every tool is read on every turn; similar tools blur together as the list grows.",
		Check:   "Warn over 15 tools in one toolkit.",
		Fix:     "Split toolkits by task, or merge tools that are steps of one job.",
		Sources: []string{"anthropic-tools", "anthropic-code"}},
	{ID: "list-tokens", Severity: Warn, Section: "Tools for tasks, not endpoints", Title: "Small tools/list",
		Why:     "Agents pay for tool definitions in every conversation before doing any work.",
		Check:   "Warn over 4,000 tokens for a toolkit's tools/list.",
		Fix:     "Shorten descriptions and schemas, drop unused tools, or split the toolkit.",
		Sources: []string{"anthropic-code"}},
	{ID: "tool-tokens", Severity: Warn, Section: "Tools for tasks, not endpoints", Title: "Small tool definition",
		Why:     "One long definition costs tokens in every conversation.",
		Check:   "One tool's definition over 800 tokens.",
		Fix:     "Move detail into short parameter descriptions; drop long examples.",
		Sources: []string{"anthropic-code"}},
	{ID: "name", Severity: Warn, Section: "Names and descriptions", Title: "Specific tool name",
		Why:     "Agents choose tools by name first. A generic name says nothing about what it returns.",
		Check:   "Generic names (new_tool, get_data, run…) warn; one-word names get a hint.",
		Fix:     "Name it verb_object after the job: find_vulnerabilities, get_customer_summary.",
		Sources: []string{"anthropic-tools"}},
	{ID: "description", Severity: Warn, Section: "Names and descriptions", Title: "Useful description",
		Why:     "The description is how an agent decides to call the tool.",
		Check:   "Empty, starter text left in, under 25 characters, or over 1,024 characters.",
		Fix:     "Say what it returns and what it needs, in one or two sentences.",
		Sources: []string{"anthropic-tools", "mcp-tools"}},
	{ID: "use-when", Severity: Hint, Section: "Names and descriptions", Title: "Says when to use it",
		Why:     "With several tools, a short \"use when\" stops agents mixing them up.",
		Check:   "Toolkits with 4 or more tools: a tool has no useWhen.",
		Fix:     "Add useWhen: the question this tool answers.",
		Sources: []string{"anthropic-tools"}},
	{ID: "write-guard", Severity: Warn, Section: "Names and descriptions", Title: "Write tools say when not to use them",
		Why:     "Write tools change data. Agents should know when not to call them.",
		Check:   "A write tool without dontUseWhen.",
		Fix:     "Add dontUseWhen, for example \"just looking something up\".",
		Sources: []string{"anthropic-tools", "mcp-tools"}},
	{ID: "actionable-errors", Severity: Hint, Section: "Names and descriptions", Title: "Errors say what to do",
		Why:     "Agents correct themselves from tool errors when the message says what was wrong and what to try.",
		Check:   "throw new Error(...) with a message under 20 characters or a bare \"error\", \"failed\", \"invalid\".",
		Fix:     "Say what was wrong and what input would work, for example \"No image named x; call list_images for names\".",
		Sources: []string{"anthropic-tools", "mcp-tools"}},
	{ID: "param-name", Severity: Warn, Section: "Inputs agents can fill", Title: "Unambiguous parameter names",
		Why:     "An agent guesses what id or name refers to, and guesses wrong when it holds several.",
		Check:   "Parameters named id, name, q, data, value, type, obj, input.",
		Fix:     "Use a specific name such as user_id.",
		Sources: []string{"anthropic-tools"}},
	{ID: "param-docs", Severity: Hint, Section: "Inputs agents can fill", Title: "Documented parameters",
		Why:     "A one-line description with an example value cuts wrong inputs.",
		Check:   "Parameters with no description and no enum.",
		Fix:     "Add a description to each parameter.",
		Sources: []string{"anthropic-tools"}},
	{ID: "enum", Severity: Hint, Section: "Inputs agents can fill", Title: "Closed sets are enums",
		Why:     "A status or sort order typed as free text gets misspelled; an enum lists the valid values.",
		Check:   "String parameters named status, state, sort, order, format, severity, priority or kind without enum.",
		Fix:     "Add enum: [...] with the accepted values.",
		Sources: []string{"anthropic-tools"}},
	{ID: "required", Severity: Warn, Section: "Inputs agents can fill", Title: "Required parameters are marked",
		Why:     "Agents call a tool with nothing and get an error or everything.",
		Check:   "Parameters exist but none are required.",
		Fix:     "List the parameters the tool can't work without in required.",
		Sources: []string{"anthropic-tools", "mcp-tools"}},
	{ID: "params", Severity: Warn, Section: "Inputs agents can fill", Title: "Few parameters",
		Why:     "Agents fill many parameters badly.",
		Check:   "More than 8 parameters.",
		Fix:     "Bind fixed values as toolkit variables; keep inputs to what changes per question.",
		Sources: []string{"anthropic-tools"}},
	{ID: "no-params", Severity: Hint, Section: "Inputs agents can fill", Title: "Tools without inputs say so",
		Why:     "The spec recommends additionalProperties: false for a tool with no parameters.",
		Check:   "No properties and additionalProperties not false.",
		Fix:     "Use { type: \"object\", additionalProperties: false }.",
		Sources: []string{"mcp-tools"}},
	{ID: "pagination", Severity: Hint, Section: "Return only what the task needs", Title: "Lists can be limited",
		Why:     "A list grows with the data; without a limit every call can return thousands of items.",
		Check:   "Code that maps or filters a list, with no limit, page, cursor or first input and no slice.",
		Fix:     "Add a limit input with a small default and return a hint when there is more.",
		Sources: []string{"anthropic-tools"}},
	{ID: "raw-return", Severity: Warn, Section: "Return only what the task needs", Title: "Returns picked fields",
		Why:     "Every field of the upstream response is paid for in tokens on every call.",
		Check:   "return res; with no pick(...).",
		Fix:     "Return only the fields the task needs with pick(res, [...]).",
		Sources: []string{"anthropic-tools", "anthropic-code"}},
	{ID: "output-tokens", Severity: Warn, Section: "Return only what the task needs", Title: "Small outputs",
		Why:     "Large results fill the agent's context. Claude Code cuts tool results over 25,000 tokens by default.",
		Check:   "Warn over 2,000 tokens in a test or run.",
		Fix:     "Drop fields, summarise lists, add a limit.",
		Sources: []string{"anthropic-tools"}},
	{ID: "field-count", Severity: Warn, Section: "Return only what the task needs", Title: "Few fields",
		Why:     "Agents rarely need more than a handful of fields to answer a question.",
		Check:   "More than 25 leaf fields in an output.",
		Fix:     "Return pick(res, [...]) with the fields the task needs.",
		Sources: []string{"anthropic-tools"}},
	{ID: "unbounded-list", Severity: Warn, Section: "Return only what the task needs", Title: "Short lists",
		Why:     "Long lists cost tokens per item.",
		Check:   "A list of more than 50 items in an output.",
		Fix:     "Add a limit input, filter by what the agent asked, or return counts.",
		Sources: []string{"anthropic-tools"}},
	{ID: "blob", Severity: Warn, Section: "Return only what the task needs", Title: "No encoded blobs",
		Why:     "Base64 and token-like strings are expensive and meaningless to an agent.",
		Check:   "Strings of 200+ base64 characters.",
		Fix:     "Leave them out, or return a short reference.",
		Sources: []string{"anthropic-tools"}},
	{ID: "deep-nesting", Severity: Hint, Section: "Return only what the task needs", Title: "Flat outputs",
		Why:     "Deep objects cost brackets and keys at every level.",
		Check:   "Nested more than 4 levels.",
		Fix:     "Flatten to the fields the agent uses.",
		Sources: []string{"anthropic-tools"}},
	{ID: "empty-fields", Severity: Hint, Section: "Return only what the task needs", Title: "No always-empty fields",
		Why:     "Empty fields cost tokens and say nothing.",
		Check:   "Fields that are null or empty in every item.",
		Fix:     "Drop them.",
		Sources: []string{"anthropic-tools"}},
	{ID: "long-text", Severity: Hint, Section: "Return only what the task needs", Title: "Short text fields",
		Why:     "Long text fills the context.",
		Check:   "Strings over 2,000 characters.",
		Fix:     "Return a summary, with a separate tool for the full text.",
		Sources: []string{"anthropic-tools"}},
	{ID: "opaque-ids", Severity: Hint, Section: "Return only what the task needs", Title: "Names next to IDs",
		Why:     "UUIDs mean little to an agent and are easy to copy wrong.",
		Check:   "Three or more fields holding UUIDs.",
		Fix:     "Return names alongside IDs; keep only IDs another tool needs.",
		Sources: []string{"anthropic-tools"}},
}

var ruleByID = func() map[string]Rule {
	m := map[string]Rule{}
	for _, r := range Rules {
		m[r.ID] = r
	}
	return m
}()

var sourceByKey = func() map[string]Source {
	m := map[string]Source{}
	for _, s := range Sources {
		m[s.Key] = s
	}
	return m
}()

// SourceURL is the first source of a rule.
func SourceURL(rule string) string {
	if r, ok := ruleByID[rule]; ok && len(r.Sources) > 0 {
		return sourceByKey[r.Sources[0]].URL
	}
	return ""
}

func withSources(fs []Finding) []Finding {
	for i := range fs {
		if fs[i].Source == "" {
			fs[i].Source = SourceURL(fs[i].Rule)
		}
	}
	return fs
}

// Blocking returns the findings that stop a save.
func Blocking(fs []Finding) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Level == Block {
			out = append(out, f)
		}
	}
	return out
}
