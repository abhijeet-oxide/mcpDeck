package scenarios

import (
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/e2e/mockenv"
)

// JFrog Artifactory and Xray, onboarded from the curl examples in the JFrog
// REST API docs, with a scoped access token from the Access API.

const aqlLatest = `items.find({"repo":"libs-release-local","name":{"$match":"payments-*.jar"}}).include("name","repo","path","size","created").sort({"$desc":["created"]}).limit(500)`

func init() {
	add("onboarding", "jfrog curl import moves the -u password into a secret", func(t *testing.T) {
		a := need(t, "jfrog")
		expect(t, len(a.Requests) >= 7, "want 7 requests from 7 curl commands, got %d", len(a.Requests))
		var raw map[string]any
		h.mustAPI(t, "GET", "/api/apis/"+a.ID, nil, &raw)
		h.noLeaks(t, "the imported API", asJSON(raw))
	})

	add("auth", "jfrog access token from basic auth feeds Artifactory calls", func(t *testing.T) {
		need(t, "jfrog")
		a, name := one(t, "read", []string{"jfrog"}, "Local repositories.", "", nil,
			`  const r = await call("jfrog", "GET /artifactory/api/repositories", { type: "local" });
  return r.map((x: any) => x.key);`)
		out := a.ok(name, map[string]any{})
		expect(t, len(out.([]any)) == 3, "got %s", asJSON(out))
		h.noLeaksInCallLog(t)
	})

	add("auth", "jfrog legacy X-JFrog-Art-Api key header", func(t *testing.T) {
		var created struct{ ID string }
		h.mustAPI(t, "POST", "/api/apis", map[string]any{"name": "jfrogKey", "title": "Artifactory (API key)", "baseUrl": h.env.URLs["jfrog"],
			"auth": map[string]any{"type": "apikey", "key": "X-JFrog-Art-Api", "value": "{{apiKey}}", "in": "header"}}, &created)
		h.patchAPI(t, created.ID, map[string]any{"tls": map[string]any{"caCerts": string(h.env.CACert)}})
		h.setVar(t, created.ID, "apiKey", mockenv.JFrogAPIKey, true)
		h.importText(t, "jfrogKey", `curl "`+h.env.URLs["jfrog"]+`/artifactory/api/system/ping"`)
		a, name := one(t, "read", []string{"jfrogKey"}, "Ping Artifactory.", "", nil, `  return await call("jfrogKey", "GET /artifactory/api/system/ping", {});`)
		out := a.ok(name, map[string]any{})
		expect(t, out == "OK", "got %v", out)
	})

	add("payloads", "jfrog AQL is sent as text/plain", func(t *testing.T) {
		need(t, "jfrog")
		a, name := one(t, "read", []string{"jfrog"}, "Search artifacts with AQL.", "", nil,
			"  const r = await call(\"jfrog\", \"POST /artifactory/api/search/aql\", { body: `"+aqlLatest+"` });\n  return { n: r.results.length, newest: r.results[0].name };")
		out := a.ok(name, map[string]any{})
		expect(t, num(get(out, "n")) == 37, "got %s", asJSON(out))
	})

	add("pagination", "jfrog AQL offset and limit until a short page", func(t *testing.T) {
		need(t, "jfrog")
		a, name := one(t, "read", []string{"jfrog"}, "Every artifact name in a repo.", `repo: { type: "string" }`, []string{"repo"},
			"  const names: string[] = [];\n  for (let off = 0; ; off += 10) {\n    const q = `items.find({\"repo\":\"${input.repo}\"}).include(\"name\").offset(${off}).limit(10)`;\n    const r = await call(\"jfrog\", \"POST /artifactory/api/search/aql\", { body: q });\n    names.push(...r.results.map((x: any) => x.name));\n    if (r.results.length < 10) break;\n  }\n  return { count: names.length };")
		out := a.ok(name, map[string]any{"repo": "libs-release-local"})
		expect(t, num(get(out, "count")) == 37, "got %s", asJSON(out))
	})

	add("chaining", "jfrog walk the storage tree to the newest version", func(t *testing.T) {
		need(t, "jfrog")
		a, name := one(t, "read", []string{"jfrog"}, "Versions under a folder.", "", nil,
			`  const r = await call("jfrog", "GET /artifactory/api/storage/libs-release-local/com/acme/payments", {});
  const versions = r.children.filter((c: any) => c.folder).map((c: any) => c.uri.slice(1));
  const last = versions.sort().pop();
  const files = await call("jfrog", "GET /artifactory/api/storage/libs-release-local/com/acme/payments/" + last, {});
  return { versions: versions.length + 1, last, file: files.children[0].uri };`)
		out := a.ok(name, map[string]any{})
		expect(t, num(get(out, "versions")) == 37 && strings.HasPrefix(asJSON(get(out, "file")), `"/payments-`), "got %s", asJSON(out))
	})

	add("chaining", "jfrog AQL to storage checksum to Xray summary", func(t *testing.T) {
		need(t, "jfrog")
		a, name := one(t, "read", []string{"jfrog"}, "Security issues in the newest payments jar.", "", nil,
			"  const r = await call(\"jfrog\", \"POST /artifactory/api/search/aql\", { body: `"+aqlLatest+"` });\n"+`  const top = r.results[0];
  const info = await call("jfrog", "GET /artifactory/api/storage/" + top.repo + "/" + top.path + "/" + top.name, {});
  const s = await call("jfrog", "POST /xray/api/v1/summary/artifact", { body: { paths: ["default/" + top.repo + "/" + top.path + "/" + top.name] } });
  const issues = s.artifacts[0].issues;
  return { artifact: top.name, sha256: info.checksums.sha256, critical: issues.filter((i: any) => i.severity === "Critical").map((i: any) => i.cves[0].cve) };`)
		out := a.ok(name, map[string]any{})
		expect(t, len(asJSON(get(out, "sha256"))) == 66 && len(get(out, "critical").([]any)) == 1, "got %s", asJSON(out))
	})

	add("pagination", "xray violations offset is a page number", func(t *testing.T) {
		need(t, "jfrog")
		a, name := one(t, "read", []string{"jfrog"}, "All violations for a watch.", "", nil,
			`  const all = [];
  for (let page = 1; ; page++) {
    const r = await call("jfrog", "POST /xray/api/v1/violations", { body: { filters: { watch_name: "prod-releases" }, pagination: { limit: 10, offset: page } } });
    all.push(...r.violations.map((v: any) => v.issue_id));
    if (all.length >= r.total_violations || r.violations.length === 0) break;
  }
  return { count: all.length, unique: new Set(all).size };`)
		out := a.ok(name, map[string]any{})
		expect(t, num(get(out, "count")) == 23 && num(get(out, "unique")) == 23, "got %s", asJSON(out))
	})

	add("shaping", "xray violations filtered and trimmed", func(t *testing.T) {
		need(t, "jfrog")
		a, name := one(t, "read", []string{"jfrog"}, "High and critical violations, short.", "", nil,
			`  const r = await call("jfrog", "POST /xray/api/v1/violations", { body: { filters: { min_severity: "High" }, pagination: { limit: 100, offset: 1 } } });
  return { total: r.total_violations, items: pick(r.violations, ["issue_id", "severity", "impacted_artifacts"]) };`)
		text, _ := a.call(name, map[string]any{})
		out := decodeOut(text)
		expect(t, num(get(out, "total")) == 15 && !strings.Contains(text, "lorem"), "got %s", clip(text, 300))
	})

	add("shaping", "jfrog build info cut to what matters", func(t *testing.T) {
		need(t, "jfrog")
		a, name := one(t, "read", []string{"jfrog"}, "A build's commit and dependency count.", `name: { type: "string" }, number: { type: "string" }`, []string{"name", "number"},
			`  const r = await call("jfrog", "GET /artifactory/api/build/" + input.name + "/" + input.number, {});
  const b = r.buildInfo;
  return { commit: b.vcs[0].revision, branch: b.vcs[0].branch, deps: b.modules[0].dependencies.length };`)
		out := a.ok(name, map[string]any{"name": "payments-ci", "number": "42"})
		expect(t, get(out, "commit") == "9f1c2ab7e0d4" && num(get(out, "deps")) == 30, "got %s", asJSON(out))
	})

	add("payloads", "jfrog binary download reaches the tool as metadata", func(t *testing.T) {
		need(t, "jfrog")
		a, name := one(t, "read", []string{"jfrog"}, "Download facts for an artifact.", "", nil,
			`  const r = await call("jfrog", "GET /artifactory/libs-release-local/com/acme/payments/1.0.0/payments-1.0.0.jar", {});
  return r;`)
		text, isErr := a.call(name, map[string]any{})
		expect(t, !isErr, "error: %s", clip(text, 200))
		expect(t, !strings.Contains(text, "PK\u0003\u0004") && !strings.Contains(text, "\ufffd"), "raw binary reached the agent: %q", clip(text, 120))
		expect(t, strings.Contains(text, "application/java-archive") && strings.Contains(text, "2056"), "want content type and size, got %s", clip(text, 300))
	})

	add("access", "jfrog delete is blocked for a read agent", func(t *testing.T) {
		need(t, "jfrog")
		before := h.env.Count("jfrog.delete")
		k := kit(t, "jfrog")
		name := uniq("rm")
		var res saveRes
		_ = h.api("POST", "/api/toolkits/"+k+"/save", map[string]any{"source": toolSrc(name, "Delete an artifact.", "", nil,
			`  return await call("jfrog", "DELETE /artifactory/libs-snapshot-local/com/acme/ledger/2.0.0-SNAPSHOT/ledger-2.0.0-SNAPSHOT.jar", {});`), "tests": []any{}}, &res)
		expect(t, res.Version.Access != "read", "a DELETE tool was saved as read")
		a := h.agentFor(t, k, "read")
		for _, n := range a.tools() {
			if n == name {
				_, _ = a.call(name, map[string]any{})
			}
		}
		expect(t, h.env.Count("jfrog.delete") == before, "an artifact was deleted")
	})

	add("errors", "jfrog 404 message is passed through", func(t *testing.T) {
		need(t, "jfrog")
		a, name := one(t, "read", []string{"jfrog"}, "Storage info for a path.", `path: { type: "string" }`, []string{"path"},
			`  return await call("jfrog", "GET /artifactory/api/storage/libs-release-local/" + input.path, {});`)
		a.fails(name, map[string]any{"path": "no/such/thing"}, "404", "Unable to find item")
	})

	add("onboarding", "jfrog AQL request keeps its text/plain body", func(t *testing.T) {
		a := need(t, "jfrog")
		for _, r := range a.Requests {
			if strings.Contains(r.URL, "/search/aql") {
				expect(t, strings.Contains(r.Body, "items.find("), "the AQL body was dropped: %q (%s)", r.Body, r.BodyType)
				return
			}
		}
		t.Fatal("no AQL request")
	})
}
