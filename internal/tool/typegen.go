package tool

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

var identRe = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

func propName(k string) string {
	if identRe.MatchString(k) {
		return k
	}
	b, _ := json.Marshal(k)
	return string(b)
}

// tsFromSchema renders a JSON schema as a TypeScript type.
func tsFromSchema(s map[string]any, indent string, depth int) string {
	if s == nil || depth > 6 {
		return "unknown"
	}
	if e, ok := s["enum"].([]any); ok && len(e) > 0 {
		var parts []string
		for _, v := range e {
			b, _ := json.Marshal(v)
			parts = append(parts, string(b))
		}
		return strings.Join(parts, " | ")
	}
	t, _ := s["type"].(string)
	if ts, ok := s["type"].([]any); ok && len(ts) > 0 {
		t, _ = ts[0].(string)
	}
	props, hasProps := s["properties"].(map[string]any)
	switch {
	case t == "object" || hasProps:
		if len(props) == 0 {
			return "Record<string, unknown>"
		}
		req := map[string]bool{}
		if r, ok := s["required"].([]any); ok {
			for _, x := range r {
				req[fmt.Sprint(x)] = true
			}
		}
		keys := make([]string, 0, len(props))
		for k := range props {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteString("{\n")
		for _, k := range keys {
			ps, _ := props[k].(map[string]any)
			opt := "?"
			if req[k] {
				opt = ""
			}
			if d, ok := ps["description"].(string); ok && d != "" {
				b.WriteString(indent + "  /** " + strings.ReplaceAll(d, "*/", "") + " */\n")
			}
			b.WriteString(indent + "  " + propName(k) + opt + ": " + tsFromSchema(ps, indent+"  ", depth+1) + ";\n")
		}
		b.WriteString(indent + "}")
		return b.String()
	case t == "array":
		items, _ := s["items"].(map[string]any)
		it := tsFromSchema(items, indent, depth+1)
		if strings.Contains(it, "|") {
			it = "(" + it + ")"
		}
		return it + "[]"
	case t == "string":
		return "string"
	case t == "integer" || t == "number":
		return "number"
	case t == "boolean":
		return "boolean"
	case t == "null":
		return "null"
	}
	return "unknown"
}

// SchemaFromExample infers a JSON schema from an example value (EX-2).
func SchemaFromExample(v any) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		props := map[string]any{}
		var req []any
		for k, x := range t {
			props[k] = SchemaFromExample(x)
			req = append(req, k)
		}
		return map[string]any{"type": "object", "properties": props, "required": req}
	case []any:
		if len(t) == 0 {
			return map[string]any{"type": "array", "items": map[string]any{}}
		}
		// Merge object items so optional fields show up.
		merged := SchemaFromExample(t[0])
		for _, x := range t[1:] {
			merged = mergeSchemas(merged, SchemaFromExample(x))
		}
		return map[string]any{"type": "array", "items": merged}
	case string:
		return map[string]any{"type": "string"}
	case float64, int, int64:
		return map[string]any{"type": "number"}
	case bool:
		return map[string]any{"type": "boolean"}
	case nil:
		return map[string]any{"type": "null"}
	}
	return map[string]any{}
}

func mergeSchemas(a, b map[string]any) map[string]any {
	ap, aok := a["properties"].(map[string]any)
	bp, bok := b["properties"].(map[string]any)
	if !aok || !bok {
		return a
	}
	for k, v := range bp {
		if _, ok := ap[k]; !ok {
			ap[k] = v
		}
	}
	// required = intersection
	inB := map[string]bool{}
	if r, ok := b["required"].([]any); ok {
		for _, x := range r {
			inB[fmt.Sprint(x)] = true
		}
	}
	var req []any
	if r, ok := a["required"].([]any); ok {
		for _, x := range r {
			if inB[fmt.Sprint(x)] {
				req = append(req, x)
			}
		}
	}
	a["required"] = req
	return a
}

// ResponseSchema is the request's response schema, or one inferred from its example.
func ResponseSchema(r store.Request) map[string]any {
	if len(r.ResponseSchema) > 0 {
		var s map[string]any
		if json.Unmarshal(r.ResponseSchema, &s) == nil && len(s) > 0 && (s["properties"] != nil || s["items"] != nil || s["type"] != nil) {
			return s
		}
	}
	if len(r.Example) > 0 {
		var v any
		if json.Unmarshal(r.Example, &v) == nil {
			return SchemaFromExample(v)
		}
	}
	return nil
}

