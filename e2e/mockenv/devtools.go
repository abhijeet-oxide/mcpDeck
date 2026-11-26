package mockenv

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// github, jira and servicenow follow their public REST APIs: GitHub's Link
// header pagination and rate-limit headers, Jira's startAt/maxResults/total
// search with Atlassian Document Format descriptions, and ServiceNow's
// Table API with sysparm_* parameters, X-Total-Count and reference fields.

func linkHeader(base *url.URL, page, last int) string {
	var parts []string
	mk := func(p int, rel string) {
		u := *base
		q := u.Query()
		q.Set("page", strconv.Itoa(p))
		u.RawQuery = q.Encode()
		parts = append(parts, fmt.Sprintf(`<%s>; rel="%s"`, u.String(), rel))
	}
	if page < last {
		mk(page+1, "next")
		mk(last, "last")
	}
	if page > 1 {
		mk(1, "first")
		mk(page-1, "prev")
	}
	return strings.Join(parts, ", ")
}

func ghIssue(n int, repo string) map[string]any {
	labels := []any{map[string]any{"id": 1, "name": "bug", "color": "d73a4a", "description": "Something isn't working", "default": true, "url": "https://api.github.com/labels/bug"}}
	if n%3 == 0 {
		labels = append(labels, map[string]any{"id": 2, "name": "security", "color": "000000", "default": false})
	}
	is := map[string]any{"id": 1000000 + n, "node_id": fmt.Sprintf("I_kw%08d", n), "number": n, "title": fmt.Sprintf("Issue %d: crash when syncing", n),
		"state": map[bool]string{true: "open", false: "closed"}[n%4 != 0], "locked": false, "comments": n % 5,
		"user":   map[string]any{"login": fmt.Sprintf("user%d", n%6), "id": 100 + n%6, "avatar_url": "https://avatars.githubusercontent.com/u/1", "type": "User", "site_admin": false, "url": "https://api.github.com/users/x", "html_url": "https://github.com/x"},
		"labels": labels, "assignee": nil, "assignees": []any{}, "milestone": nil, "author_association": "CONTRIBUTOR",
		"created_at": fmt.Sprintf("2026-10-%02dT08:00:00Z", n%28+1), "updated_at": "2026-11-20T08:00:00Z", "closed_at": nil,
		"body":       "### Steps\n" + longText(60), "reactions": map[string]any{"total_count": n % 7, "+1": n % 7, "-1": 0, "laugh": 0, "url": "https://api.github.com/r"},
		"html_url":   fmt.Sprintf("https://github.com/acme/%s/issues/%d", repo, n), "url": fmt.Sprintf("https://api.github.com/repos/acme/%s/issues/%d", repo, n),
		"repository_url": "https://api.github.com/repos/acme/" + repo, "timeline_url": "https://api.github.com/t", "performed_via_github_app": nil}
	if n%5 == 0 {
		is["pull_request"] = map[string]any{"url": fmt.Sprintf("https://api.github.com/repos/acme/%s/pulls/%d", repo, n), "merged_at": nil}
	}
	return is
}

