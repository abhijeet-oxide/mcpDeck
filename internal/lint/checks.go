package lint

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// MaxToolTokens is one tool definition's budget.
const MaxToolTokens = 800

var (
	nameSpec = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
	// Credential shapes, each specific enough to rarely match prose.
	secretShapes = []struct {
		what string
		re   *regexp.Regexp
	}{
		{"a private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
		{"a bearer token", regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{20,}`)},
		{"an AWS access key", regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)},
		{"a GitHub token", regexp.MustCompile(`\b(ghp|gho|ghs|ghu|ghr)_[A-Za-z0-9]{36}\b|\bgithub_pat_[A-Za-z0-9_]{40,}`)},
		{"a Slack token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`)},
		{"an API key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`)},
		{"a JWT", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)},
		{"a password", regexp.MustCompile(`(?i)["'](password|passwd|api[_-]?key|client[_-]?secret)["']\s*:\s*["'][^"'{}\s]{6,}["']`)},
	}
	secretParams = regexp.MustCompile(`(?i)^(password|passwd|pass|secret|client_?secret|api_?key|apikey|access_?token|refresh_?token|auth_?token|bearer_?token|id_?token|session_?token|private_?key|credentials?)$`)
	credKey      = regexp.MustCompile(`(?i)^(.*[_-])?(password|passwd|secret|client_?secret|api_?key|apikey|access_?token|refresh_?token|auth_?token|id_?token|session_?token|token|authorization|cookie|set-cookie|private_?key)$`)
	notCredKey   = regexp.MustCompile(`(?i)^(next|page|continuation|cursor|prev|previous|csrf_?token_?name)[_-]?token$|(?i)token_?(count|type|usage)$`)
	closedSets   = map[string]bool{"status": true, "state": true, "sort": true, "sort_by": true, "order": true, "sort_order": true, "format": true, "response_format": true, "severity": true, "priority": true, "kind": true}
	jsonTypes    = map[string]bool{"string": true, "number": true, "integer": true, "boolean": true, "object": true, "array": true, "null": true}
	limitParams  = regexp.MustCompile(`(?i)^(limit|max|max_results|page|page_size|per_page|cursor|first|top|offset|count|n)$`)
	listCode     = regexp.MustCompile(`\.(map|filter)\(`)
	sliceCode    = regexp.MustCompile(`\.slice\(|\bpaginate\(|\blimit\b`)
	throwRe      = regexp.MustCompile("throw new Error\\(\\s*[\"'`]([^\"'`]*)[\"'`]\\s*\\)")
	bareError    = regexp.MustCompile(`(?i)^(error|failed|failure|invalid|bad request|not found|oops|something went wrong)\.?$`)
)

func credentialKey(k string) bool { return credKey.MatchString(k) && !notCredKey.MatchString(k) }

// FindSecret reports what kind of credential s holds, or "".
func FindSecret(s string) string {
	for _, sh := range secretShapes {
		if sh.re.MatchString(s) {
			return sh.what
		}
	}
	return ""
}

