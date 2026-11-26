package scenarios

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/e2e/mockenv"
)

// Each system is onboarded once, the way its docs lead a person to: import
// what the vendor publishes (an OpenAPI or Swagger document, curl examples,
// a Postman collection), store the credentials, and set up auth. Scenarios
// that use a system call need(t, name) first.

var (
	onceMu sync.Mutex
	once   = map[string]*onboarding{}
)

type onboarding struct {
	done chan struct{}
	err  string
	api  apiDetail
}

// need onboards a system (once) and returns its API; a failed onboarding
// fails every scenario that depends on it, with the reason.
func need(t *testing.T, system string) apiDetail {
	t.Helper()
	onceMu.Lock()
	o, ok := once[system]
	if !ok {
		o = &onboarding{done: make(chan struct{})}
		once[system] = o
		onceMu.Unlock()
		func() {
			defer close(o.done)
			sub := &recorder{T: t}
			func() {
				defer func() {
					if r := recover(); r != nil && r != errStop {
						panic(r)
					}
				}()
				onboard[system](sub)
			}()
			if sub.failed {
				o.err = sub.msg
				return
			}
			o.api = h.apiByName(t, system)
		}()
	} else {
		onceMu.Unlock()
		<-o.done
	}
	if o.err != "" {
		t.Fatalf("onboarding %s failed: %s", system, o.err)
	}
	return h.apiByName(t, system)
}

// recorder runs onboarding steps and keeps the first failure instead of
// failing the scenario that happened to trigger it.
type recorder struct {
	*testing.T
	failed bool
	msg    string
}

var errStop = fmt.Errorf("stop")

func (r *recorder) Fatalf(format string, args ...any) {
	r.failed, r.msg = true, fmt.Sprintf(format, args...)
	panic(errStop)
}

func (r *recorder) Fatal(args ...any) {
	r.failed, r.msg = true, fmt.Sprint(args...)
	panic(errStop)
}