func (e *Env) github() http.Handler {
	mux := http.NewServeMux()
	var mu sync.Mutex
	comments := 0
	guard := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			a := r.Header.Get("Authorization")
			if a != "Bearer "+GitHubPAT && a != "token "+GitHubPAT {
				writeJSON(w, 401, map[string]any{"message": "Bad credentials", "documentation_url": "https://docs.github.com/rest", "status": "401"})
				return
			}
			w.Header().Set("X-RateLimit-Limit", "5000")
			w.Header().Set("X-RateLimit-Remaining", "4999")
			w.Header().Set("X-GitHub-Api-Version", "2022-11-28")
			h(w, r)
		}
	}
	mux.HandleFunc("GET /repos/{owner}/{repo}/issues", guard(func(w http.ResponseWriter, r *http.Request) {
		per, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if per <= 0 || per > 100 {
			per = 30
		}
		if page <= 0 {
			page = 1
		}
		state := r.URL.Query().Get("state")
		if state == "" {
			state = "open"
		}
		var all []map[string]any
		for n := 1; n <= 73; n++ {
			is := ghIssue(n, r.PathValue("repo"))
			if state == "all" || is["state"] == state {
				all = append(all, is)
			}
		}
		last := (len(all) + per - 1) / per
		lo, hi := (page-1)*per, page*per
		if lo > len(all) {
			lo = len(all)
		}
		if hi > len(all) {
			hi = len(all)
		}
		u, _ := url.Parse(origin(r) + r.URL.RequestURI())
		if l := linkHeader(u, page, last); l != "" {
			w.Header().Set("Link", l)
		}
		writeJSON(w, 200, all[lo:hi])
	}))
	mux.HandleFunc("GET /repos/{owner}/{repo}/issues/{number}", guard(func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.PathValue("number"))
		if n < 1 || n > 73 {
			writeJSON(w, 404, map[string]any{"message": "Not Found", "status": "404"})
			return
		}
		writeJSON(w, 200, ghIssue(n, r.PathValue("repo")))
	}))
	mux.HandleFunc("GET /search/issues", guard(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			writeJSON(w, 422, map[string]any{"message": "Validation Failed", "errors": []any{map[string]any{"resource": "Search", "field": "q", "code": "missing"}}})
			return
		}
		var items []any
		for n := 1; n <= 73 && len(items) < 10; n += 3 {
			items = append(items, ghIssue(n, "api"))
		}
		writeJSON(w, 200, map[string]any{"total_count": 25, "incomplete_results": false, "items": items})
	}))
	mux.HandleFunc("POST /repos/{owner}/{repo}/issues/{number}/comments", guard(func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Body string }
		if err := readJSON(r, &in); err != nil || in.Body == "" {
			writeJSON(w, 422, map[string]any{"message": "Validation Failed"})
			return
		}
		mu.Lock()
		comments++
		id := comments
		mu.Unlock()
		e.inc("github.comment")
		writeJSON(w, 201, map[string]any{"id": id, "body": in.Body, "html_url": "https://github.com/c/" + strconv.Itoa(id)})
	}))
	serveSpec(mux, "/openapi.json", func(r *http.Request) map[string]any {
		return specDoc{title: "GitHub v3 REST API", desc: "Subset: issues and search.", server: origin(r),
			schemes:  map[string]any{"bearer": map[string]any{"type": "http", "scheme": "bearer"}},
			security: []string{"bearer"},
			ops: []op{
				{method: "GET", path: "/repos/{owner}/{repo}/issues", id: "issues/list-for-repo", summary: "List repository issues", tag: "issues",
					params: []param{pp("owner"), pp("repo"), q("state", "open"), q("per_page", 30), q("page", 1)}},
				{method: "GET", path: "/repos/{owner}/{repo}/issues/{issue_number}", id: "issues/get", summary: "Get an issue", tag: "issues",
					params: []param{pp("owner"), pp("repo"), pp("issue_number")}},
				{method: "GET", path: "/search/issues", id: "search/issues-and-pull-requests", summary: "Search issues and pull requests", tag: "search", params: []param{qr("q", "is:open label:bug")}},
				{method: "POST", path: "/repos/{owner}/{repo}/issues/{issue_number}/comments", id: "issues/create-comment", summary: "Create an issue comment", tag: "issues",
					params: []param{pp("owner"), pp("repo"), pp("issue_number")}, body: jsonBody(map[string]any{"body": "Me too"})},
			}}.v3()
	})
	// GitHub's real paths use {issue_number}; the mock routes accept both.
	return mux
}

func adf(text string) map[string]any {
	return map[string]any{"type": "doc", "version": 1, "content": []any{
		map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text}}},
		map[string]any{"type": "codeBlock", "attrs": map[string]any{"language": "text"}, "content": []any{map[string]any{"type": "text", "text": longText(30)}}},
	}}
}

