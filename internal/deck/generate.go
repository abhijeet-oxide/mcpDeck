package deck

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// GenerateInput asks for a starter tool around one saved request (TOOL-10).
type GenerateInput struct {
	From        string `json:"from"` // api.request
	Name        string `json:"name"` // snake_case; default from the request name
	Description string `json:"description"`
}

// Generated is a starter tool's source, not saved yet.
type Generated struct {
	Source string           `json:"source"`
	Access string           `json:"access"` // read | write
	Name   string           `json:"name"`
	Tests  []store.ToolTest `json:"tests"` // a smoke test when the tool has no required inputs
}

var snakeRe = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// GenerateTool writes a starter tool that calls one request: its path
// parameters are required inputs, its query parameters optional ones. The
// same code backs the UI's "Make a tool", `mcpdeck tool generate` and the
// builder's generate_tool, so all three give the same file.
func (d *Deck) GenerateTool(ctx context.Context, in GenerateInput) (*Generated, error) {
	apiName, reqName, ok := strings.Cut(strings.TrimSpace(in.From), ".")
	if !ok {
		return nil, badRequest("say which request as api.request, like shop.listOrders")
	}
	a, err := d.Store.APIByName(ctx, apiName)
	if err != nil {
		return nil, notFound("API " + apiName)
	}
	reqs, _ := d.Store.Requests(ctx, a.ID)
	var r *store.Request
	for i := range reqs {
		if reqs[i].Name == reqName {
			r = &reqs[i]
		}
	}
	if r == nil {
		return nil, notFound("request " + in.From)
	}
	for _, id := range a.Auth.AuthRequestIDs() {
		if id == r.ID {
			return nil, badRequest("%s signs in to %s; mcpDeck runs it for you, so it can't be a tool", in.From, apiName)
		}
	}
	name := in.Name
	if name == "" {
		name = strings.ToLower(snakeRe.ReplaceAllString(reqName, "${1}_${2}"))
	}
	if !toolNameRe.MatchString(name) {
		return nil, badRequest("tool name %q must be snake_case, 3 to 64 characters, starting with a letter", name)
	}
	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		// The spec's summary says what the operation does; its description
		// often starts with usage notes ("Multiple values can be ...").
		if r.Title != reqName {
			desc = firstSentence(r.Title)
		}
		if desc == "" {
			desc = firstSentence(r.Description)
		}
		if desc == "" || desc == reqName {
			desc = fmt.Sprintf("Calls %s %s on %s", r.Method, strings.TrimPrefix(r.URL, "{{baseUrl}}"), a.Title)
		}
		desc = strings.TrimSuffix(desc, ".") + "."
	}
	type prop struct{ name, typ, desc string }
	var props []prop
	var required []string
	for _, p := range proxy.PathParams(r.URL) {
		props = append(props, prop{p, "string", "path parameter"})
		required = append(required, p)
	}
	for _, q := range r.Query {
		if !validIdent(q.Key) {
			continue
		}
		typ := "string"
		if _, err := fmt.Sscanf(q.Value, "%d", new(int)); err == nil && q.Value != "" {
			typ = "integer"
		}
		props = append(props, prop{q.Key, typ, "query parameter"})
		if !q.Disabled && q.Value == "" {
			required = append(required, q.Key)
		}
	}
	if r.BodyType == "json" && strings.TrimSpace(r.Body) != "" {
		var body map[string]any
		if json.Unmarshal([]byte(r.Body), &body) == nil {
			keys := make([]string, 0, len(body))
			for k := range body {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if !validIdent(k) {
					continue
				}
				if s, ok := body[k].(string); ok && strings.Contains(s, "{{") {
					continue // filled from variables
				}
				typ := "string"
				switch body[k].(type) {
				case float64:
					typ = "number"
				case bool:
					typ = "boolean"
				case map[string]any:
					typ = "object"
				case []any:
					typ = "array"
				}
				props = append(props, prop{k, typ, "body field"})
			}
		}
	}
	var b strings.Builder
	b.WriteString("export const tool = {\n")
	fmt.Fprintf(&b, "  name: %q,\n", name)
	fmt.Fprintf(&b, "  description: %q,\n", desc)
	b.WriteString("  inputSchema: {\n    type: \"object\",\n    properties: {")
	if len(props) == 0 {
		b.WriteString("},\n")
	} else {
		b.WriteString("\n")
		for _, p := range props {
			fmt.Fprintf(&b, "      %s: { type: %q, description: %q },\n", p.name, p.typ, p.desc)
		}
		b.WriteString("    },\n")
	}
	qs := make([]string, len(required))
	for i, s := range required {
		qs[i] = fmt.Sprintf("%q", s)
	}
	fmt.Fprintf(&b, "    required: [%s],\n  },\n};\n\n", strings.Join(qs, ", "))
	b.WriteString("export default async function (input: Input) {\n")
	fmt.Fprintf(&b, "  return await %s.%s({ ...input });\n}\n", apiName, reqName)
	g := &Generated{Source: b.String(), Access: r.EffectiveAccess(), Name: name, Tests: []store.ToolTest{}}
	if len(required) == 0 && g.Access == "read" {
		g.Tests = append(g.Tests, store.ToolTest{Name: "returns data", Input: json.RawMessage(`{}`),
			Script: `pm.test("returns data", () => pm.expect(output).to.exist)`})
	}
	return g, nil
}

var toolNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)
var identRe = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

func validIdent(s string) bool { return identRe.MatchString(s) }

func firstSentence(s string) string {
	s = strings.TrimSpace(strings.SplitN(s, "\n", 2)[0])
	if i := strings.Index(s, ". "); i > 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
