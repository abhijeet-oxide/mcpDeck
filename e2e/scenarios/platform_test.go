package scenarios

import (
	"strings"
	"testing"
	"time"
)

// Kubernetes, GraphQL and the failure modes of production APIs.

func init() {
	// ── Kubernetes ──
	add("onboarding", "kube Swagger 2.0 basePath / resolves to the spec host", func(t *testing.T) {
		a := need(t, "kube")
		expect(t, strings.TrimSuffix(a.v("baseUrl"), "/") == h.env.URLs["kube"], "baseUrl is %q", a.v("baseUrl"))
	})

	add("pagination", "kube limit and continue token", func(t *testing.T) {
		need(t, "kube")
		a, name := one(t, "read", []string{"kube"}, "Every pod name in a namespace.", `ns: { type: "string" }`, []string{"ns"},
			`  const names: string[] = [];
  let cont: string | undefined;
  do {
    const r = await kube.listCoreV1NamespacedPod(cont ? { namespace: input.ns, limit: 5, continue: cont } : { namespace: input.ns, limit: 5 });
    names.push(...r.items.map((p: any) => p.metadata.name));
    cont = r.metadata.continue;
  } while (cont);
  return { count: names.length };`)
		out := a.ok(name, map[string]any{"ns": "shop"})
		expect(t, num(get(out, "count")) == 23, "got %s", asJSON(out))
	})

	add("shaping", "kube managedFields stripped from pod lists", func(t *testing.T) {
		need(t, "kube")
		a, name := one(t, "read", []string{"kube"}, "Pods that are not ready, short.", `ns: { type: "string" }`, []string{"ns"},
			`  const r = await kube.listCoreV1NamespacedPod({ namespace: input.ns });
  return r.items.filter((p: any) => !p.status.containerStatuses[0].ready)
    .map((p: any) => ({ name: p.metadata.name, restarts: p.status.containerStatuses[0].restartCount, reason: Object.values(p.status.containerStatuses[0].state)[0] }));`)
		text, _ := a.call(name, map[string]any{"ns": "shop"})
		expect(t, !strings.Contains(text, "managedFields") && strings.Contains(text, "CrashLoopBackOff"), "got %s", clip(text, 300))
	})

	add("chaining", "kube crashing pods with the error lines of their logs", func(t *testing.T) {
		need(t, "kube")
		a, name := one(t, "read", []string{"kube"}, "Crashing pods with recent errors.", `ns: { type: "string" }`, []string{"ns"},
			`  const r = await kube.listCoreV1NamespacedPod({ namespace: input.ns });
  const bad = r.items.filter((p: any) => p.status.containerStatuses[0].restartCount > 5);
  const out = [];
  for (const p of bad) {
    const log: string = await kube.readCoreV1NamespacedPodLog({ namespace: input.ns, name: p.metadata.name, tailLines: 100 });
    out.push({ pod: p.metadata.name, errors: log.split("\n").filter((l) => l.includes("ERROR")).length });
  }
  return out;`)
		out := a.ok(name, map[string]any{"ns": "shop"}).([]any)
		expect(t, len(out) == 3 && num(get(out[0], "errors")) == 6, "got %s", asJSON(out))
	})

	add("payloads", "kube pod log is plain text", func(t *testing.T) {
		need(t, "kube")
		a, name := one(t, "read", []string{"kube"}, "Last log lines.", "", nil,
			`  const log = await kube.readCoreV1NamespacedPodLog({ namespace: "shop", name: "cart-x", tailLines: 3 });
  return { type: typeof log, lines: log.trim().split("\n").length };`)
		out := a.ok(name, map[string]any{})
		expect(t, get(out, "type") == "string" && num(get(out, "lines")) == 3, "got %s", asJSON(out))
	})

	add("payloads", "kube PATCH sends application/merge-patch+json", func(t *testing.T) {
		need(t, "kube")
		k := kit(t, "kube")
		name := uniq("scale")
		h.saveTool(t, k, toolSrc(name, "Scale a deployment.", `name: { type: "string" }, replicas: { type: "number" }`, []string{"name", "replicas"},
			`  const r = await kube.patchAppsV1NamespacedDeploymentScale({ namespace: "shop", name: input.name, body: { spec: { replicas: input.replicas } } });
  return { replicas: r.spec.replicas };`))
		before := h.env.Count("kube.scale")
		a := h.agentFor(t, k, "read_write")
		out := a.ok(name, map[string]any{"name": "cart", "replicas": 3})
		expect(t, num(get(out, "replicas")) == 3 && h.env.Count("kube.scale") == before+1, "got %s", asJSON(out))
	})

	add("access", "kube scale hidden from read agents", func(t *testing.T) {
		need(t, "kube")
		k := kit(t, "kube")
		name := uniq("scale")
		h.saveTool(t, k, toolSrc(name, "Scale a deployment.", `name: { type: "string" }`, []string{"name"},
			`  return await kube.patchAppsV1NamespacedDeploymentScale({ namespace: "shop", name: input.name, body: { spec: { replicas: 0 } } });`))
		before := h.env.Count("kube.scale")
		a := h.agentFor(t, k, "read")
		for _, n := range a.tools() {
			if n == name {
				a.fails(name, map[string]any{"name": "search"})
			}
		}
		expect(t, h.env.Count("kube.scale") == before, "scaled by a read agent")
	})

	add("errors", "kube Status 404 message reaches the agent", func(t *testing.T) {
		need(t, "kube")
		k := kit(t, "kube")
		name := uniq("scale")
		h.saveTool(t, k, toolSrc(name, "Scale a deployment.", `name: { type: "string" }`, []string{"name"},
			`  return await kube.patchAppsV1NamespacedDeploymentScale({ namespace: "shop", name: input.name, body: { spec: { replicas: 1 } } });`))
		a := h.agentFor(t, k, "read_write")
		a.fails(name, map[string]any{"name": "nope"}, "404", "not found")
	})

	add("chaining", "kube deployments with unready replicas", func(t *testing.T) {
		need(t, "kube")
		a, name := one(t, "read", []string{"kube"}, "Deployments that aren't fully ready.", "", nil,
			`  const r = await kube.listAppsV1NamespacedDeployment({ namespace: "shop" });
  return r.items.filter((d: any) => d.status.readyReplicas < d.spec.replicas).map((d: any) => d.metadata.name);`)
		out := a.ok(name, map[string]any{})
		expect(t, asJSON(out) == `["checkout"]`, "got %s", asJSON(out))
	})

	// ── GraphQL ──
	add("graphql", "GraphQL query request imported from curl is a read", func(t *testing.T) {
		a := need(t, "graphql")
		expect(t, len(a.Requests) == 1 && a.Requests[0].Access == "read", "a GraphQL query POST should be read access: %s", asJSON(a.Requests))
	})

	add("graphql", "GraphQL cursor pagination with endCursor", func(t *testing.T) {
		need(t, "graphql")
		a, name := one(t, "read", []string{"graphql"}, "Every order name.", "", nil,
			`  const q = "query Orders($first: Int, $after: String) { orders(first: $first, after: $after) { edges { node { name status } } pageInfo { hasNextPage endCursor } } }";
  const names: string[] = [];
  let after: string | null = null;
  for (;;) {
    const r = await call("graphql", "POST /graphql", { body: { query: q, variables: { first: 10, after } } });
    names.push(...r.data.orders.edges.map((e: any) => e.node.name));
    if (!r.data.orders.pageInfo.hasNextPage) break;
    after = r.data.orders.pageInfo.endCursor;
  }
  return { count: names.length };`)
		out := a.ok(name, map[string]any{})
		expect(t, num(get(out, "count")) == 31, "got %s", asJSON(out))
	})

	add("graphql", "GraphQL errors in a 200 body are surfaced", func(t *testing.T) {
		need(t, "graphql")
		a, name := one(t, "read", []string{"graphql"}, "A bad query.", "", nil,
			`  const r = await call("graphql", "POST /graphql", { body: { query: "{ nope }" } });
  if (r.errors) throw new Error(r.errors.map((e: any) => e.message).join("; "));
  return r.data;`)
		a.fails(name, map[string]any{}, "Cannot query field")
	})

	add("graphql", "GraphQL mutation needs write access", func(t *testing.T) {
		need(t, "graphql")
		k := kit(t, "graphql")
		name := uniq("cancel")
		src := toolSrc(name, "Cancel an order.", `id: { type: "string" }`, []string{"id"},
			`  return await call("graphql", "POST /graphql", { body: { query: "mutation Cancel($id: ID!) { cancelOrder(id: $id) { order { id status } } }", variables: { id: input.id } } });`)
		var res saveRes
		h.mustAPI(t, "POST", "/api/toolkits/"+k+"/save", map[string]any{"source": src, "tests": []any{}}, &res)
		expect(t, res.Status == "pending" || res.Version.Access == "read_write" || res.Version.Access == "write", "a mutation tool was saved as read (%s, %s)", res.Status, res.Version.Access)
	})

	add("graphql", "GraphQL query tool is a read", func(t *testing.T) {
		need(t, "graphql")
		k := kit(t, "graphql")
		name := uniq("orders")
		src := toolSrc(name, "First orders.", "", nil,
			`  const r = await call("graphql", "POST /graphql", { body: { query: "query { orders(first: 3) { edges { node { name } } } }" } });
  return r.data.orders.edges.map((e: any) => e.node.name);`)
		var res saveRes
		h.mustAPI(t, "POST", "/api/toolkits/"+k+"/save", map[string]any{"source": src, "tests": []any{}}, &res)
		expect(t, res.Status == "live", "a GraphQL query tool waits for review as a write (%s)", res.Status)
		a := h.agentFor(t, k, "read")
		out := a.ok(name, map[string]any{})
		expect(t, len(out.([]any)) == 3, "got %s", asJSON(out))
	})

	// ── Resilience ──
	add("resilience", "429 with Retry-After is retried", func(t *testing.T) {
		need(t, "flaky")
		a, name := one(t, "read", []string{"flaky"}, "A rate-limited read.", "", nil, `  return await flaky.getLimited({});`)
		start := time.Now()
		out := a.ok(name, map[string]any{})
		expect(t, get(out, "ok") == true, "got %s", asJSON(out))
		expect(t, time.Since(start) >= time.Second, "Retry-After was not honoured")
	})

	add("resilience", "503 on a GET is retried once", func(t *testing.T) {
		need(t, "flaky")
		a, name := one(t, "read", []string{"flaky"}, "An unstable read.", "", nil, `  return await flaky.getUnstable({});`)
		for i := 0; i < 2; i++ {
			out := a.ok(name, map[string]any{})
			expect(t, get(out, "ok") == true, "got %s", asJSON(out))
		}
	})

	add("resilience", "500 HTML error page becomes a short message", func(t *testing.T) {
		need(t, "flaky")
		a, name := one(t, "read", []string{"flaky"}, "A broken read.", "", nil, `  return await flaky.getBroken({});`)
		text := a.fails(name, map[string]any{}, "500")
		expect(t, !strings.Contains(text, "<html>") && len(text) < 600, "error is raw HTML or too long: %s", clip(text, 300))
	})

	add("resilience", "slow upstream within the tool's timeout", func(t *testing.T) {
		need(t, "flaky")
		a, name := one(t, "read", []string{"flaky"}, "A slow read.", "", nil, `  return await flaky.getSlow({ ms: 1500 });`)
		out := a.ok(name, map[string]any{})
		expect(t, num(get(out, "waited_ms")) == 1500, "got %s", asJSON(out))
	})

	add("resilience", "tool timeout is configurable for long jobs", func(t *testing.T) {
		need(t, "flaky")
		k := kit(t, "flaky")
		name := uniq("long")
		src := strings.Replace(toolSrc(name, "Three slow reads in a row.", "", nil,
			`  for (let i = 0; i < 3; i++) await flaky.getSlow({ ms: 4000 });
  return "done";`), "inputSchema:", "timeout: 30,\n  inputSchema:", 1)
		h.saveTool(t, k, src)
		a := h.agentFor(t, k, "read")
		out := a.ok(name, map[string]any{})
		expect(t, out == "done", "got %v", out)
	})

	add("resilience", "default timeout gives a clear message", func(t *testing.T) {
		need(t, "flaky")
		a, name := one(t, "read", []string{"flaky"}, "Too slow.", "", nil, `  for (let i = 0; i < 4; i++) await flaky.getSlow({ ms: 5000 });
  return "done";`)
		a.fails(name, map[string]any{}, "timed out")
	})

	add("async", "export job: 202, poll Location, download CSV", func(t *testing.T) {
		need(t, "flaky")
		k := kit(t, "flaky")
		name := uniq("export")
		h.saveTool(t, k, toolSrc(name, "Run a revenue export and return its rows.", "", nil,
			`  const job = await flaky.startExport.raw({});
  const loc = job.headers["location"];
  let st: any;
  for (let i = 0; i < 10; i++) {
    st = await call("flaky", "GET " + loc, {});
    if (st.status === "done") break;
    await sleep(100);
  }
  const csv: string = await call("flaky", "GET " + st.download_url, {});
  const [head, ...rows] = csv.trim().split("\n");
  const keys = head.split(",");
  return rows.map((r) => Object.fromEntries(r.split(",").map((v, i) => [keys[i], v])));`))
		a := h.agentFor(t, k, "read_write")
		out := a.ok(name, map[string]any{}).([]any)
		expect(t, len(out) == 3 && get(out[1], "region") == "amer", "got %s", asJSON(out))
	})

	add("payloads", "PDF download reaches the tool as metadata", func(t *testing.T) {
		need(t, "flaky")
		a, name := one(t, "read", []string{"flaky"}, "A file's facts.", "", nil, `  return await flaky.getFile({ id: "f1" });`)
		text, _ := a.call(name, map[string]any{})
		expect(t, strings.Contains(text, "application/pdf") && !strings.Contains(text, "%PDF"), "got %q", clip(text, 200))
	})

	add("payloads", "huge response is refused with advice, not truncated JSON", func(t *testing.T) {
		need(t, "flaky")
		a, name := one(t, "read", []string{"flaky"}, "The whole catalog.", "", nil, `  return await flaky.getCatalog({});`)
		text, isErr := a.call(name, map[string]any{})
		expect(t, isErr && len(text) < 2000, "a 1 MB response reached the agent (%d chars)", len(text))
	})

	add("shaping", "huge response summarized inside the tool", func(t *testing.T) {
		need(t, "flaky")
		a, name := one(t, "read", []string{"flaky"}, "Catalog stats per warehouse.", "", nil,
			`  const r = await flaky.getCatalog({});
  const by: Record<string, number> = {};
  for (const x of r.items) by[x.warehouse.name] = (by[x.warehouse.name] || 0) + 1;
  return { total: r.count, byWarehouse: by };`)
		out := a.ok(name, map[string]any{})
		expect(t, num(get(out, "total")) == 6000 && num(get(out, "byWarehouse.WH-0")) == 667, "got %s", asJSON(out))
	})
}