func jiraIssue(i int) map[string]any {
	st := []string{"To Do", "In Progress", "Done", "In Review"}[i%4]
	return map[string]any{"id": strconv.Itoa(10000 + i), "key": fmt.Sprintf("OPS-%d", i), "self": fmt.Sprintf("https://corp.atlassian.net/rest/api/3/issue/%d", 10000+i),
		"expand": "renderedFields,names,schema,operations,editmeta,changelog,versionedRepresentations",
		"fields": map[string]any{
			"summary": fmt.Sprintf("Deploy pipeline step %d fails intermittently", i), "description": adf("The step times out after 10 minutes."),
			"status":   map[string]any{"name": st, "id": strconv.Itoa(i % 4), "statusCategory": map[string]any{"key": map[string]string{"Done": "done"}[st], "colorName": "blue-gray", "name": st}},
			"priority": map[string]any{"name": []string{"Highest", "High", "Medium", "Low"}[i%4], "id": strconv.Itoa(i%4 + 1), "iconUrl": "https://corp.atlassian.net/images/icons/priorities/high.svg"},
			"assignee": map[string]any{"accountId": fmt.Sprintf("5b10a2844c20165700ede21%d", i%3), "displayName": []string{"Ana Lima", "Bo Chen", "Cy Patel"}[i%3], "emailAddress": "dev@corp.example", "active": true, "timeZone": "Europe/Berlin",
				"avatarUrls": map[string]any{"48x48": "https://a/48", "24x24": "https://a/24", "16x16": "https://a/16", "32x32": "https://a/32"}},
			"reporter": map[string]any{"displayName": "Release Bot", "accountId": "bot"}, "labels": []string{"ci", "flaky"},
			"created": "2026-11-01T09:00:00.000+0000", "updated": fmt.Sprintf("2026-11-%02dT10:00:00.000+0000", i%25+1),
			"issuetype": map[string]any{"name": "Bug", "subtask": false, "iconUrl": "https://corp.atlassian.net/bug.svg"},
			"project":   map[string]any{"key": "OPS", "name": "Operations", "projectTypeKey": "software"}, "customfield_10016": 3.0, "customfield_10020": nil,
		}}
}

func (e *Env) jira() http.Handler {
	mux := http.NewServeMux()
	var mu sync.Mutex
	transitions := map[string]string{}
	guard := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			u, p, ok := r.BasicAuth()
			if !ok || u != JiraEmail || p != JiraToken {
				w.Header().Set("WWW-Authenticate", `Basic realm="protected-area"`)
				writeJSON(w, 401, map[string]any{"errorMessages": []string{"You are not authenticated. Authentication required to perform this operation."}, "errors": map[string]any{}})
				return
			}
			h(w, r)
		}
	}
	search := func(w http.ResponseWriter, r *http.Request, jql string, startAt, max int) {
		if max <= 0 || max > 100 {
			max = 50
		}
		var all []map[string]any
		for i := 1; i <= 64; i++ {
			is := jiraIssue(i)
			if s, ok := transitions[is["key"].(string)]; ok {
				is["fields"].(map[string]any)["status"].(map[string]any)["name"] = s
			}
			name := is["fields"].(map[string]any)["status"].(map[string]any)["name"].(string)
			if strings.Contains(jql, "statusCategory != Done") && name == "Done" {
				continue
			}
			if strings.Contains(jql, `status = "In Progress"`) && name != "In Progress" {
				continue
			}
			all = append(all, is)
		}
		lo, hi := startAt, startAt+max
		if lo > len(all) {
			lo = len(all)
		}
		if hi > len(all) {
			hi = len(all)
		}
		writeJSON(w, 200, map[string]any{"expand": "schema,names", "startAt": startAt, "maxResults": max, "total": len(all), "issues": all[lo:hi]})
	}
	mux.HandleFunc("GET /rest/api/3/search", guard(func(w http.ResponseWriter, r *http.Request) {
		s, _ := strconv.Atoi(r.URL.Query().Get("startAt"))
		m, _ := strconv.Atoi(r.URL.Query().Get("maxResults"))
		mu.Lock()
		defer mu.Unlock()
		search(w, r, r.URL.Query().Get("jql"), s, m)
	}))
	mux.HandleFunc("GET /rest/api/3/issue/{key}", guard(func(w http.ResponseWriter, r *http.Request) {
		var n int
		if _, err := fmt.Sscanf(r.PathValue("key"), "OPS-%d", &n); err != nil || n < 1 || n > 64 {
			writeJSON(w, 404, map[string]any{"errorMessages": []string{"Issue does not exist or you do not have permission to see it."}, "errors": map[string]any{}})
			return
		}
		writeJSON(w, 200, jiraIssue(n))
	}))
	mux.HandleFunc("POST /rest/api/3/issue/{key}/transitions", guard(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Transition struct{ ID string } `json:"transition"`
		}
		if err := readJSON(r, &in); err != nil || in.Transition.ID == "" {
			writeJSON(w, 400, map[string]any{"errorMessages": []string{"transition.id is required"}})
			return
		}
		mu.Lock()
		transitions[r.PathValue("key")] = map[string]string{"11": "To Do", "21": "In Progress", "31": "Done"}[in.Transition.ID]
		mu.Unlock()
		e.inc("jira.transition")
		w.WriteHeader(204)
	}))
	serveSpec(mux, "/openapi.json", func(r *http.Request) map[string]any {
		return specDoc{title: "The Jira Cloud platform REST API", desc: "Subset: search, issues and transitions.", server: origin(r),
			schemes:  map[string]any{"basicAuth": map[string]any{"type": "http", "scheme": "basic"}},
			security: []string{"basicAuth"},
			ops: []op{
				{method: "GET", path: "/rest/api/3/search", id: "searchForIssuesUsingJql", summary: "Search for issues using JQL (GET)", tag: "Issue search",
					params: []param{q("jql", "project = OPS"), q("startAt", 0), q("maxResults", 50), q("fields", nil)}},
				{method: "GET", path: "/rest/api/3/issue/{issueIdOrKey}", id: "getIssue", summary: "Get issue", tag: "Issues", params: []param{pp("issueIdOrKey")}},
				{method: "POST", path: "/rest/api/3/issue/{issueIdOrKey}/transitions", id: "doTransition", summary: "Transition issue", tag: "Issues",
					params: []param{pp("issueIdOrKey")}, body: jsonBody(map[string]any{"transition": map[string]any{"id": "21"}})},
			}}.v3()
	})
	mux.HandleFunc("GET /rest/api/3/issue/{key}/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	return mux
}