// onboard holds each system's steps. They take *testing.T-compatible
// recorders so the same helpers work.
var onboard = map[string]func(t *recorder){
	"shop": func(t *recorder) {
		h.importURL(t, "shop", h.env.URLs["shop"]+"/openapi.json")
		a := h.apiByName(t, "shop")
		login := a.req(t, "login")
		h.editRequest(t, login.ID, map[string]any{"body": `{"email": "{{email}}", "password": "{{password}}"}`, "bodyType": "json"})
		h.setVar(t, a.ID, "email", mockenv.ShopUser, false)
		h.setVar(t, a.ID, "password", mockenv.ShopPass, true)
		h.setAuth(t, a.ID, map[string]any{"type": "login", "loginRequestId": login.ID, "capture": "json:access_token"})
	},
	"jfrog": func(t *recorder) {
		h.importText(t, "jfrog", h.env.JFrogCurl())
		a := h.apiByName(t, "jfrog")
		h.patchAPI(t, a.ID, map[string]any{"tls": map[string]any{"caCerts": string(h.env.CACert)}})
		var tokenReq reqRow
		for _, r := range a.Requests {
			if strings.Contains(r.URL, "/access/api/v1/tokens") {
				tokenReq = r
			}
		}
		if tokenReq.ID == "" {
			t.Fatalf("the curl import has no token request: %v", a.Requests)
		}
		h.setAuth(t, a.ID, map[string]any{"type": "login", "loginRequestId": tokenReq.ID, "capture": "json:access_token"})
		for _, r := range a.Requests {
			if strings.Contains(r.URL, "/search/aql") || strings.Contains(r.URL, "/xray/api/v1/") {
				h.markRead(t, r.ID, "searches only")
			}
		}
	},
	"anchore": func(t *recorder) {
		h.importURL(t, "anchore", h.env.URLs["anchore"]+"/v2/swagger.json")
		a := h.apiByName(t, "anchore")
		h.setVar(t, a.ID, "username", mockenv.AnchoreUser, false)
		h.setVar(t, a.ID, "password", mockenv.AnchorePass, true)
	},
	"billing": func(t *recorder) {
		h.importURL(t, "billing", h.env.URLs["billing"]+"/openapi.json")
		a := h.apiByName(t, "billing")
		h.patchAPI(t, a.ID, map[string]any{"tls": map[string]any{"caCerts": string(h.env.CACert)}})
		h.setVar(t, a.ID, "clientId", mockenv.OAuthClientID, false)
		h.setVar(t, a.ID, "clientSecret", mockenv.OAuthSecret, true)
		h.setAuth(t, a.ID, map[string]any{"type": "oauth2", "grant": "client_credentials", "tokenUrl": h.env.URLs["idp"] + "/oauth2/token",
			"clientId": "{{clientId}}", "clientSecret": "{{clientSecret}}", "scope": "billing.read"})
	},
	"legacy": func(t *recorder) {
		h.importText(t, "legacy", h.env.LegacyCurl())
		a := h.apiByName(t, "legacy")
		h.patchAPI(t, a.ID, map[string]any{"tls": map[string]any{"caCerts": string(h.env.CACert)}})
		var page, form reqRow
		for _, r := range a.Requests {
			if strings.HasSuffix(r.URL, "/accounts/login/") && r.Method == "GET" {
				page = r
			}
			if strings.HasSuffix(r.URL, "/accounts/login/") && r.Method == "POST" {
				form = r
			}
		}
		if page.ID == "" || form.ID == "" {
			t.Fatalf("curl import lost the login requests")
		}
		h.editRequest(t, form.ID, map[string]any{"body": "csrfmiddlewaretoken={{login.csrf}}&username={{username}}&password={{password}}", "bodyType": "form"})
		for _, r := range a.Requests {
			if strings.HasSuffix(r.URL, "/soap/CreditService") {
				h.markRead(t, r.ID, "GetCreditLimit only reads")
			}
		}
		h.setVar(t, a.ID, "username", mockenv.LegacyUser, false)
		h.setVar(t, a.ID, "password", mockenv.LegacyPass, true)
		h.setAuth(t, a.ID, map[string]any{"type": "login", "steps": []any{
			map[string]any{"requestId": page.ID, "captures": []any{map[string]any{"name": "csrf", "from": "cookie:csrftoken"}}},
			map[string]any{"requestId": form.ID},
		}, "apply": []any{map[string]any{"key": "X-CSRFToken", "value": "{{login.csrf}}"}}})
	},
	"payments": func(t *recorder) {
		h.importURL(t, "payments", h.env.URLs["payments"]+"/openapi.json")
		a := h.apiByName(t, "payments")
		h.setVar(t, a.ID, "keyId", mockenv.HMACKeyID, false)
		h.setVar(t, a.ID, "signingSecret", mockenv.HMACSecret, true)
		h.setAuth(t, a.ID, map[string]any{"type": "hmac", "algorithm": "sha256", "secret": "{{signingSecret}}", "encoding": "hex",
			"stringToSign": "{{method}}\n{{path}}\n{{timestamp}}\n{{bodySha256}}",
			"apply": []any{map[string]any{"key": "X-Key-Id", "value": "{{keyId}}"}, map[string]any{"key": "X-Timestamp", "value": "{{sign.timestamp}}"},
				map[string]any{"key": "X-Signature", "value": "{{sign.signature}}"}}})
	},
	"vault": func(t *recorder) {
		var created struct{ ID string }
		h.mustAPI(t, "POST", "/api/apis", map[string]any{"name": "vault", "title": "Vault", "baseUrl": h.env.URLs["vault"],
			"auth": map[string]any{"type": "apikey", "key": "X-Vault-Token", "value": "{{vaultToken}}", "in": "header"}}, &created)
		h.setVar(t, created.ID, "vaultToken", mockenv.MTLSToken, true)
		h.setVar(t, created.ID, "clientKey", string(h.env.ClientKey), true)
		h.patchAPI(t, created.ID, map[string]any{"tls": map[string]any{"caCerts": string(h.env.CACert), "clientCert": string(h.env.ClientCert), "clientKey": "{{clientKey}}"}})
		h.importText(t, "vault", `curl "`+h.env.URLs["vault"]+`/v1/sys/health"`+"\n\n"+`curl "`+h.env.URLs["vault"]+`/v1/secret/metadata/apps/payments" -H "X-Vault-Token: hvs.example"`)
	},
	"github": func(t *recorder) {
		h.importURL(t, "github", h.env.URLs["github"]+"/openapi.json")
		a := h.apiByName(t, "github")
		h.setVar(t, a.ID, "token", mockenv.GitHubPAT, true)
	},
	"jira": func(t *recorder) {
		h.importURL(t, "jira", h.env.URLs["jira"]+"/openapi.json")
		a := h.apiByName(t, "jira")
		h.setVar(t, a.ID, "username", mockenv.JiraEmail, false)
		h.setVar(t, a.ID, "password", mockenv.JiraToken, true)
	},
	"servicenow": func(t *recorder) {
		h.importText(t, "servicenow", h.env.ServiceNowPostman())
		a := h.apiByName(t, "servicenow")
		h.setVar(t, a.ID, "baseUrl", h.env.URLs["servicenow"], false)
		h.setVar(t, a.ID, "sn_user", mockenv.SNUser, false)
		h.setVar(t, a.ID, "sn_password", mockenv.SNPass, true)
	},
	"kube": func(t *recorder) {
		h.importURL(t, "kube", h.env.URLs["kube"]+"/openapi/v2")
		a := h.apiByName(t, "kube")
		h.patchAPI(t, a.ID, map[string]any{"tls": map[string]any{"caCerts": string(h.env.CACert)}})
		h.setVar(t, a.ID, "token", mockenv.KubeToken, true)
		h.setAuth(t, a.ID, map[string]any{"type": "bearer", "token": "{{token}}"})
	},
	"graphql": func(t *recorder) {
		u := h.env.URLs["graphql"]
		h.importText(t, "graphql", `curl -X POST "`+u+`/graphql" -H "Authorization: Bearer `+mockenv.GraphQLToken+`" -H "Content-Type: application/json" -d '{"query":"query Orders($first: Int, $after: String) { orders(first: $first, after: $after) { edges { cursor node { id name status totalPrice { amount currencyCode } } } pageInfo { hasNextPage endCursor } } }","variables":{"first":10}}'`)
	},
	"flaky": func(t *recorder) {
		h.importURL(t, "flaky", h.env.URLs["flaky"]+"/openapi.json")
		a := h.apiByName(t, "flaky")
		h.setVar(t, a.ID, "apiKey", mockenv.FlakyKey, true)
	},
	"weather": func(t *recorder) {
		h.importURL(t, "weather", h.env.URLs["weather"]+"/openapi.json")
		a := h.apiByName(t, "weather")
		h.setVar(t, a.ID, "apiKey", mockenv.WeatherKey, true)
	},
}

