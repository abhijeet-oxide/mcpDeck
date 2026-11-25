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
		res.BaseURL = strings.TrimSuffix(doc.Servers[0].URL, "/")
	}
	res.Vars = append(res.Vars, Var{Name: "baseUrl", Value: res.BaseURL, Preview: res.BaseURL})

	// Auth from the first security scheme used globally.
	if doc.Components != nil {
		names := make([]string, 0, len(doc.Components.SecuritySchemes))
		for n := range doc.Components.SecuritySchemes {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			ss := doc.Components.SecuritySchemes[n].Value
			if ss == nil {
				continue
			}
			switch {
			case ss.Type == "http" && strings.EqualFold(ss.Scheme, "bearer"), ss.Type == "oauth2":
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
				for ct, mt := range op.RequestBody.Value.Content {
					if !strings.Contains(ct, "json") {
						continue
					}
					r.BodyType = "json"
					if ex := exampleOf(mt); ex != nil {
						b, _ := json.MarshalIndent(ex, "", "  ")
						r.Body = string(b)
					} else {
						r.Body = "{}"
					}
				}
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