func (e *Env) servicenow() http.Handler {
	mux := http.NewServeMux()
	var mu sync.Mutex
	incidents := make([]map[string]any, 0, 42)
	for i := 1; i <= 42; i++ {
		sid := fmt.Sprintf("%032x", i*7919)
		incidents = append(incidents, map[string]any{"sys_id": sid, "number": fmt.Sprintf("INC%07d", 10000+i),
			"short_description": fmt.Sprintf("Email outage in region %d", i%5), "state": strconv.Itoa(i%3 + 1), "priority": strconv.Itoa(i%4 + 1),
			"impact": "2", "urgency": "2", "active": "true", "sys_created_on": fmt.Sprintf("2026-11-%02d 08:00:00", i%25+1), "sys_updated_by": "admin",
			"assigned_to":      map[string]any{"link": "https://corp.service-now.com/api/now/table/sys_user/6816f79cc0a8016401c5a33be04be441", "value": "6816f79cc0a8016401c5a33be04be441"},
			"assignment_group": map[string]any{"link": "https://corp.service-now.com/api/now/table/sys_user_group/d625dccec0a8016700a222a0f7900d06", "value": "d625dccec0a8016700a222a0f7900d06"},
			"caller_id":        map[string]any{"link": "https://corp.service-now.com/api/now/table/sys_user/5137153cc611227c000bbd1bd8cd2005", "value": "5137153cc611227c000bbd1bd8cd2005"},
			"description":      longText(40), "work_notes": "", "close_notes": "", "sys_tags": "", "sys_class_name": "incident", "sys_domain": map[string]any{"link": "x", "value": "global"}})
	}
	display := map[string]string{"6816f79cc0a8016401c5a33be04be441": "Beth Anglin", "d625dccec0a8016700a222a0f7900d06": "Service Desk", "5137153cc611227c000bbd1bd8cd2005": "Fred Luddy"}
	guard := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			u, p, ok := r.BasicAuth()
			if !ok || u != SNUser || p != SNPass {
				writeJSON(w, 401, map[string]any{"error": map[string]any{"message": "User Not Authenticated", "detail": "Required to provide Auth information"}, "status": "failure"})
				return
			}
			h(w, r)
		}
	}
	shape := func(in map[string]any, fields []string, disp bool) map[string]any {
		out := map[string]any{}
		for k, v := range in {
			if len(fields) > 0 && !contains(fields, k) {
				continue
			}
			if ref, ok := v.(map[string]any); ok && disp {
				if d, ok := display[ref["value"].(string)]; ok {
					v = d
				} else {
					v = ref["value"]
				}
			}
			out[k] = v
		}
		return out
	}
	mux.HandleFunc("GET /api/now/table/incident", guard(func(w http.ResponseWriter, r *http.Request) {
		qv := r.URL.Query()
		lim, _ := strconv.Atoi(qv.Get("sysparm_limit"))
		off, _ := strconv.Atoi(qv.Get("sysparm_offset"))
		if lim <= 0 {
			lim = 10000
		}
		var fields []string
		if f := qv.Get("sysparm_fields"); f != "" {
			fields = strings.Split(f, ",")
		}
		disp := qv.Get("sysparm_display_value") == "true"
		mu.Lock()
		var list []map[string]any
		for _, in := range incidents {
			if strings.Contains(qv.Get("sysparm_query"), "active=true") && in["active"] != "true" {
				continue
			}
			if strings.Contains(qv.Get("sysparm_query"), "priority=1") && in["priority"] != "1" {
				continue
			}
			list = append(list, in)
		}
		mu.Unlock()
		total := len(list)
		lo, hi := off, off+lim
		if lo > total {
			lo = total
		}
		if hi > total {
			hi = total
		}
		var res []any
		for _, in := range list[lo:hi] {
			res = append(res, shape(in, fields, disp))
		}
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
		if hi < total {
			u, _ := url.Parse(origin(r) + r.URL.RequestURI())
			q2 := u.Query()
			q2.Set("sysparm_offset", strconv.Itoa(hi))
			u.RawQuery = q2.Encode()
			w.Header().Set("Link", fmt.Sprintf(`<%s>;rel="next"`, u.String()))
		}
		writeJSON(w, 200, map[string]any{"result": res})
	}))
	mux.HandleFunc("PATCH /api/now/table/incident/{sys_id}", guard(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		_ = readJSON(r, &in)
		mu.Lock()
		defer mu.Unlock()
		for _, inc := range incidents {
			if inc["sys_id"] == r.PathValue("sys_id") {
				for k, v := range in {
					inc[k] = v
				}
				e.inc("servicenow.patch")
				writeJSON(w, 200, map[string]any{"result": shape(inc, nil, false)})
				return
			}
		}
		writeJSON(w, 404, map[string]any{"error": map[string]any{"message": "No Record found", "detail": "Record doesn't exist or ACL restricts the record retrieval"}, "status": "failure"})
	}))
	return mux
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if strings.TrimSpace(x) == s {
			return true
		}
	}
	return false
}

