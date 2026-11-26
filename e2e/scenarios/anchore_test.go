package scenarios

import (
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/e2e/mockenv"
)

// Anchore Enterprise: Swagger 2.0 with a relative basePath, basic auth or an
// OAuth password-grant token, and asynchronous image analysis.

func init() {
	add("onboarding", "anchore Swagger 2.0 relative basePath resolves to the spec host", func(t *testing.T) {
		a := need(t, "anchore")
		want := h.env.URLs["anchore"] + "/v2"
		expect(t, a.v("baseUrl") == want, "baseUrl is %q, want %q", a.v("baseUrl"), want)
	})

	add("auth", "anchore basic auth from the spec's security scheme", func(t *testing.T) {
		need(t, "anchore")
		a, name := one(t, "read", []string{"anchore"}, "Images and their analysis status.", "", nil,
			`  const r = await anchore.listImages({});
  return r.items.map((i: any) => ({ tag: i.image_detail[0].full_tag, status: i.analysis_status }));`)
		out := a.ok(name, map[string]any{}).([]any)
		expect(t, len(out) >= 3, "got %s", asJSON(out))
	})

	add("auth", "anchore OAuth password grant token", func(t *testing.T) {
		h.importURL(t, "anchoreTok", h.env.URLs["anchore"]+"/v2/swagger.json")
		a := h.apiByName(t, "anchoreTok")
		h.setVar(t, a.ID, "baseUrl", h.env.URLs["anchore"]+"/v2", false)
		h.setVar(t, a.ID, "username", mockenv.AnchoreUser, false)
		h.setVar(t, a.ID, "password", mockenv.AnchorePass, true)
		h.setAuth(t, a.ID, map[string]any{"type": "oauth2", "grant": "password", "tokenUrl": h.env.URLs["anchore"] + "/v2/oauth/token",
			"clientId": "anonymous", "username": "{{username}}", "password": "{{password}}", "clientAuth": "body"})
		before := h.env.Count("anchore.token")
		ag, name := one(t, "read", []string{"anchoreTok"}, "Count images.", "", nil, `  return (await anchoreTok.listImages({})).items.length;`)
		out := ag.ok(name, map[string]any{})
		expect(t, num(out) >= 3, "got %v", out)
		expect(t, h.env.Count("anchore.token") > before, "no token was requested; the call used something else")
	})

	add("async", "anchore add image, poll until analyzed, then vulnerabilities", func(t *testing.T) {
		need(t, "anchore")
		k := kit(t, "anchore")
		name := uniq("scan")
		h.saveTool(t, k, toolSrc(name, "Scan an image and summarize its vulnerabilities.", `image: { type: "string" }`, []string{"image"},
			`  const added = await anchore.addImage({ body: { source: { tag: { pull_string: input.image } } } });
  const digest = added.image_digest;
  let status = added.analysis_status;
  for (let i = 0; i < 20 && status !== "analyzed"; i++) {
    await sleep(200);
    status = (await anchore.getImage({ image_digest: digest })).analysis_status;
  }
  if (status !== "analyzed") return { digest, status };
  const v = await anchore.getImageVulnerabilitiesByType({ image_digest: digest, vtype: "os" });
  const bySeverity: Record<string, number> = {};
  for (const x of v.vulnerabilities) bySeverity[x.severity] = (bySeverity[x.severity] || 0) + 1;
  return { digest, status, bySeverity };`))
		a := h.agentFor(t, k, "read_write")
		out := a.ok(name, map[string]any{"image": "registry.acme.example/checkout:2.0." + strings.TrimPrefix(name, "scan_")})
		expect(t, get(out, "status") == "analyzed" && num(get(out, "bySeverity.Critical")) > 0, "got %s", asJSON(out))
	})

	add("async", "anchore read-only polling of an image in analysis", func(t *testing.T) {
		need(t, "anchore")
		// Someone else queued the image; the agent only watches it.
		var digest string
		b := h.builderFor(t, kit(t, "anchore"), "read_write")
		text, _ := b.call("run_api", map[string]any{"request": "anchore.addImage", "args": map[string]any{"body": map[string]any{"source": map[string]any{"tag": map[string]any{"pull_string": "registry.acme.example/poll:" + uniq("v")}}}}})
		if i := strings.Index(text, "sha256:"); i >= 0 {
			digest = text[i : i+71]
		}
		expect(t, digest != "", "could not queue an image: %s", clip(text, 300))
		a, name := one(t, "read", []string{"anchore"}, "Wait for an image's analysis.", `digest: { type: "string" }`, []string{"digest"},
			`  for (let i = 0; i < 20; i++) {
    const r = await anchore.getImage({ image_digest: input.digest });
    if (r.analysis_status === "analyzed") return { polls: i + 1, status: r.analysis_status };
    await sleep(150);
  }
  return { status: "timeout" };`)
		out := a.ok(name, map[string]any{"digest": digest})
		expect(t, get(out, "status") == "analyzed", "got %s", asJSON(out))
	})

	add("errors", "anchore vulnerabilities before analysis explain why", func(t *testing.T) {
		need(t, "anchore")
		a, name := one(t, "read", []string{"anchore"}, "Vulnerabilities for a digest.", `digest: { type: "string" }`, []string{"digest"},
			`  return await anchore.getImageVulnerabilitiesByType({ image_digest: input.digest, vtype: "os" });`)
		a.fails(name, map[string]any{"digest": "sha256:" + strings.Repeat("0", 64)}, "404", "image not found")
	})

	add("shaping", "anchore fixable criticals only, highest score first", func(t *testing.T) {
		need(t, "anchore")
		a, name := one(t, "read", []string{"anchore"}, "Fixable critical vulnerabilities of an image.", `tag: { type: "string" }`, []string{"tag"},
			`  const imgs = await anchore.listImages({});
  const im = imgs.items.find((i: any) => i.image_detail[0].full_tag === input.tag);
  if (!im) return { error: "unknown image" };
  const v = await anchore.getImageVulnerabilitiesByType({ image_digest: im.image_digest, vtype: "os" });
  return v.vulnerabilities.filter((x: any) => x.severity === "Critical" && x.fix !== "None")
    .sort((a: any, b: any) => b.nvd_data[0].cvss_v3.base_score - a.nvd_data[0].cvss_v3.base_score)
    .map((x: any) => ({ cve: x.vuln, pkg: x.package_name, fix: x.fix }));`)
		text, _ := a.call(name, map[string]any{"tag": "docker.io/library/nginx:1.27"})
		out := decodeOut(text)
		arr, _ := out.([]any)
		expect(t, len(arr) > 0 && !strings.Contains(text, "lorem"), "got %s", clip(text, 300))
	})

	add("chaining", "anchore policy check with the required tag", func(t *testing.T) {
		need(t, "anchore")
		a, name := one(t, "read", []string{"anchore"}, "Policy result for an image tag.", `tag: { type: "string" }`, []string{"tag"},
			`  const imgs = await anchore.listImages({});
  const im = imgs.items.find((i: any) => i.image_detail[0].full_tag === input.tag);
  const c = await anchore.getImagePolicyCheckByDigest({ image_digest: im.image_digest, tag: input.tag });
  const ev = c.evaluations[0];
  return { status: ev.status, action: ev.final_action, stops: ev.details.findings.filter((f: any) => f.action === "stop").map((f: any) => f.message) };`)
		out := a.ok(name, map[string]any{"tag": "docker.io/library/nginx:1.27"})
		expect(t, get(out, "status") == "fail" && len(get(out, "stops").([]any)) == 1, "got %s", asJSON(out))
		out = a.ok(name, map[string]any{"tag": "registry.acme.example/payments:1.4.2"})
		expect(t, get(out, "status") == "pass", "got %s", asJSON(out))
	})

	add("errors", "anchore 400 for a missing query parameter is readable", func(t *testing.T) {
		need(t, "anchore")
		a, name := one(t, "read", []string{"anchore"}, "Policy check without a tag.", `digest: { type: "string" }`, []string{"digest"},
			`  return await call("anchore", "GET /images/" + input.digest + "/check", {});`)
		var digest string
		ag, n2 := one(t, "read", []string{"anchore"}, "First image digest.", "", nil, `  return (await anchore.listImages({})).items[0].image_digest;`)
		digest, _ = ag.ok(n2, map[string]any{}).(string)
		a.fails(name, map[string]any{"digest": digest}, "400", "tag is required")
	})
}
