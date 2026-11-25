package importer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

type pmCollection struct {
	Info struct {
		Name        string `json:"name"`
		Description any    `json:"description"`
	} `json:"info"`
	Auth     *pmAuth  `json:"auth"`
	Variable []pmKV   `json:"variable"`
	Item     []pmItem `json:"item"`
}

type pmKV struct {
	Key      string `json:"key"`
	Value    any    `json:"value"`
	Type     string `json:"type"`
	Disabled bool   `json:"disabled"`
	Enabled  *bool  `json:"enabled"`
}

type pmAuth struct {
	Type   string `json:"type"`
	Bearer []pmKV `json:"bearer"`
	Basic  []pmKV `json:"basic"`
	APIKey []pmKV `json:"apikey"`
}

type pmItem struct {
	Name    string     `json:"name"`
	Item    []pmItem   `json:"item"`
	Request *pmRequest `json:"request"`
	Event   []struct {
		Listen string `json:"listen"`
	} `json:"event"`
	Response []struct {
		Body string `json:"body"`
		Code int    `json:"code"`
	} `json:"response"`
}

type pmRequest struct {
	Method      string          `json:"method"`
	Header      []pmKV          `json:"header"`
	URL         json.RawMessage `json:"url"`
	Body        *pmBody         `json:"body"`
	Auth        *pmAuth         `json:"auth"`
	Description any             `json:"description"`
}

type pmBody struct {
	Mode       string `json:"mode"`
	Raw        string `json:"raw"`
	URLEncoded []pmKV `json:"urlencoded"`
	Options    struct {
		Raw struct {
			Language string `json:"language"`
		} `json:"raw"`
	} `json:"options"`
	GraphQL *struct {
		Query string `json:"query"`
	} `json:"graphql"`
}

type pmEnv struct {
	Name   string `json:"name"`
	Values []pmKV `json:"values"`
}

func kvString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func pmGet(list []pmKV, key string) string {
	for _, kv := range list {
		if kv.Key == key {
			return kvString(kv.Value)
		}
	}
	return ""
}

func convertAuth(a *pmAuth) (*store.Auth, []Var) {
	if a == nil {
		return nil, nil
	}
	switch a.Type {
	case "noauth":
		return &store.Auth{Type: "none"}, nil
	case "bearer":
		return &store.Auth{Type: "bearer", Token: secretRef(pmGet(a.Bearer, "token"), "token")}, secretVars(pmGet(a.Bearer, "token"), "token")
	case "basic":
		return &store.Auth{Type: "basic", Username: pmGet(a.Basic, "username"), Password: secretRef(pmGet(a.Basic, "password"), "password")},
			secretVars(pmGet(a.Basic, "password"), "password")
	case "apikey":
		in := "header"
		if pmGet(a.APIKey, "in") == "query" {
			in = "query"
		}
		return &store.Auth{Type: "apikey", Key: pmGet(a.APIKey, "key"), Value: secretRef(pmGet(a.APIKey, "value"), "apiKey"), In: in},
			secretVars(pmGet(a.APIKey, "value"), "apiKey")
	}
	return nil, nil
}

// secretRef keeps {{var}} references; a literal secret becomes {{name}}.
func secretRef(v, name string) string {
	if strings.HasPrefix(v, "{{") || v == "" {
		if v == "" {
			return "{{" + name + "}}"
		}
		return v
	}
	return "{{" + name + "}}"
}

func secretVars(v, name string) []Var {
	if v == "" || strings.HasPrefix(v, "{{") {
		return nil
	}
	return []Var{{Name: name, Value: v, Secret: true, Preview: maskPreview(v)}}
}

