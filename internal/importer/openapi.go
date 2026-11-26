package importer

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
)

// OpenAPI imports an OpenAPI 3.x or Swagger 2.0 document (IMP-1).
func OpenAPI(data []byte) (*Result, error) {
	m, err := toJSONMap(data)
	if err != nil {
		return nil, err
	}
	js, _ := json.Marshal(m)
	res := &Result{Kind: "openapi"}
	var doc *openapi3.T
	if v, ok := m["swagger"].(string); ok && strings.HasPrefix(v, "2") {
		res.Kind = "swagger"
		var d2 openapi2.T
		if err := json.Unmarshal(js, &d2); err != nil {
			return nil, fmt.Errorf("swagger 2.0: %w", err)
		}
		doc, err = openapi2conv.ToV3(&d2)
		if err != nil {
			return nil, fmt.Errorf("swagger 2.0: %w", err)
		}
	} else {
		loader := openapi3.NewLoader()
		loader.IsExternalRefsAllowed = false
		doc, err = loader.LoadFromData(js)
		if err != nil {
			return nil, fmt.Errorf("openapi: %w", err)
		}
	}
	if doc.Info != nil {
		res.Title = doc.Info.Title
		res.Description = strings.TrimSpace(doc.Info.Description)
	}
	res.SuggestName = ClientName(strings.TrimPrefix(strings.TrimPrefix(res.Title, "Acme "), "acme "))
	if len(doc.Servers) > 0 {
		sv := doc.Servers[0]
		u := sv.URL
		// Server variables take their defaults (OpenAPI 3.1 4.8.6).
		for name, v := range sv.Variables {
			if v != nil {
				u = strings.ReplaceAll(u, "{"+name+"}", v.Default)
			}
		}
		res.BaseURL = strings.TrimSuffix(u, "/")
	}
	res.Vars = append(res.Vars, Var{Name: "baseUrl", Value: res.BaseURL, Preview: res.BaseURL})

	// Auth from the scheme the document requires globally, else the first
	// one declared (OpenAPI 3.1 4.8.30).
	if doc.Components != nil {
		names := make([]string, 0, len(doc.Components.SecuritySchemes))
		for n := range doc.Components.SecuritySchemes {
			names = append(names, n)
		}
		sort.Strings(names)
		var global []string
		for _, req := range doc.Security {
			for n := range req {
				global = append(global, n)
			}
		}
		sort.Strings(global)
		names = append(global, names...)
		for _, n := range names {
			ref := doc.Components.SecuritySchemes[n]
			if ref == nil || ref.Value == nil {
				continue
			}
			ss := ref.Value
			switch {
			case ss.Type == "oauth2" && ss.Flows != nil && ss.Flows.ClientCredentials != nil:
				f := ss.Flows.ClientCredentials
				res.Auth = store.Auth{Type: "oauth2", Grant: "client_credentials", TokenURL: f.TokenURL, ClientID: "{{clientId}}", ClientSecret: "{{clientSecret}}", Scope: scopes(f.Scopes)}
				res.Vars = append(res.Vars, Var{Name: "clientId"}, Var{Name: "clientSecret", Secret: true})
			case ss.Type == "oauth2" && ss.Flows != nil && ss.Flows.Password != nil:
				f := ss.Flows.Password
				res.Auth = store.Auth{Type: "oauth2", Grant: "password", TokenURL: f.TokenURL, ClientID: "{{clientId}}", ClientSecret: "{{clientSecret}}",
					Username: "{{username}}", Password: "{{password}}", Scope: scopes(f.Scopes)}
				res.Vars = append(res.Vars, Var{Name: "clientId"}, Var{Name: "clientSecret", Secret: true}, Var{Name: "username"}, Var{Name: "password", Secret: true})
			case ss.Type == "http" && strings.EqualFold(ss.Scheme, "bearer"), ss.Type == "oauth2", ss.Type == "openIdConnect":
				// Authorization-code flows need a browser; a token you paste works.
				res.Auth = store.Auth{Type: "bearer", Token: "{{token}}"}
				res.Vars = append(res.Vars, Var{Name: "token", Secret: true})
			case ss.Type == "http" && strings.EqualFold(ss.Scheme, "basic"):
				res.Auth = store.Auth{Type: "basic", Username: "{{username}}", Password: "{{password}}"}
				res.Vars = append(res.Vars, Var{Name: "username"}, Var{Name: "password", Secret: true})
			case ss.Type == "apiKey":
				in := "header"
				if ss.In == "query" {
					in = "query"
				}
				res.Auth = store.Auth{Type: "apikey", Key: ss.Name, Value: "{{apiKey}}", In: in}
				res.Vars = append(res.Vars, Var{Name: "apiKey", Secret: true})
			}
			if res.Auth.Type != "" {
				break
			}
		}
	}

	paths := doc.Paths.Map()
	keys := make([]string, 0, len(paths))
	for p := range paths {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	methodOrder := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	for _, p := range keys {
		item := paths[p]
		ops := item.Operations()
		for _, method := range methodOrder {
			op, ok := ops[method]
			if !ok || op == nil {
				continue
			}
			r := store.Request{Method: method, URL: "{{baseUrl}}" + p, BodyType: "none"}
			r.Name = MethodName(op.OperationID)
			if op.OperationID == "" {
				r.Name = NameFromPath(method, p)
			}
			r.Title = op.Summary
			if r.Title == "" {
				r.Title = r.Name
			}
			r.Description = strings.TrimSpace(op.Description)
			if len(op.Tags) > 0 {
				r.Folder = op.Tags[0]
			}
			params := append(openapi3.Parameters{}, item.Parameters...)
			params = append(params, op.Parameters...)
			for _, pr := range params {
				if pr.Value == nil {
					continue
				}
				switch pr.Value.In {
				case "query":
					r.Query = append(r.Query, store.KV{Key: pr.Value.Name, Value: exampleString(pr.Value), Disabled: !pr.Value.Required})
				case "header":
					r.Headers = append(r.Headers, store.KV{Key: pr.Value.Name, Value: exampleString(pr.Value), Disabled: !pr.Value.Required})
				}
			}
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				requestBody(&r, op.RequestBody.Value.Content)
			}
			if op.Responses != nil {
				for _, code := range []string{"200", "201", "default"} {
					rr := op.Responses.Value(code)
					if rr == nil || rr.Value == nil {
						continue
					}
					for ct, mt := range rr.Value.Content {
						if !strings.Contains(ct, "json") {
							continue
						}
						if mt.Schema != nil && mt.Schema.Value != nil {
							if b, err := json.Marshal(mt.Schema.Value); err == nil {
								r.ResponseSchema = b
							}
						}
						if ex := exampleOf(mt); ex != nil {
							b, _ := json.Marshal(ex)
							r.Example = b
						}
					}
					break
				}
			}
			Classify(&r)
			res.Requests = append(res.Requests, r)
		}
	}
	uniqueNames(res.Requests)
	if len(res.Requests) == 0 {
		res.Warnings = append(res.Warnings, "the spec has no operations")
	}
	return res, nil
}

