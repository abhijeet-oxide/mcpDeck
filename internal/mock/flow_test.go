package mock

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/vault"
)

// The flow the builder produces: four chained requests, output trimmed.
const scanFlow = `export const tool = {
  name: "find_image_vulnerabilities",
  description: "Lists the fixable vulnerabilities of an application's newest image, with CVE, severity and fixed version.",
  useWhen: "A question is about security issues in one application",
  inputSchema: {
    type: "object",
    properties: { app_name: { type: "string", description: "Application name, like checkout" } },
    required: ["app_name"],
  },
};

export default async function (input: Input) {
  const login = await scanner.login();
  const apps = await scanner.listApplications({ name: input.app_name, headers: { Authorization: ` + "`Bearer ${login.access_token}`" + ` } });
  const images = await scanner.listImages({ appId: apps.items[0].id, headers: { Authorization: ` + "`Bearer ${login.access_token}`" + ` } });
  const vulns = await scanner.listVulnerabilities({ imageId: images.items[0].id, headers: { Authorization: ` + "`Bearer ${login.access_token}`" + ` } });
  return pick(vulns, ["total", "items[].cve", "items[].severity", "items[].fixedVersion"]);
}
`

// REQ: MOCK-6, API-9, EX-3, FLOW-1, FLOW-3, GUIDE-3
func TestScannerFlow_EndToEnd(t *testing.T) {
	ctx := context.Background()
	w, err := Start(0)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s, _ := store.Open(ctx, t.TempDir())
	defer s.Close()
	v, _ := vault.Open(vault.NoKeychain{}, t.TempDir())
	c := clock.Real{}
	ids := clock.ULIDs{Clock: c}
	d := deck.New(s, v, proxy.New(s, v, c, ids, nil), audit.New(s, c, ids), c, ids, t.TempDir(), 47800)
	spec := w.URL("scanner") + "/openapi.json"

	// 1. The self-signed certificate stops the import, with what to do.
	if _, err := d.PreviewImport(ctx, deck.ImportSource{URL: spec}); err == nil || !strings.Contains(err.Error(), "Skip certificate check") {
		t.Fatalf("want a certificate error, got %v", err)
	}
	rep, err := d.Import(ctx, deck.ImportInput{ImportSource: deck.ImportSource{URL: spec, Insecure: true}, Name: "scanner"})
	if err != nil {
		t.Fatal(err)
	}
	api, _ := d.GetAPI(ctx, rep.APIID)
	if !api.TLS.Insecure || len(api.Requests) != 4 {
		t.Fatalf("api: tls %+v, %d requests", api.TLS, len(api.Requests))
	}

	// 2. Credentials live in the environment; the login body refers to them.
	_, _ = d.SetVariable(ctx, api.ID, deck.SetVariableInput{EnvID: api.ActiveEnvID, Name: "username", Source: "literal", Value: ScannerUser})
	_, _ = d.SetVariable(ctx, api.ID, deck.SetVariableInput{EnvID: api.ActiveEnvID, Name: "password", Source: "secret", Value: ScannerPass})
	byName := map[string]store.Request{}
	for _, r := range api.Requests {
		byName[r.Name] = r
	}
	login := byName["login"]
	login.BodyType, login.Body = "json", `{"username":"{{username}}","password":"{{password}}"}`
	if _, err := d.SaveRequest(ctx, api.ID, login); err != nil {
		t.Fatal(err)
	}

	// 3. Run each request once and save the responses (a 200 and a 401).
	res, _ := d.Send(ctx, deck.SendInput{APIID: api.ID, RequestID: login.ID})
	if res.Error != "" || res.Status != 200 {
		t.Fatalf("login: %+v", res)
	}
	if _, err := d.SaveResponse(ctx, login.ID, deck.SaveResponseInput{Status: res.Status, Body: json.RawMessage(res.Body)}); err != nil {
		t.Fatal(err)
	}
	bad := login
	bad.Body = `{"username":"x","password":"y"}`
	res401, _ := d.Send(ctx, deck.SendInput{APIID: api.ID, Draft: &bad})
	if res401.Status != 401 {
		t.Fatalf("want 401, got %+v", res401)
	}
	got, _ := d.SaveResponse(ctx, login.ID, deck.SaveResponseInput{Status: 401, Body: json.RawMessage(res401.Body)})
	if len(got.Responses) != 2 || strings.Contains(string(got.Example), "scn_") {
		t.Fatalf("responses %+v, example %s", got.Responses, got.Example)
	}

	// 4. The chained tool runs, and returns a fraction of what it fetched.
	name, desc := "security", "Security questions about our applications"
	tk, err := d.CreateToolkit(ctx, deck.ToolkitInput{Name: &name, Description: &desc})
	if err != nil {
		t.Fatal(err)
	}
	r, _ := d.TestTool(ctx, deck.Caller{Actor: audit.You, Toolkit: tk}, deck.TestInput{Source: scanFlow, Input: map[string]any{"app_name": "checkout"}})
	if r.Error != "" || len(r.Calls) != 4 || !strings.Contains(r.OutputJSON, `"cve":"CVE-2026-`) {
		t.Fatalf("flow: %s %s calls=%d", r.Error, r.OutputJSON, len(r.Calls))
	}
	if strings.Contains(r.OutputJSON, "description") || r.UpstreamBytes < r.OutputBytes*5 {
		t.Fatalf("pick should trim: out %d vs upstream %d", r.OutputBytes, r.UpstreamBytes)
	}

	// 5. The guidelines see the raw vulnerability list as too big, and the
	// trimmed output as fine on size.
	vr := byName["listVulnerabilities"]
	vr.URL = strings.ReplaceAll(vr.URL, "{imageId}", "img-7d41a0")
	vr.Headers = []store.KV{{Key: "Authorization", Value: "Bearer " + scannerTok}}
	raw, _ := d.Send(ctx, deck.SendInput{APIID: api.ID, Draft: &vr})
	lr, err := d.LintOutput(json.RawMessage(raw.Body))
	if err != nil {
		t.Fatalf("lint output: %v (%s)", err, raw.Error)
	}
	rules := map[string]bool{}
	for _, f := range lr.Findings {
		rules[f.Rule] = true
	}
	if !rules["output-tokens"] || !rules["unbounded-list"] || !rules["blob"] {
		t.Fatalf("raw findings %+v", lr.Findings)
	}
	lint, err := d.LintToolkit(ctx, "security", deck.LintInput{Draft: scanFlow})
	if err != nil || lint.DraftError != "" {
		t.Fatalf("lint: %v %s", err, lint.DraftError)
	}
	for _, f := range lint.Draft {
		if f.Level == "must" {
			t.Errorf("the finished flow has a must-fix: %+v", f)
		}
	}
}