// checks are the rules added for the MCP spec and security (LINT-1).
func checks(t Tool) []Finding {
	var out []Finding
	add := func(rule, level, path, title, detail, fix string) {
		out = append(out, Finding{Rule: rule, Level: level, Tool: t.Name, Path: path, Title: title, Detail: detail, Fix: fix})
	}
	if !nameSpec.MatchString(t.Name) {
		add("name-format", Warn, "", "Tool name "+fmt.Sprintf("%q", t.Name)+" has characters the spec doesn't allow",
			"Names should be 1 to 128 letters, digits, _, - or .", "Rename it to snake_case.")
	}
	out = append(out, schemaFindings(t)...)
	// Credentials written into the tool.
	for _, part := range []struct{ where, text string }{{"description", describe(t)}, {"input schema", string(t.InputSchema)}, {"code", t.Source}} {
		if what := FindSecret(part.text); what != "" {
			add("secret-in-definition", Block, part.where, "Credential in the tool's "+part.where,
				"The "+part.where+" contains "+what+". It would be stored in every version"+map[bool]string{true: " and sent to every agent.", false: "."}[part.where != "code"],
				"Store it as a secret variable on the application and use {{name}}, or set the application's auth.")
			break
		}
	}
	// Annotations against what the tool really does.
	if a := t.Annotations; a != nil && t.Access == "write" {
		if a.ReadOnlyHint != nil && *a.ReadOnlyHint {
			add("write-annotation", Block, "annotations.readOnlyHint", "Write tool claims to be read-only",
				"It calls a write request, so clients that skip confirmation for read-only tools would run it unasked.",
				"Remove readOnlyHint; mcpDeck sets it from the requests the tool calls.")
		} else if a.DestructiveHint != nil && !*a.DestructiveHint && hasMethod(t.Methods, "DELETE") {
			add("write-annotation", Block, "annotations.destructiveHint", "Deleting tool claims to be non-destructive",
				"It calls a DELETE request but declares destructiveHint: false, so clients may not ask before it runs.",
				"Remove destructiveHint: false.")
		}
	}
	// Code.
	if listCode.MatchString(t.Source) && !sliceCode.MatchString(t.Source) && !hasParam(t, limitParams) {
		add("pagination", Hint, "", "Lists without a limit",
			"The code builds a list but takes no limit, page or cursor input, so results grow with the data.",
			"Add a limit input with a small default, use .slice(0, input.limit), and say when there is more.")
	}
	for _, m := range throwRe.FindAllStringSubmatch(t.Source, -1) {
		msg := strings.TrimSpace(m[1])
		if len(msg) < 20 || bareError.MatchString(msg) {
			add("actionable-errors", Hint, "", fmt.Sprintf("Error %q doesn't say what to do", msg),
				"Agents correct themselves from the error text. A short or generic message gives them nothing to work with.",
				"Say what was wrong and what to try: \"No image named x; call list_images for names\".")
			break
		}
	}
	if n := ListTokens(t); n > MaxToolTokens {
		add("tool-tokens", Warn, "", fmt.Sprintf("Definition is %s tokens", fmtNum(n)),
			"Every agent pays for this definition in every conversation.", "Move detail into short parameter descriptions; drop long examples.")
	}
	return out
}

func hasMethod(ms []string, m string) bool {
	for _, x := range ms {
		if strings.EqualFold(x, m) {
			return true
		}
	}
	return false
}

func hasParam(t Tool, re *regexp.Regexp) bool {
	var s struct {
		Properties map[string]any `json:"properties"`
	}
	_ = json.Unmarshal(orEmpty(t.InputSchema), &s)
	for k := range s.Properties {
		if re.MatchString(k) {
			return true
		}
	}
	return false
}