// ServiceNowPostman is a Postman v2.1 collection like the ones ServiceNow
// developers share: basic auth on the collection, {{instance}} variable.
func (e *Env) ServiceNowPostman() string {
	return `{
  "info": { "name": "ServiceNow Table API", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json" },
  "auth": { "type": "basic", "basic": [ { "key": "username", "value": "{{sn_user}}" }, { "key": "password", "value": "{{sn_password}}" } ] },
  "variable": [ { "key": "instance", "value": "` + e.URLs["servicenow"] + `" }, { "key": "sn_user", "value": "` + SNUser + `" }, { "key": "sn_password", "value": "` + SNPass + `" } ],
  "item": [
    { "name": "List incidents", "request": { "method": "GET", "header": [ { "key": "Accept", "value": "application/json" } ],
      "url": { "raw": "{{instance}}/api/now/table/incident?sysparm_query=active=true&sysparm_limit=10&sysparm_offset=0", "host": ["{{instance}}"], "path": ["api","now","table","incident"],
        "query": [ { "key": "sysparm_query", "value": "active=true" }, { "key": "sysparm_limit", "value": "10" }, { "key": "sysparm_offset", "value": "0" }, { "key": "sysparm_fields", "value": "", "disabled": true }, { "key": "sysparm_display_value", "value": "", "disabled": true } ] } } },
    { "name": "Update incident", "request": { "method": "PATCH", "header": [ { "key": "Content-Type", "value": "application/json" } ],
      "body": { "mode": "raw", "raw": "{\"work_notes\": \"Investigating\"}" },
      "url": { "raw": "{{instance}}/api/now/table/incident/:sys_id", "host": ["{{instance}}"], "path": ["api","now","table","incident",":sys_id"],
        "variable": [ { "key": "sys_id", "value": "" } ] } } }
  ]
}`
}
