package importer

import (
	"testing"
)

func TestCurl_MixedAuthStaysPerRequest(t *testing.T) {
	res, err := Curl(`curl -u admin:s3cret -X POST "https://art.example/access/api/v1/tokens" -d "username=admin"
curl -H "Authorization: Bearer abc" "https://art.example/artifactory/api/repositories"
curl -H "Authorization: Bearer abc" "https://art.example/artifactory/api/storage/x"
curl "https://art.example/api/customers/" -H "Cookie: sessionid=abc"`)
	if err != nil {
		t.Fatal(err)
	}
	if res.Auth.Type != "bearer" {
		t.Fatalf("API auth should be the common bearer, got %+v", res.Auth)
	}
	tok := res.Requests[0]
	if tok.Auth == nil || tok.Auth.Type != "basic" || tok.Auth.Username != "admin" {
		t.Fatalf("token request keeps its basic auth, got %+v", tok.Auth)
	}
	for _, r := range res.Requests {
		for _, h := range r.Headers {
			if h.Key == "Cookie" {
				t.Fatalf("example cookie kept on %s", r.Name)
			}
		}
	}
	if len(res.Warnings) == 0 {
		t.Fatal("dropping the cookie should be explained")
	}
}

func TestOpenAPI_OAuth2ClientCredentialsAndRelativeServer(t *testing.T) {
	spec := `{"openapi":"3.0.3","info":{"title":"Billing","version":"1"},
"servers":[{"url":"/{ver}","variables":{"ver":{"default":"v1"}}}],
"security":[{"oauth":["invoices:read"]}],
"components":{"securitySchemes":{"apiKey":{"type":"apiKey","in":"header","name":"X-Key"},
 "oauth":{"type":"oauth2","flows":{"clientCredentials":{"tokenUrl":"https://idp.example/token","scopes":{"invoices:read":""}}}}}},
"paths":{"/invoices":{"post":{"operationId":"search","requestBody":{"content":{"text/plain":{"example":"status=open"}}},"responses":{"200":{"description":"ok"}}}}}}`
	res, err := OpenAPI([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if res.Auth.Type != "oauth2" || res.Auth.TokenURL != "https://idp.example/token" || res.Auth.Grant != "client_credentials" {
		t.Fatalf("global oauth2 scheme should win: %+v", res.Auth)
	}
	res.ResolveBase("https://billing.example/docs/openapi.json")
	if res.BaseURL != "https://billing.example/v1" {
		t.Fatalf("relative server not resolved: %q", res.BaseURL)
	}
	r := res.Requests[0]
	if r.BodyType != "text" || r.Body != "status=open" {
		t.Fatalf("text/plain body not imported: %q %q", r.BodyType, r.Body)
	}
}

func TestClassify_GraphQLQueryReads(t *testing.T) {
	res, err := Curl(`curl -X POST https://shop.example/graphql -H "Content-Type: application/json" -d '{"query":"query { orders { id } }"}'`)
	if err != nil {
		t.Fatal(err)
	}
	if r := res.Requests[0]; r.Access != "read" || r.AccessFlag {
		t.Fatalf("GraphQL query POST should read: %+v", r)
	}
}