// ArgNames lists a request's arguments: path params, query keys and top-level body keys.
func ArgNames(r store.Request) (path, query, body []string) {
	path = proxy.PathParams(r.URL)
	for _, q := range r.Query {
		query = append(query, q.Key)
	}
	if r.BodyType == "json" && strings.TrimSpace(r.Body) != "" {
		var m map[string]any
		if json.Unmarshal([]byte(r.Body), &m) == nil {
			for k := range m {
				body = append(body, k)
			}
			sort.Strings(body)
		}
	}
	return
}

func typeName(r store.Request) string {
	n := r.Name
	if n == "" {
		return "Response"
	}
	return strings.ToUpper(n[:1]) + n[1:] + "Response"
}

// routeOf is METHOD /path without the base URL variable.
func routeOf(r store.Request) string {
	u := regexp.MustCompile(`^\{\{[^}]+\}\}`).ReplaceAllString(r.URL, "")
	if i := strings.Index(u, "://"); i >= 0 {
		if j := strings.Index(u[i+3:], "/"); j >= 0 {
			u = u[i+3+j:]
		}
	}
	return strings.ToUpper(r.Method) + " " + u
}

// Signature is the one-line form search_apis returns (MCP-3):
// crm.getAccount({ id }) → GetAccountResponse · GET /accounts/{id} · read
func Signature(api store.API, r store.Request) string {
	p, q, b := ArgNames(r)
	args := append(append(append([]string{}, p...), q...), b...)
	a := ""
	if len(args) > 0 {
		if len(args) > 6 {
			args = append(args[:6], "…")
		}
		a = "{ " + strings.Join(args, ", ") + " }"
	}
	access := r.EffectiveAccess()
	if r.AccessFlag {
		access = "check access (counts as write)"
	}
	return fmt.Sprintf("%s.%s(%s) → %s · %s · %s", api.Name, r.Name, a, typeName(r), routeOf(r), access)
}

// Describe returns the TypeScript declaration of one request (describe_api).
func Describe(api store.API, r store.Request) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// %s · %s\n", routeOf(r), r.EffectiveAccess())
	if r.Description != "" {
		b.WriteString("// " + strings.ReplaceAll(strings.TrimSpace(r.Description), "\n", " ") + "\n")
	}
	fmt.Fprintf(&b, "%s.%s(args: %s): Promise<%s>\n", api.Name, r.Name, argsType(r), typeName(r))
	fmt.Fprintf(&b, "type %s = %s\n", typeName(r), tsFromSchema(ResponseSchema(r), "", 0))
	return b.String()
}

func argsType(r store.Request) string {
	p, q, body := ArgNames(r)
	var fields []string
	for _, n := range p {
		fields = append(fields, propName(n)+": string | number")
	}
	for _, n := range q {
		fields = append(fields, propName(n)+"?: string | number | boolean")
	}
	for _, n := range body {
		fields = append(fields, propName(n)+"?: unknown")
	}
	fields = append(fields, "path?: Record<string, unknown>", "query?: Record<string, unknown>", "headers?: Record<string, string>", "body?: unknown")
	return "{ " + strings.Join(fields, "; ") + " }"
}

// DTS generates mcpdeck.d.ts for the editor and for tools as files (TOOL-3).
func DTS(clients []Client) string {
	var b strings.Builder
	b.WriteString("// Generated by mcpDeck. Types for tool code: one namespace per API, one function per request.\n\n")
	b.WriteString("declare type Input = any;\n")
	b.WriteString("declare const vars: Readonly<Record<string, string>>;\n")
	b.WriteString("declare class ApiError extends Error { status: number; body: unknown; code: string }\n")
	var names []string
	for _, c := range clients {
		names = append(names, fmt.Sprintf("%q", c.API.Name))
	}
	if len(names) == 0 {
		names = []string{"string"}
	}
	b.WriteString("/** Call any route of an API without a saved request: call(\"crm\", \"GET /owners/{id}\", { id }) */\n")
	b.WriteString("declare function call(api: " + strings.Join(names, " | ") + ", route: string, args?: Record<string, unknown>): Promise<any>;\n\n")
	for _, c := range clients {
		title := c.API.Title
		if title == "" {
			title = c.API.Name
		}
		b.WriteString("/** " + title + " */\n")
		b.WriteString("declare namespace " + c.API.Name + " {\n")
		for _, r := range c.Requests {
			doc := routeOf(r) + " · " + r.EffectiveAccess()
			if r.Title != "" && r.Title != r.Name {
				doc = r.Title + " · " + doc
			}
			b.WriteString("  /** " + strings.ReplaceAll(doc, "*/", "") + " */\n")
			b.WriteString("  function " + r.Name + "(args?: " + argsType(r) + "): Promise<" + typeName(r) + ">;\n")
			b.WriteString("  type " + typeName(r) + " = " + tsFromSchema(ResponseSchema(r), "  ", 0) + ";\n")
		}
		b.WriteString("}\n\n")
	}
	return b.String()
}