// schemaFindings checks the input schema against the MCP spec (LINT-1).
func schemaFindings(t Tool) []Finding {
	var out []Finding
	bad := func(path, detail string) {
		out = append(out, Finding{Rule: "schema-valid", Level: Block, Tool: t.Name, Path: path, Title: "Input schema is not valid",
			Detail: detail, Fix: "Use { type: \"object\", properties: {...}, required: [...] }."})
	}
	raw := strings.TrimSpace(string(t.InputSchema))
	if raw == "" {
		return nil // tools default to {"type":"object"}
	}
	var root any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		bad("", "inputSchema is not JSON: "+err.Error())
		return out
	}
	obj, ok := root.(map[string]any)
	if !ok {
		bad("", "inputSchema must be a JSON Schema object, not "+kind(root)+".")
		return out
	}
	if ty, has := obj["type"]; has && ty != "object" {
		bad("type", fmt.Sprintf("The top-level type is %v; MCP tool inputs are objects.", ty))
	} else if !has {
		bad("type", "The top-level schema has no type; MCP tool inputs must be type \"object\".")
	}
	props := map[string]any{}
	if p, has := obj["properties"]; has {
		pm, ok := p.(map[string]any)
		if !ok {
			bad("properties", "properties must be an object of parameter schemas.")
		} else {
			props = pm
		}
	}
	if r, has := obj["required"]; has {
		list, ok := r.([]any)
		if !ok {
			bad("required", "required must be a list of parameter names.")
		} else {
			for _, x := range list {
				name, ok := x.(string)
				if !ok {
					bad("required", "required must contain only strings.")
					continue
				}
				if _, ok := props[name]; !ok {
					bad("required", fmt.Sprintf("%q is required but not in properties.", name))
				}
			}
		}
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p, ok := props[k].(map[string]any)
		if !ok {
			if _, isBool := props[k].(bool); !isBool {
				bad(k, fmt.Sprintf("Parameter %q must be a schema object.", k))
			}
			continue
		}
		switch ty := p["type"].(type) {
		case string:
			if !jsonTypes[ty] {
				bad(k, fmt.Sprintf("Parameter %q has type %q; use string, number, integer, boolean, object, array or null.", k, ty))
			}
		case []any:
			for _, x := range ty {
				if s, _ := x.(string); !jsonTypes[s] {
					bad(k, fmt.Sprintf("Parameter %q has an unknown type %v.", k, x))
				}
			}
		case nil:
		default:
			bad(k, fmt.Sprintf("Parameter %q: type must be a string or a list of strings.", k))
		}
		if e, has := p["enum"]; has {
			if l, ok := e.([]any); !ok || len(l) == 0 {
				bad(k, fmt.Sprintf("Parameter %q: enum must be a non-empty list.", k))
			}
		}
		if p["type"] == "array" {
			if it, has := p["items"]; has {
				if _, ok := it.(map[string]any); !ok {
					if _, isBool := it.(bool); !isBool {
						bad(k, fmt.Sprintf("Parameter %q: items must be a schema object.", k))
					}
				}
			}
		}
		// Credentials as inputs.
		if secretParams.MatchString(k) {
			out = append(out, Finding{Rule: "secret-input", Level: Block, Tool: t.Name, Path: k, Title: "Agents would pass " + k,
				Detail: "An input named " + k + " puts a credential in the agent's context and transcript.",
				Fix:    "Set the application's auth (login steps, OAuth, API key) so mcpDeck adds it; remove the input."})
		}
		if closedSets[strings.ToLower(k)] && p["type"] == "string" && p["enum"] == nil {
			out = append(out, Finding{Rule: "enum", Level: Hint, Tool: t.Name, Path: k, Title: k + " has no enum",
				Detail: "Agents guess the spelling of a value from a closed set.", Fix: "Add enum: [...] with the accepted values."})
		}
	}
	// A catch-all: method plus path or url.
	_, hasMethod := props["method"]
	_, hasPath := props["path"]
	_, hasURL := props["url"]
	_, hasEndpoint := props["endpoint"]
	if hasMethod && (hasPath || hasURL || hasEndpoint) {
		out = append(out, Finding{Rule: "catch-all", Level: Warn, Tool: t.Name, Title: "Takes any method and path",
			Detail: "An agent can call anything through this tool, and its access can't be checked before it runs.",
			Fix:    "Write one tool per task. Agents with build access already have search and execute."})
	}
	if len(props) == 0 && obj["additionalProperties"] != false && len(out) == 0 {
		out = append(out, Finding{Rule: "no-params", Level: Hint, Tool: t.Name, Title: "No inputs, but any object accepted",
			Detail: "The spec recommends additionalProperties: false for a tool with no parameters.",
			Fix:    "Use { type: \"object\", additionalProperties: false }."})
	}
	return out
}

// similarDescriptions flags tools whose descriptions are nearly the same.
func similarDescriptions(tools []Tool) []Finding {
	var out []Finding
	words := func(s string) map[string]bool {
		m := map[string]bool{}
		for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return (r < 'a' || r > 'z') && (r < '0' || r > '9') }) {
			if len(w) > 2 {
				m[w] = true
			}
		}
		return m
	}
	ws := make([]map[string]bool, len(tools))
	for i, t := range tools {
		ws[i] = words(t.Description)
	}
	for i := 0; i < len(tools); i++ {
		for j := i + 1; j < len(tools); j++ {
			a, b := ws[i], ws[j]
			if len(a) < 5 || len(b) < 5 {
				continue
			}
			inter := 0
			for w := range a {
				if b[w] {
					inter++
				}
			}
			union := len(a) + len(b) - inter
			if inter*10 >= union*8 {
				out = append(out, Finding{Rule: "overlap", Level: Warn, Tool: tools[j].Name,
					Title:  fmt.Sprintf("%s and %s have nearly the same description", tools[i].Name, tools[j].Name),
					Detail: "Agents choose between tools by description; these read the same.",
					Fix:    "Merge them, or say in each what makes it different and when to use it."})
			}
		}
	}
	return out
}
