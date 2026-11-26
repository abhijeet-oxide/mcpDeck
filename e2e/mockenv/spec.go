package mockenv

import (
	"encoding/json"
	"net/http"
	"strings"
)

// A tiny builder for the OpenAPI documents the mocks publish, so each
// system's spec stays next to its handlers and in sync with them.

type op struct {
	method, path, id, summary, tag string
	params                         []param
	body                           *body
	resp                           any    // example 200 response
	respCT                         string // default application/json
	security                       []string
	noSecurity                     bool
}

type param struct {
	name, in string
	required bool
	example  any
	typ      string
}

type body struct {
	ct      string
	example any
	schema  map[string]any
}

func q(name string, example any) param { return param{name: name, in: "query", example: example} }
func qr(name string, example any) param {
	return param{name: name, in: "query", example: example, required: true}
}
func pp(name string) param { return param{name: name, in: "path", required: true} }
func hp(name string, example any) param {
	return param{name: name, in: "header", example: example, required: true}
}

func jsonBody(example any) *body { return &body{ct: "application/json", example: example} }

type specDoc struct {
	title    string
	desc     string
	server   string // absolute or relative
	schemes  map[string]any
	security []string
	ops      []op
}

func (d specDoc) v3() map[string]any {
	paths := map[string]any{}
	for _, o := range d.ops {
		item, _ := paths[o.path].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[o.path] = item
		}
		x := map[string]any{"operationId": o.id, "summary": o.summary}
		if o.tag != "" {
			x["tags"] = []string{o.tag}
		}
		var ps []any
		for _, p := range o.params {
			typ := p.typ
			if typ == "" {
				typ = "string"
				switch p.example.(type) {
				case int, float64:
					typ = "integer"
				case bool:
					typ = "boolean"
				}
			}
			m := map[string]any{"name": p.name, "in": p.in, "required": p.required, "schema": map[string]any{"type": typ}}
			if p.example != nil {
				m["example"] = p.example
			}
			ps = append(ps, m)
		}
		if len(ps) > 0 {
			x["parameters"] = ps
		}
		if o.body != nil {
			mt := map[string]any{}
			if o.body.example != nil {
				mt["example"] = o.body.example
			}
			if o.body.schema != nil {
				mt["schema"] = o.body.schema
			} else {
				mt["schema"] = map[string]any{"type": "object"}
				if o.body.ct == "text/plain" || strings.Contains(o.body.ct, "xml") {
					mt["schema"] = map[string]any{"type": "string"}
				}
			}
			x["requestBody"] = map[string]any{"required": true, "content": map[string]any{o.body.ct: mt}}
		}
		ct := o.respCT
		if ct == "" {
			ct = "application/json"
		}
		r200 := map[string]any{"description": "OK"}
		if o.resp != nil {
			r200["content"] = map[string]any{ct: map[string]any{"example": o.resp}}
		}
		x["responses"] = map[string]any{"200": r200, "401": map[string]any{"description": "Unauthorized"}}
		if o.noSecurity {
			x["security"] = []any{}
		} else if len(o.security) > 0 {
			x["security"] = []any{map[string]any{o.security[0]: []any{}}}
		}
		item[strings.ToLower(o.method)] = x
	}
	doc := map[string]any{
		"openapi": "3.0.3",
		"info":    map[string]any{"title": d.title, "version": "1.0.0", "description": d.desc},
		"servers": []any{map[string]any{"url": d.server}},
		"paths":   paths,
	}
	if len(d.schemes) > 0 {
		doc["components"] = map[string]any{"securitySchemes": d.schemes}
	}
	if len(d.security) > 0 {
		var sec []any
		for _, s := range d.security {
			sec = append(sec, map[string]any{s: []any{}})
		}
		doc["security"] = sec
	}
	return doc
}

// v2 renders Swagger 2.0, as older products (Anchore, Kubernetes) publish.
func (d specDoc) v2() map[string]any {
	paths := map[string]any{}
	for _, o := range d.ops {
		item, _ := paths[o.path].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[o.path] = item
		}
		x := map[string]any{"operationId": o.id, "summary": o.summary, "produces": []string{"application/json"}}
		if o.tag != "" {
			x["tags"] = []string{o.tag}
		}
		var ps []any
		for _, p := range o.params {
			typ := p.typ
			if typ == "" {
				typ = "string"
				if _, ok := p.example.(int); ok {
					typ = "integer"
				}
			}
			ps = append(ps, map[string]any{"name": p.name, "in": p.in, "required": p.required, "type": typ})
		}
		if o.body != nil {
			x["consumes"] = []string{o.body.ct}
			sch := map[string]any{"type": "object"}
			if o.body.example != nil {
				sch["example"] = o.body.example
			}
			ps = append(ps, map[string]any{"name": "body", "in": "body", "required": true, "schema": sch})
		}
		if len(ps) > 0 {
			x["parameters"] = ps
		}
		r := map[string]any{"description": "OK"}
		if o.resp != nil {
			r["examples"] = map[string]any{"application/json": o.resp}
		}
		x["responses"] = map[string]any{"200": r}
		item[strings.ToLower(o.method)] = x
	}
	doc := map[string]any{"swagger": "2.0", "info": map[string]any{"title": d.title, "version": "1.0.0", "description": d.desc},
		"basePath": d.server, "paths": paths}
	if len(d.schemes) > 0 {
		doc["securityDefinitions"] = d.schemes
	}
	if len(d.security) > 0 {
		var sec []any
		for _, s := range d.security {
			sec = append(sec, map[string]any{s: []any{}})
		}
		doc["security"] = sec
	}
	return doc
}

func serveSpec(mux *http.ServeMux, path string, build func(r *http.Request) map[string]any) {
	mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.MarshalIndent(build(r), "", "  ")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	})
}

// origin is the scheme and host the request came in on.
func origin(r *http.Request) string {
	s := "http"
	if r.TLS != nil {
		s = "https"
	}
	return s + "://" + r.Host
}