func exampleString(p *openapi3.Parameter) string {
	if p.Example != nil {
		return fmt.Sprint(p.Example)
	}
	if p.Schema != nil && p.Schema.Value != nil {
		if p.Schema.Value.Example != nil {
			return fmt.Sprint(p.Schema.Value.Example)
		}
		if p.Schema.Value.Default != nil {
			return fmt.Sprint(p.Schema.Value.Default)
		}
	}
	return ""
}

func exampleOf(mt *openapi3.MediaType) any {
	if mt == nil {
		return nil
	}
	if mt.Example != nil {
		return mt.Example
	}
	for _, ex := range mt.Examples {
		if ex != nil && ex.Value != nil && ex.Value.Value != nil {
			return ex.Value.Value
		}
	}
	if mt.Schema != nil && mt.Schema.Value != nil {
		return sampleFromSchema(mt.Schema.Value, 0)
	}
	return nil
}

// sampleFromSchema builds a small example value from a schema.
func sampleFromSchema(s *openapi3.Schema, depth int) any {
	if s == nil || depth > 5 {
		return nil
	}
	if s.Example != nil {
		return s.Example
	}
	if s.Default != nil {
		return s.Default
	}
	if len(s.Enum) > 0 {
		return s.Enum[0]
	}
	switch {
	case s.Type.Is("object") || len(s.Properties) > 0:
		out := map[string]any{}
		for k, p := range s.Properties {
			if p != nil {
				out[k] = sampleFromSchema(p.Value, depth+1)
			}
		}
		return out
	case s.Type.Is("array"):
		if s.Items != nil {
			return []any{sampleFromSchema(s.Items.Value, depth+1)}
		}
		return []any{}
	case s.Type.Is("integer"), s.Type.Is("number"):
		return 0
	case s.Type.Is("boolean"):
		return false
	case s.Type.Is("string"):
		return "string"
	}
	return nil
}

func scopes(m map[string]string) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

// requestBody picks the body an operation takes: JSON first, then form,
// XML or text, keeping the exact content type (merge-patch, AQL text).
func requestBody(r *store.Request, content openapi3.Content) {
	cts := make([]string, 0, len(content))
	for ct := range content {
		cts = append(cts, ct)
	}
	rank := func(ct string) int {
		switch {
		case ct == "application/json":
			return 0
		case strings.Contains(ct, "json"):
			return 1
		case strings.Contains(ct, "x-www-form-urlencoded"):
			return 2
		case strings.Contains(ct, "xml"):
			return 3
		case strings.HasPrefix(ct, "text/"):
			return 4
		}
		return 9
	}
	sort.Slice(cts, func(i, j int) bool {
		return rank(cts[i]) < rank(cts[j]) || (rank(cts[i]) == rank(cts[j]) && cts[i] < cts[j])
	})
	if len(cts) == 0 || rank(cts[0]) == 9 {
		return
	}
	ct, mt := cts[0], content[cts[0]]
	ex := exampleOf(mt)
	switch {
	case strings.Contains(ct, "json"):
		r.BodyType = "json"
		r.Body = "{}"
		if ex != nil {
			b, _ := json.MarshalIndent(ex, "", "  ")
			r.Body = string(b)
		}
	case strings.Contains(ct, "x-www-form-urlencoded"):
		r.BodyType = "form"
		if m, ok := ex.(map[string]any); ok {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var parts []string
			for _, k := range keys {
				parts = append(parts, k+"="+fmt.Sprint(m[k]))
			}
			r.Body = strings.Join(parts, "&")
		}
	case strings.Contains(ct, "xml"):
		r.BodyType = "xml"
		if s, ok := ex.(string); ok {
			r.Body = s
		}
	default:
		r.BodyType = "text"
		if s, ok := ex.(string); ok {
			r.Body = s
		}
	}
	if ct != "application/json" && ct != "application/x-www-form-urlencoded" {
		r.Headers = append(r.Headers, store.KV{Key: "Content-Type", Value: ct})
	}
}
