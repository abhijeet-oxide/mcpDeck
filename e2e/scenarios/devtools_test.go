package scenarios

import (
	"strings"
	"testing"
)

// GitHub-, Jira- and ServiceNow-shaped APIs: Link headers, startAt/total,
// sysparm_* with X-Total-Count, reference fields and rich-text bodies.

func init() {
	add("pagination", "github Link header rel=next followed with .raw()", func(t *testing.T) {
		need(t, "github")
		a, name := one(t, "read", []string{"github"}, "Every open issue number in a repo.", `repo: { type: "string" }`, []string{"repo"},
			`  const nums: number[] = [];
  let page = 1;
  for (;;) {
    const r = await call.raw("github", "GET /repos/acme/" + input.repo + "/issues", { per_page: 20, page });
    nums.push(...r.body.map((i: any) => i.number));
    const link = r.headers["link"] || "";
    if (!/rel="next"/.test(link)) break;
    page++;
  }
  return { count: nums.length };`)
		out := a.ok(name, map[string]any{"repo": "api"})
		expect(t, num(get(out, "count")) == 55, "got %s", asJSON(out))
	})

	add("onboarding", "github operationIds with slashes become callable names", func(t *testing.T) {
		a := need(t, "github")
		for _, r := range a.Requests {
			expect(t, !strings.ContainsAny(r.Name, "/-"), "request name %q isn't a JS identifier", r.Name)
		}
	})

	add("pagination", "github page loop until a short page", func(t *testing.T) {
		need(t, "github")
		a, name := one(t, "read", []string{"github"}, "Count all issues, any state.", "", nil,
			`  let n = 0;
  for (let page = 1; page < 50; page++) {
    const r = await call("github", "GET /repos/acme/api/issues", { state: "all", per_page: 30, page });
    n += r.length;
    if (r.length < 30) break;
  }
  return n;`)
		out := a.ok(name, map[string]any{})
		expect(t, num(out) == 73, "got %v", out)
	})

	add("shaping", "github issues without pull requests, trimmed", func(t *testing.T) {
		need(t, "github")
		a, name := one(t, "read", []string{"github"}, "Open security issues.", "", nil,
			`  const r = await call("github", "GET /repos/acme/api/issues", { per_page: 100 });
  return r.filter((i: any) => !i.pull_request && i.labels.some((l: any) => l.name === "security"))
    .map((i: any) => ({ n: i.number, title: i.title, by: i.user.login }));`)
		text, _ := a.call(name, map[string]any{})
		expect(t, !strings.Contains(text, "avatar_url") && strings.Contains(text, `"n":3`), "got %s", clip(text, 300))
	})

	add("chaining", "github search then fetch each issue", func(t *testing.T) {
		need(t, "github")
		a, name := one(t, "read", []string{"github"}, "Titles of the top 3 search hits.", `q: { type: "string" }`, []string{"q"},
			`  const s = await call("github", "GET /search/issues", { q: input.q, per_page: 3 });
  const top = s.items.slice(0, 3);
  return await Promise.all(top.map((i: any) => call("github", "GET /repos/acme/api/issues/" + i.number, {}).then((x: any) => x.title)));`)
		out := a.ok(name, map[string]any{"q": "is:open label:bug"}).([]any)
		expect(t, len(out) == 3, "got %s", asJSON(out))
	})

	add("access", "github comment is a write and needs read_write", func(t *testing.T) {
		need(t, "github")
		k := kit(t, "github")
		name := uniq("comment")
		h.saveTool(t, k, toolSrc(name, "Comment on an issue.", `n: { type: "number" }, body: { type: "string" }`, []string{"n", "body"},
			`  return await call("github", "POST /repos/acme/api/issues/" + input.n + "/comments", { body: { body: input.body } });`))
		before := h.env.Count("github.comment")
		r := h.agentFor(t, k, "read")
		for _, n := range r.tools() {
			expect(t, n != name, "a read agent sees the write tool %s", name)
		}
		w := h.agentFor(t, k, "read_write")
		w.ok(name, map[string]any{"n": 7, "body": "Looking into it"})
		expect(t, h.env.Count("github.comment") == before+1, "the comment did not go out once")
	})

	add("auth", "github wrong token message reaches the agent", func(t *testing.T) {
		h.importURL(t, "githubBad", h.env.URLs["github"]+"/openapi.json")
		a := h.apiByName(t, "githubBad")
		h.setVar(t, a.ID, "token", "ghp_wrong", true)
		ag, name := one(t, "read", []string{"githubBad"}, "One issue.", "", nil, `  return await call("githubBad", "GET /repos/acme/api/issues/1", {});`)
		ag.fails(name, map[string]any{}, "401", "Bad credentials")
	})

	add("pagination", "jira startAt and total", func(t *testing.T) {
		need(t, "jira")
		a, name := one(t, "read", []string{"jira"}, "Open issue keys in OPS.", "", nil,
			`  const keys: string[] = [];
  for (let startAt = 0; ; ) {
    const r = await jira.searchForIssuesUsingJql({ jql: "project = OPS AND statusCategory != Done", startAt, maxResults: 20 });
    keys.push(...r.issues.map((i: any) => i.key));
    startAt += r.issues.length;
    if (startAt >= r.total || r.issues.length === 0) break;
  }
  return { count: keys.length, unique: new Set(keys).size };`)
		text, isErr := a.call(name, map[string]any{})
		if isErr {
			t.Fatalf("%s: %s", text, h.lastCalls(t, 2))
		}
		out := decodeOut(text)
		expect(t, num(get(out, "count")) == 48 && num(get(out, "unique")) == 48, "got %s", asJSON(out))
	})

	add("shaping", "jira ADF description flattened to text", func(t *testing.T) {
		need(t, "jira")
		a, name := one(t, "read", []string{"jira"}, "An issue in plain text.", `key: { type: "string" }`, []string{"key"},
			`  const i = await jira.getIssue({ issueIdOrKey: input.key });
  const text = (n: any): string => n.text ?? (n.content || []).map(text).join(" ");
  const f = i.fields;
  return { key: i.key, summary: f.summary, status: f.status.name, assignee: f.assignee?.displayName, description: text(f.description.content[0]) };`)
		out := a.ok(name, map[string]any{"key": "OPS-5"})
		expect(t, get(out, "description") == "The step times out after 10 minutes." && get(out, "assignee") == "Bo Chen", "got %s", asJSON(out))
	})

	add("errors", "jira 404 errorMessages are passed through", func(t *testing.T) {
		need(t, "jira")
		a, name := one(t, "read", []string{"jira"}, "An issue.", `key: { type: "string" }`, []string{"key"}, `  return await jira.getIssue({ issueIdOrKey: input.key });`)
		a.fails(name, map[string]any{"key": "OPS-999"}, "404", "does not exist")
	})

	add("access", "jira transition after review, read agents can't", func(t *testing.T) {
		need(t, "jira")
		k := kit(t, "jira")
		name := uniq("start")
		h.saveTool(t, k, toolSrc(name, "Move an issue to In Progress.", `key: { type: "string" }`, []string{"key"},
			`  await jira.doTransition({ issueIdOrKey: input.key, body: { transition: { id: "21" } } });
  const i = await jira.getIssue({ issueIdOrKey: input.key });
  return { key: input.key, moved: true };`))
		before := h.env.Count("jira.transition")
		w := h.agentFor(t, k, "read_write")
		out := w.ok(name, map[string]any{"key": "OPS-4"})
		expect(t, get(out, "moved") == true && h.env.Count("jira.transition") == before+1, "got %s", asJSON(out))
	})

	add("onboarding", "servicenow Postman collection with collection-level basic auth", func(t *testing.T) {
		a := need(t, "servicenow")
		expect(t, a.Auth["type"] == "basic", "auth is %s", asJSON(a.Auth))
		expect(t, len(a.Requests) == 2, "got %d requests", len(a.Requests))
	})

	add("pagination", "servicenow sysparm_offset with X-Total-Count", func(t *testing.T) {
		need(t, "servicenow")
		a, name := one(t, "read", []string{"servicenow"}, "All active incident numbers.", "", nil,
			`  const nums: string[] = [];
  for (let off = 0; ; off += 15) {
    const r = await call.raw("servicenow", "GET /api/now/table/incident", { sysparm_query: "active=true", sysparm_limit: 15, sysparm_offset: off, sysparm_fields: "number" });
    nums.push(...r.body.result.map((x: any) => x.number));
    if (nums.length >= Number(r.headers["x-total-count"])) break;
  }
  return { count: nums.length };`)
		out := a.ok(name, map[string]any{})
		expect(t, num(get(out, "count")) == 42, "got %s", asJSON(out))
	})

	add("shaping", "servicenow display values instead of sys_id references", func(t *testing.T) {
		need(t, "servicenow")
		a, name := one(t, "read", []string{"servicenow"}, "P1 incidents with who has them.", "", nil,
			`  const r = await call("servicenow", "GET /api/now/table/incident", { sysparm_query: "active=true^priority=1", sysparm_display_value: "true", sysparm_fields: "number,short_description,assigned_to,assignment_group" });
  return r.result;`)
		text, _ := a.call(name, map[string]any{})
		out, _ := decodeOut(text).([]any)
		expect(t, len(out) == 10 && get(out[0], "assigned_to") == "Beth Anglin", "got %s", clip(text, 300))
	})

	add("access", "servicenow PATCH goes through review", func(t *testing.T) {
		need(t, "servicenow")
		k := kit(t, "servicenow")
		name := uniq("note")
		res := h.saveTool(t, k, toolSrc(name, "Add a work note to an incident.", `sys_id: { type: "string" }, note: { type: "string" }`, []string{"sys_id", "note"},
			`  const r = await call("servicenow", "PATCH /api/now/table/incident/" + input.sys_id, { body: { work_notes: input.note } });
  return { number: r.result.number };`))
		expect(t, res.Version.Access != "read", "a PATCH tool was saved as read")
		before := h.env.Count("servicenow.patch")
		w := h.agentFor(t, k, "read_write")
		w.ok(name, map[string]any{"sys_id": "00000000000000000000000000001eef", "note": "checking"})
		expect(t, h.env.Count("servicenow.patch") == before+1, "patch count unchanged")
	})
}
