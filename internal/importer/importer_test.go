package importer

import (
	"os"
	"strings"
	"testing"
)

func names(r *Result) map[string]string {
	m := map[string]string{}
	for _, q := range r.Requests {
		m[q.Name] = q.Method + " " + q.URL + " " + q.Access
	}
	return m
}

// REQ: IMP-1
func TestOpenAPI_CRMFixture(t *testing.T) {
	b, _ := os.ReadFile("testdata/acme-crm.openapi.json")
	r, err := Detect(b)
	if err != nil {
		t.Fatal(err)
	}
	n := names(r)
	if n["getAccount"] != "GET {{baseUrl}}/accounts/{id} read" {
		t.Fatalf("getAccount: %q (all %v)", n["getAccount"], n)
	}
	if !strings.HasSuffix(n["updateAccount"], "write") {
		t.Fatalf("updateAccount should write: %v", n)
	}
	if r.Auth.Type != "bearer" || r.BaseURL != "http://127.0.0.1:9001" {
		t.Fatalf("auth/base: %+v %s", r.Auth, r.BaseURL)
	}
}

// REQ: IMP-1
func TestOpenAPI_Swagger2YAML(t *testing.T) {
	spec := `swagger: "2.0"
info: {title: Pets, version: "1"}
host: petstore.example.com
basePath: /v2
schemes: [https]
paths:
  /pet/{petId}:
    get:
      operationId: getPetById
      parameters: [{name: petId, in: path, required: true, type: integer}]
      responses: {"200": {description: ok}}
  /pet:
    post:
      parameters: [{name: body, in: body, schema: {type: object}}]
      responses: {"200": {description: ok}}
`
	r, err := Detect([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	n := names(r)
	if n["getPetById"] != "GET {{baseUrl}}/pet/{petId} read" || n["postPet"] == "" {
		t.Fatalf("got %v", n)
	}
	if r.BaseURL != "https://petstore.example.com/v2" {
		t.Fatalf("base %q", r.BaseURL)
	}
}

// REQ: IMP-3
func TestPostman_CollectionAndEnvironment(t *testing.T) {
	c, _ := os.ReadFile("testdata/acme-support.postman_collection.json")
	e, _ := os.ReadFile("testdata/acme-support.postman_environment.json")
	r, err := Postman(c, e)
	if err != nil {
		t.Fatal(err)
	}
	n := names(r)
	if n["searchIssues"] == "" || n["assignIssue"] == "" {
		t.Fatalf("got %v", n)
	}
	for _, q := range r.Requests {
		if q.Name == "searchIssues" && !q.AccessFlag {
			t.Fatal("POST search should be flagged check access")
		}
	}
	var secret bool
	for _, v := range r.Vars {
		if v.Secret && v.Env != "" {
			secret = true
			if strings.Contains(v.Preview, v.Value) && len(v.Value) > 4 {
				t.Fatal("preview must not show the secret")
			}
		}
	}
	if !secret {
		t.Fatalf("environment token should be a secret: %+v", r.Vars)
	}
}

// REQ: IMP-4
func TestCurl_MovesCredentialsOut(t *testing.T) {
	in := `curl -s 'https://api.acme.example/v1/invoices?status=overdue&api_key=k-123456' \
  -H 'Accept: application/json'
curl -X POST https://api.acme.example/v1/invoices/7/remind -H "Authorization: Bearer tok-abcdef" -d '{"channel":"email"}'`
	r, err := Detect([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Requests) != 2 {
		t.Fatalf("want 2 requests, got %d", len(r.Requests))
	}
	for _, q := range r.Requests {
		all := q.URL
		for _, kv := range append(q.Headers, q.Query...) {
			all += kv.Key + kv.Value
		}
		if strings.Contains(all, "k-123456") || strings.Contains(all, "tok-abcdef") {
			t.Fatalf("credential left on request: %+v", q)
		}
	}
	if r.Requests[1].URL != "{{baseUrl}}/v1/invoices/{invoiceId}/remind" || r.Requests[1].Name != "postV1InvoicesRemind" {
		t.Fatalf("ID segments should become params: %s %s", r.Requests[1].URL, r.Requests[1].Name)
	}
	if r.Requests[1].BodyType != "json" || r.Requests[1].Access != "write" {
		t.Fatalf("second request: %+v", r.Requests[1])
	}
}

// REQ: IMP-2
func TestFindSpecURL(t *testing.T) {
	html := `<script>const ui = SwaggerUIBundle({ url: "/v3/openapi.json", dom_id: '#swagger-ui' })</script>`
	if got, _ := FindSpecURL("https://petstore3.swagger.io/", []byte(html)); got != "https://petstore3.swagger.io/v3/openapi.json" {
		t.Fatalf("got %q", got)
	}
	if got, _ := FindSpecURL("https://x.example/docs", []byte(`<redoc spec-url="api.yaml"></redoc>`)); got != "https://x.example/api.yaml" {
		t.Fatalf("redoc: %q", got)
	}
}
