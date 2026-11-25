// Package importer turns OpenAPI, Swagger, Postman and curl into mcpDeck
// requests (IMP-1…4). Everything here is a pure function over bytes.
package importer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	yaml "go.yaml.in/yaml/v4"
)

// Var is a variable found by an import. Secret ones become encrypted credentials.
type Var struct {
	Name   string `json:"name"`
	Value  string `json:"-"`
	Secret bool   `json:"secret"`
	Env    string `json:"env"` // "" = API scope
	// Preview is what the UI may show: the value, or a mask for secrets.
	Preview string `json:"preview"`
}

// Result is what an import found, before anything is saved.
type Result struct {
	Kind         string          `json:"kind"` // openapi | swagger | postman | curl
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	SuggestName  string          `json:"suggestName"`
	BaseURL      string          `json:"baseUrl"`
	Auth         store.Auth      `json:"auth"`
	Requests     []store.Request `json:"requests"`
	Vars         []Var           `json:"vars"`
	Environments []string        `json:"environments"`
	Warnings     []string        `json:"warnings"`
}

// Detect parses any supported document: OpenAPI 3, Swagger 2, a Postman
// collection, or one or more curl commands.
func Detect(data []byte) (*Result, error) {
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil, fmt.Errorf("nothing to import: paste a spec, a Postman collection or a curl command")
	}
	if strings.HasPrefix(text, "curl ") || strings.Contains(text, "\ncurl ") {
		return Curl(text)
	}
	doc, err := toJSONMap(data)
	if err != nil {
		return nil, fmt.Errorf("not JSON, YAML or curl: %w", err)
	}
	switch {
	case doc["openapi"] != nil || doc["swagger"] != nil:
		return OpenAPI(data)
	case doc["info"] != nil && doc["item"] != nil:
		return Postman(data, nil)
	}
	return nil, fmt.Errorf("didn't recognise this as OpenAPI, Swagger, a Postman collection or curl")
}

func toJSONMap(data []byte) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err == nil {
		return m, nil
	}
	var y any
	if err := yaml.Unmarshal(data, &y); err != nil {
		return nil, err
	}
	b, err := json.Marshal(normalizeYAML(y))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("top level must be an object")
	}
	return m, nil
}

func normalizeYAML(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			t[k] = normalizeYAML(x)
		}
		return t
	case map[any]any:
		m := map[string]any{}
		for k, x := range t {
			m[fmt.Sprint(k)] = normalizeYAML(x)
		}
		return m
	case []any:
		for i := range t {
			t[i] = normalizeYAML(t[i])
		}
		return t
	}
	return v
}

var nonWord = regexp.MustCompile(`[^A-Za-z0-9]+`)

// MethodName makes a camelCase identifier: "Get issue" → getIssue.
func MethodName(s string) string {
	parts := nonWord.Split(strings.TrimSpace(s), -1)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if b.Len() == 0 {
			r := []rune(p)
			// keep already camelCase ids (getAccount) intact
			r[0] = unicode.ToLower(r[0])
			b.WriteString(string(r))
			continue
		}
		r := []rune(p)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	out := b.String()
	if out == "" {
		return "request"
	}
	if unicode.IsDigit(rune(out[0])) {
		out = "r" + out
	}
	return out
}

// ClientName makes a short lowercase API client name: "Acme CRM" → acmeCrm.
func ClientName(title string) string {
	n := MethodName(strings.ToLower(title))
	if len(n) > 24 {
		n = n[:24]
	}
	return n
}

// NameFromPath names a request without an operationId: GET /accounts/{id} → getAccountsById.
func NameFromPath(method, path string) string {
	var words []string
	words = append(words, strings.ToLower(method))
	for _, seg := range strings.Split(path, "/") {
		if seg == "" {
			continue
		}
		if strings.HasPrefix(seg, "{") || strings.HasPrefix(seg, ":") {
			words = append(words, "by", strings.Trim(seg, "{}:"))
			continue
		}
		words = append(words, seg)
	}
	return MethodName(strings.Join(words, " "))
}

var nonRESTPath = regexp.MustCompile(`(?i)(delete|remove|update|create|set|reset)`)

// Classify sets Access, AccessReason and AccessFlag (API-1).
func Classify(r *store.Request) {
	m := strings.ToUpper(r.Method)
	r.Access = "write"
	r.AccessReason = m + " writes"
	r.AccessFlag = false
	switch m {
	case "GET", "HEAD", "OPTIONS":
		r.Access = "read"
		r.AccessReason = m + " reads"
	}
	body := strings.ToLower(r.Body)
	switch {
	case strings.Contains(body, "soap:envelope") || strings.Contains(body, "<soapenv:") || strings.Contains(body, "<methodcall>"):
		r.AccessFlag = true
		r.AccessReason = "SOAP or XML-RPC: can't tell whether it reads or writes; confirm it"
	case strings.Contains(body, "mutation ") || strings.Contains(body, `"mutation`):
		r.AccessFlag = true
		r.AccessReason = "GraphQL mutation: confirm whether it writes"
	case r.Access == "read" && nonRESTPath.MatchString(pathOf(r.URL)):
		r.AccessFlag = true
		r.AccessReason = m + " with an action word in its path; confirm it only reads"
	}
	if r.Access == "write" && !r.AccessFlag && m == "POST" && regexp.MustCompile(`(?i)(search|query|find|lookup|list)`).MatchString(r.Name+" "+pathOf(r.URL)) {
		r.AccessFlag = true
		r.AccessReason = "POST that looks like a search: mark it read if it doesn't change anything"
	}
}

func pathOf(u string) string {
	u = regexp.MustCompile(`\{\{[^}]+\}\}`).ReplaceAllString(u, "")
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
		if j := strings.Index(u, "/"); j >= 0 {
			u = u[j:]
		}
	}
	if i := strings.Index(u, "?"); i >= 0 {
		u = u[:i]
	}
	return u
}

// uniqueNames makes request names unique within one import.
func uniqueNames(reqs []store.Request) {
	seen := map[string]int{}
	for i := range reqs {
		n := reqs[i].Name
		seen[n]++
		if seen[n] > 1 {
			reqs[i].Name = fmt.Sprintf("%s%d", n, seen[n])
		}
	}
}

func maskPreview(v string) string {
	if len(v) <= 4 {
		return "••••"
	}
	return "••••" + v[len(v)-4:]
}