// Postman imports a v2.1 collection and optional environment (IMP-3).
func Postman(collection, environment []byte) (*Result, error) {
	var c pmCollection
	if err := json.Unmarshal(collection, &c); err != nil {
		return nil, fmt.Errorf("postman collection: %w", err)
	}
	res := &Result{Kind: "postman", Title: c.Info.Name, Description: kvString(c.Info.Description)}
	res.SuggestName = ClientName(strings.TrimPrefix(c.Info.Name, "Acme "))
	if a, vars := convertAuth(c.Auth); a != nil {
		res.Auth = *a
		res.Vars = append(res.Vars, vars...)
	}
	for _, v := range c.Variable {
		val := kvString(v.Value)
		sec := IsSecretName(v.Key) || v.Type == "secret"
		pv := val
		if sec {
			pv = maskPreview(val)
		}
		res.Vars = append(res.Vars, Var{Name: v.Key, Value: val, Secret: sec, Preview: pv})
		if v.Key == "baseUrl" {
			res.BaseURL = val
		}
	}
	scripts := 0
	var walk func(items []pmItem, folder string)
	walk = func(items []pmItem, folder string) {
		for _, it := range items {
			if it.Request == nil {
				f := it.Name
				if folder != "" {
					f = folder + "/" + it.Name
				}
				walk(it.Item, f)
				continue
			}
			if len(it.Event) > 0 {
				scripts++
			}
			pr := it.Request
			r := store.Request{Name: MethodName(it.Name), Title: it.Name, Folder: folder, Method: strings.ToUpper(pr.Method), BodyType: "none",
				Description: kvString(pr.Description)}
			if r.Method == "" {
				r.Method = "GET"
			}
			r.URL = pmURL(pr.URL)
			for _, h := range pr.Header {
				if IsSecretName(h.Key) {
					continue
				}
				r.Headers = append(r.Headers, store.KV{Key: h.Key, Value: kvString(h.Value), Disabled: h.Disabled})
			}
			if pr.Body != nil {
				switch pr.Body.Mode {
				case "raw":
					r.Body = pr.Body.Raw
					switch pr.Body.Options.Raw.Language {
					case "json":
						r.BodyType = "json"
					case "xml":
						r.BodyType = "xml"
					default:
						r.BodyType = "text"
						if json.Valid([]byte(r.Body)) {
							r.BodyType = "json"
						}
					}
				case "urlencoded":
					var parts []string
					for _, kv := range pr.Body.URLEncoded {
						parts = append(parts, kv.Key+"="+kvString(kv.Value))
					}
					r.Body, r.BodyType = strings.Join(parts, "&"), "form"
				case "graphql":
					if pr.Body.GraphQL != nil {
						b, _ := json.Marshal(map[string]string{"query": pr.Body.GraphQL.Query})
						r.Body, r.BodyType = string(b), "json"
					}
				}
			}
			if a, vars := convertAuth(pr.Auth); a != nil {
				r.Auth = a
				res.Vars = append(res.Vars, vars...)
			}
			for _, resp := range it.Response {
				if resp.Code < 300 && json.Valid([]byte(resp.Body)) {
					r.Example = json.RawMessage(resp.Body)
					break
				}
			}
			Classify(&r)
			res.Requests = append(res.Requests, r)
		}
	}
	walk(c.Item, "")
	uniqueNames(res.Requests)
	if scripts > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("%d requests have scripts; mcpDeck v1 doesn't run Postman scripts, so they were left out", scripts))
	}
	if len(environment) > 0 {
		var e pmEnv
		if err := json.Unmarshal(environment, &e); err != nil {
			return nil, fmt.Errorf("postman environment: %w", err)
		}
		name := e.Name
		if name == "" {
			name = "Default"
		}
		res.Environments = append(res.Environments, name)
		for _, v := range e.Values {
			if v.Enabled != nil && !*v.Enabled {
				continue
			}
			val := kvString(v.Value)
			sec := v.Type == "secret" || IsSecretName(v.Key)
			pv := val
			if sec {
				pv = maskPreview(val)
			}
			res.Vars = append(res.Vars, Var{Name: v.Key, Value: val, Secret: sec, Env: name, Preview: pv})
			if v.Key == "baseUrl" && res.BaseURL == "" {
				res.BaseURL = val
			}
		}
	}
	return res, nil
}

func pmURL(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var u struct {
		Raw string `json:"raw"`
	}
	_ = json.Unmarshal(raw, &u)
	return u.Raw
}

// IsSecretName reports whether a header, variable or query key holds a credential.
func IsSecretName(k string) bool {
	l := strings.ToLower(k)
	for _, s := range []string{"authorization", "cookie", "x-api-key", "api_key", "apikey", "api-key", "token", "secret", "password", "passwd", "access_key"} {
		if strings.Contains(l, s) {
			return true
		}
	}
	return l == "key"
}