// ───────── helpers for scenarios ─────────

var seq atomic.Int64

// uniq makes a toolkit or tool name unique to one scenario.
func uniq(prefix string) string { return fmt.Sprintf("%s_%d", prefix, seq.Add(1)) }

// kit creates a toolkit for one scenario and returns its name.
func kit(t *testing.T, apis ...string) string {
	t.Helper()
	name := strings.ReplaceAll(uniq("kit"), "_", "-")
	h.toolkit(t, name, apis...)
	return name
}

// toolSrc writes a tool file: a name, one sentence, input properties and the body.
func toolSrc(name, desc, props string, required []string, body string) string {
	req, _ := json.Marshal(required)
	if required == nil {
		req = []byte("[]")
	}
	return fmt.Sprintf(`export const tool = {
  name: %q,
  description: %q,
  inputSchema: { type: "object", properties: { %s }, required: %s },
};

export default async function (input: any) {
%s
}
`, name, desc, props, req, body)
}

// one saves a single tool in a fresh toolkit and connects an agent with access.
func one(t *testing.T, access string, apis []string, desc, props string, required []string, body string) (*agent, string) {
	t.Helper()
	k := kit(t, apis...)
	name := uniq("t")
	h.saveTool(t, k, toolSrc(name, desc, props, required, body))
	return h.agentFor(t, k, access), name
}

var pathRe = regexp.MustCompile(`([^.\[\]]+)|\[(\d+)\]`)

// get reads a path like "items[0].name" from decoded JSON.
func get(v any, path string) any {
	for _, m := range pathRe.FindAllStringSubmatch(path, -1) {
		if m[2] != "" {
			i, _ := strconv.Atoi(m[2])
			arr, ok := v.([]any)
			if !ok || i >= len(arr) {
				return nil
			}
			v = arr[i]
			continue
		}
		obj, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = obj[m[1]]
	}
	return v
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return -1
}

func expect(t *testing.T, cond bool, format string, args ...any) {
	t.Helper()
	if !cond {
		t.Fatalf(format, args...)
	}
}

func asJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
