package mockenv

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// jfrog is one JFrog Platform host: Artifactory under /artifactory, Access
// under /access and Xray under /xray, as on a real instance. Auth matches
// the JFrog docs: basic auth or a Bearer access token everywhere, the legacy
// X-JFrog-Art-Api key header on Artifactory, and scoped access tokens from
// POST /access/api/v1/tokens. JFrog publishes no single OpenAPI document,
// so people onboard it from the curl examples in its REST API docs.

type jfArtifact struct {
	Repo, Path, Name string
	Size             int
	Created          string
}

var jfRepos = []map[string]any{
	{"key": "libs-release-local", "type": "LOCAL", "packageType": "maven", "description": "Release builds"},
	{"key": "libs-snapshot-local", "type": "LOCAL", "packageType": "maven", "description": "Snapshot builds"},
	{"key": "docker-local", "type": "LOCAL", "packageType": "docker", "description": "Images"},
	{"key": "npm-remote", "type": "REMOTE", "packageType": "npm", "description": "Proxy of registry.npmjs.org"},
	{"key": "libs-release", "type": "VIRTUAL", "packageType": "maven", "description": "All releases"},
}

func jfArtifacts() []jfArtifact {
	var out []jfArtifact
	for i := 0; i < 37; i++ {
		v := fmt.Sprintf("1.%d.%d", i/10, i%10)
		out = append(out, jfArtifact{Repo: "libs-release-local", Path: "com/acme/payments/" + v, Name: "payments-" + v + ".jar",
			Size: 120000 + i*731, Created: fmt.Sprintf("2026-%02d-%02dT08:00:00.000Z", i%11+1, i%27+1)})
	}
	for i := 0; i < 8; i++ {
		v := fmt.Sprintf("2.0.%d-SNAPSHOT", i)
		out = append(out, jfArtifact{Repo: "libs-snapshot-local", Path: "com/acme/ledger/" + v, Name: "ledger-" + v + ".jar",
			Size: 98000 + i*17, Created: fmt.Sprintf("2026-11-%02dT09:00:00.000Z", i+1)})
	}
	return out
}

func sha(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

var aqlRepoRe = regexp.MustCompile(`"repo"\s*:\s*(?:\{\s*"\$eq"\s*:\s*)?"([^"]+)"`)
var aqlNameRe = regexp.MustCompile(`"name"\s*:\s*\{\s*"\$match"\s*:\s*"([^"]+)"`)
var aqlOffRe = regexp.MustCompile(`\.offset\((\d+)\)`)
var aqlLimRe = regexp.MustCompile(`\.limit\((\d+)\)`)

func (e *Env) jfrog() http.Handler {
	mux := http.NewServeMux()
	var mu sync.Mutex
	arts := jfArtifacts()
	type who struct{ ok, admin bool }
	check := func(r *http.Request, allowKey bool) who {
		if u, p, ok := r.BasicAuth(); ok {
			return who{ok: u == JFrogUser && p == JFrogPass, admin: true}
		}
		if allowKey && r.Header.Get("X-JFrog-Art-Api") != "" {
			return who{ok: r.Header.Get("X-JFrog-Art-Api") == JFrogAPIKey}
		}
		if t := bearer(r); t != "" {
			_, ok := e.tokens.valid("jfrog", t, false)
			return who{ok: ok}
		}
		return who{}
	}
	deny := func(w http.ResponseWriter) {
		writeJSON(w, 401, map[string]any{"errors": []any{map[string]any{"status": 401, "message": "Bad credentials"}}})
	}
	art := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !check(r, true).ok {
				deny(w)
				return
			}
			h(w, r)
		}
	}
	xr := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !check(r, false).ok {
				writeJSON(w, 401, map[string]any{"error": "Unauthorized"})
				return
			}
			h(w, r)
		}
	}

	// Access: scoped tokens. Accepts form or JSON bodies, like the real API.
	mux.HandleFunc("POST /access/api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		if !check(r, false).ok {
			deny(w)
			return
		}
		scope, user := "applied-permissions/user", JFrogUser
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			var in map[string]any
			_ = readJSON(r, &in)
			if s, ok := in["scope"].(string); ok && s != "" {
				scope = s
			}
		} else {
			_ = r.ParseForm()
			if s := r.PostForm.Get("scope"); s != "" {
				scope = s
			}
			if u := r.PostForm.Get("username"); u != "" {
				user = u
			}
		}
		e.inc("jfrog.token")
		tok := e.tokens.issue("jfrog", user, e.ttl(), false)
		writeJSON(w, 200, map[string]any{"token_id": "tid-" + tok[len(tok)-6:], "access_token": tok, "expires_in": int(e.ttl().Seconds()),
			"scope": scope, "token_type": "Bearer"})
	})

	mux.HandleFunc("GET /artifactory/api/system/ping", art(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("OK"))
	}))
	mux.HandleFunc("GET /artifactory/api/repositories", art(func(w http.ResponseWriter, r *http.Request) {
		typ := strings.ToUpper(r.URL.Query().Get("type"))
		var out []map[string]any
		for _, rp := range jfRepos {
			if typ == "" || rp["type"] == typ {
				m := map[string]any{}
				for k, v := range rp {
					m[k] = v
				}
				m["url"] = origin(r) + "/artifactory/" + rp["key"].(string)
				out = append(out, m)
			}
		}
		writeJSON(w, 200, out)
	}))
	mux.HandleFunc("GET /artifactory/api/storage/{repo}/{path...}", art(func(w http.ResponseWriter, r *http.Request) {
		repo, path := r.PathValue("repo"), strings.Trim(r.PathValue("path"), "/")
		mu.Lock()
		defer mu.Unlock()
		for _, a := range arts {
			if a.Repo == repo && a.Path+"/"+a.Name == path {
				writeJSON(w, 200, map[string]any{"repo": repo, "path": "/" + path, "created": a.Created, "createdBy": "ci", "lastModified": a.Created,
					"downloadUri": origin(r) + "/artifactory/" + repo + "/" + path, "mimeType": "application/java-archive", "size": strconv.Itoa(a.Size),
					"checksums": map[string]any{"sha1": sha(path)[:40], "md5": sha(path)[:32], "sha256": sha(path)}, "uri": origin(r) + r.URL.Path})
				return
			}
		}
		children := map[string]bool{}
		for _, a := range arts {
			full := a.Path + "/" + a.Name
			if a.Repo == repo && (path == "" || strings.HasPrefix(full, path+"/")) {
				rest := strings.TrimPrefix(full, path+"/")
				if path == "" {
					rest = full
				}
				first, _, more := strings.Cut(rest, "/")
				children["/"+first+map[bool]string{true: "/", false: ""}[more]] = true
			}
		}
		if len(children) == 0 {
			writeJSON(w, 404, map[string]any{"errors": []any{map[string]any{"status": 404, "message": "Unable to find item"}}})
			return
		}
		var ch []map[string]any
		for c := range children {
			ch = append(ch, map[string]any{"uri": strings.TrimSuffix(c, "/"), "folder": strings.HasSuffix(c, "/")})
		}
		writeJSON(w, 200, map[string]any{"repo": repo, "path": "/" + path, "created": "2026-01-01T00:00:00.000Z", "children": ch, "uri": origin(r) + r.URL.Path})
	}))
	mux.HandleFunc("POST /artifactory/api/search/aql", art(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "text/plain") {
			writeJSON(w, 415, map[string]any{"errors": []any{map[string]any{"status": 415, "message": "AQL queries are sent as text/plain"}}})
			return
		}
		b := make([]byte, 8192)
		n, _ := r.Body.Read(b)
		query := string(b[:n])
		if !strings.HasPrefix(strings.TrimSpace(query), "items.find(") {
			writeJSON(w, 400, map[string]any{"errors": []any{map[string]any{"status": 400, "message": "Failed to parse query"}}})
			return
		}
		repo := ""
		if m := aqlRepoRe.FindStringSubmatch(query); m != nil {
			repo = m[1]
		}
		var nameRe *regexp.Regexp
		if m := aqlNameRe.FindStringSubmatch(query); m != nil {
			nameRe = regexp.MustCompile("^" + strings.ReplaceAll(regexp.QuoteMeta(m[1]), `\*`, ".*") + "$")
		}
		off, lim := 0, 1000
		if m := aqlOffRe.FindStringSubmatch(query); m != nil {
			off, _ = strconv.Atoi(m[1])
		}
		if m := aqlLimRe.FindStringSubmatch(query); m != nil {
			lim, _ = strconv.Atoi(m[1])
		}
		mu.Lock()
		var all []map[string]any
		for _, a := range arts {
			if (repo == "" || a.Repo == repo) && (nameRe == nil || nameRe.MatchString(a.Name)) {
				all = append(all, map[string]any{"repo": a.Repo, "path": a.Path, "name": a.Name, "type": "file", "size": a.Size,
					"created": a.Created, "created_by": "ci", "modified": a.Created, "modified_by": "ci", "updated": a.Created})
			}
		}
		mu.Unlock()
		if off > len(all) {
			off = len(all)
		}
		end := off + lim
		if end > len(all) {
			end = len(all)
		}
		page := all[off:end]
		writeJSON(w, 200, map[string]any{"results": page, "range": map[string]any{"start_pos": off, "end_pos": end, "total": len(page), "limit": lim}})
	}))
	mux.HandleFunc("GET /artifactory/api/build", art(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"uri": origin(r) + "/artifactory/api/build", "builds": []any{
			map[string]any{"uri": "/payments-ci", "lastStarted": "2026-11-24T10:00:00.000+0000"},
			map[string]any{"uri": "/ledger-ci", "lastStarted": "2026-11-20T08:30:00.000+0000"}}})
	}))
	mux.HandleFunc("GET /artifactory/api/build/{name}/{number}", art(func(w http.ResponseWriter, r *http.Request) {
		name, num := r.PathValue("name"), r.PathValue("number")
		if name != "payments-ci" && name != "ledger-ci" {
			writeJSON(w, 404, map[string]any{"errors": []any{map[string]any{"status": 404, "message": "No build was found for build name: " + name}}})
			return
		}
		var deps []any
		for i := 0; i < 30; i++ {
			deps = append(deps, map[string]any{"id": fmt.Sprintf("org.lib:dep%d:1.%d", i, i), "type": "jar", "sha1": sha(fmt.Sprint(i))[:40], "md5": sha(fmt.Sprint(i))[:32], "scopes": []string{"compile"}})
		}
		writeJSON(w, 200, map[string]any{"uri": origin(r) + r.URL.Path, "buildInfo": map[string]any{
			"version": "1.0.1", "name": name, "number": num, "type": "GENERIC", "started": "2026-11-24T10:00:00.000+0000", "durationMillis": 183000,
			"agent": map[string]any{"name": "GitHub Actions", "version": "2.320"}, "buildAgent": map[string]any{"name": "Maven", "version": "3.9.9"},
			"vcs":        []any{map[string]any{"revision": "9f1c2ab7e0d4", "url": "https://github.com/acme/" + name, "branch": "main", "message": "release " + num}},
			"modules":    []any{map[string]any{"id": "com.acme:" + strings.TrimSuffix(name, "-ci") + ":1.4." + num, "artifacts": []any{map[string]any{"type": "jar", "sha1": sha(name)[:40], "name": strings.TrimSuffix(name, "-ci") + "-1.4." + num + ".jar"}}, "dependencies": deps}},
			"properties": map[string]any{"buildInfo.env.GITHUB_RUN_ID": "123456"},
		}})
	}))
	// Download and delete an artifact.
	mux.HandleFunc("GET /artifactory/{repo}/{path...}", art(func(w http.ResponseWriter, r *http.Request) {
		repo, path := r.PathValue("repo"), r.PathValue("path")
		if repo == "api" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		for _, a := range arts {
			if a.Repo == repo && a.Path+"/"+a.Name == path {
				w.Header().Set("Content-Type", "application/java-archive")
				w.Header().Set("X-Checksum-Sha256", sha(path))
				// A zip header and some bytes: binary content.
				buf := []byte{0x50, 0x4b, 0x03, 0x04, 0x14, 0x00, 0x08, 0x00}
				for i := 0; i < 2048; i++ {
					buf = append(buf, byte(i*31))
				}
				_, _ = w.Write(buf)
				return
			}
		}
		writeJSON(w, 404, map[string]any{"errors": []any{map[string]any{"status": 404, "message": "File not found."}}})
	}))
	mux.HandleFunc("DELETE /artifactory/{repo}/{path...}", art(func(w http.ResponseWriter, r *http.Request) {
		repo, path := r.PathValue("repo"), r.PathValue("path")
		mu.Lock()
		defer mu.Unlock()
		for i, a := range arts {
			if a.Repo == repo && a.Path+"/"+a.Name == path {
				arts = append(arts[:i], arts[i+1:]...)
				e.inc("jfrog.delete")
				w.WriteHeader(204)
				return
			}
		}
		writeJSON(w, 404, map[string]any{"errors": []any{map[string]any{"status": 404, "message": "Could not locate artifact"}}})
	}))

	// Xray.
	mux.HandleFunc("GET /xray/api/v1/system/ping", xr(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"status": "pong"})
	}))
	mux.HandleFunc("GET /xray/api/v2/watches", xr(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, []any{
			map[string]any{"general_data": map[string]any{"id": "w1", "name": "prod-releases", "description": "Release repos", "active": true},
				"project_resources": map[string]any{"resources": []any{map[string]any{"type": "repository", "bin_mgr_id": "default", "name": "libs-release-local"}}},
				"assigned_policies": []any{map[string]any{"name": "block-critical", "type": "security"}, map[string]any{"name": "no-gpl", "type": "license"}}},
			map[string]any{"general_data": map[string]any{"id": "w2", "name": "snapshots", "active": false},
				"project_resources": map[string]any{"resources": []any{map[string]any{"type": "repository", "bin_mgr_id": "default", "name": "libs-snapshot-local"}}},
				"assigned_policies": []any{map[string]any{"name": "block-critical", "type": "security"}}},
		})
	}))
	sevRank := map[string]int{"Low": 1, "Medium": 2, "High": 3, "Critical": 4}
	violations := func() []map[string]any {
		var out []map[string]any
		sevs := []string{"Critical", "High", "High", "Medium", "Low"}
		for i := 0; i < 23; i++ {
			v := fmt.Sprintf("1.%d.%d", i/10, i%10)
			out = append(out, map[string]any{
				"description": "A flaw in a transitive dependency allows remote code execution. " + longText(30),
				"severity":    sevs[i%len(sevs)], "type": map[bool]string{true: "License", false: "Security"}[i%7 == 6],
				"infected_components": []string{fmt.Sprintf("gav://org.lib:dep%d:1.%d", i%5, i)},
				"created":             fmt.Sprintf("2026-11-%02dT12:00:00Z", i%25+1), "watch_name": "prod-releases", "issue_id": fmt.Sprintf("XRAY-%d", 100000+i),
				"violation_details_url": "https://jfrog.example/ui/violations/" + strconv.Itoa(i),
				"impacted_artifacts":    []string{"default/libs-release-local/com/acme/payments/" + v + "/payments-" + v + ".jar"},
			})
		}
		return out
	}()
	mux.HandleFunc("POST /xray/api/v1/violations", xr(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Filters struct {
				WatchName     string `json:"watch_name"`
				MinSeverity   string `json:"min_severity"`
				ViolationType string `json:"violation_type"`
			} `json:"filters"`
			Pagination struct {
				Limit  int `json:"limit"`
				Offset int `json:"offset"`
			} `json:"pagination"`
		}
		if err := readJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]any{"error": "invalid json"})
			return
		}
		var list []map[string]any
		for _, v := range violations {
			if in.Filters.WatchName != "" && v["watch_name"] != in.Filters.WatchName {
				continue
			}
			if in.Filters.MinSeverity != "" && sevRank[v["severity"].(string)] < sevRank[in.Filters.MinSeverity] {
				continue
			}
			if in.Filters.ViolationType != "" && v["type"] != in.Filters.ViolationType {
				continue
			}
			list = append(list, v)
		}
		lim := in.Pagination.Limit
		if lim <= 0 {
			lim = 25
		}
		page := in.Pagination.Offset // Xray's offset is a 1-based page number
		if page <= 0 {
			page = 1
		}
		lo, hi := (page-1)*lim, page*lim
		if lo > len(list) {
			lo = len(list)
		}
		if hi > len(list) {
			hi = len(list)
		}
		writeJSON(w, 200, map[string]any{"total_violations": len(list), "violations": list[lo:hi]})
	}))
	mux.HandleFunc("POST /xray/api/v1/summary/artifact", xr(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Paths []string `json:"paths"`
		}
		_ = readJSON(r, &in)
		var out []any
		for _, p := range in.Paths {
			var issues []any
			for i := 0; i < 4; i++ {
				issues = append(issues, map[string]any{"issue_id": fmt.Sprintf("XRAY-%d", 200000+i+len(p)), "summary": "Deserialization of untrusted data",
					"description": longText(50), "issue_type": "security", "severity": []string{"Critical", "High", "Medium", "Low"}[i],
					"provider": "JFrog", "cves": []any{map[string]any{"cve": fmt.Sprintf("CVE-2026-%04d", 3000+i+len(p)), "cvss_v3": "9.8/CVSS:3.1/AV:N/AC:L"}},
					"created": "2026-10-01T00:00:00Z", "impact_path": []string{p, "org.lib:dep1:1.1"}})
			}
			out = append(out, map[string]any{"general": map[string]any{"name": p[strings.LastIndex(p, "/")+1:], "path": p, "pkg_type": "Maven", "sha256": sha(p), "component_id": "gav://com.acme:payments"},
				"issues": issues, "licenses": []any{map[string]any{"name": "Apache-2.0", "full_name": "The Apache Software License, Version 2.0", "components": []string{"gav://com.acme:payments"}}}})
		}
		writeJSON(w, 200, map[string]any{"artifacts": out, "errors": []any{}})
	}))
	return mux
}

// JFrogCurl is how JFrog's REST API docs show these calls, ready to paste
// into mcpDeck's curl import (with the real host).
func (e *Env) JFrogCurl() string {
	u := e.URLs["jfrog"]
	return strings.Join([]string{
		`curl -u admin:` + JFrogPass + ` -X POST "` + u + `/access/api/v1/tokens" -d "scope=applied-permissions/user" -d "username=admin" -d "expires_in=3600"`,
		`curl -H "Authorization: Bearer <TOKEN>" "` + u + `/artifactory/api/repositories?type=local"`,
		`curl -H "Authorization: Bearer <TOKEN>" "` + u + `/artifactory/api/storage/libs-release-local/com/acme/payments"`,
		`curl -H "Authorization: Bearer <TOKEN>" -X POST "` + u + `/artifactory/api/search/aql" -H "Content-Type: text/plain" -d 'items.find({"repo":"libs-release-local"}).include("name","repo","path","size","created").sort({"$desc":["created"]}).offset(0).limit(10)'`,
		`curl -H "Authorization: Bearer <TOKEN>" "` + u + `/artifactory/api/build/payments-ci/42"`,
		`curl -H "Authorization: Bearer <TOKEN>" -X POST "` + u + `/xray/api/v1/violations" -H "Content-Type: application/json" -d '{"filters":{"watch_name":"prod-releases","min_severity":"High"},"pagination":{"order_by":"created","limit":25,"offset":1}}'`,
		`curl -H "Authorization: Bearer <TOKEN>" "` + u + `/xray/api/v2/watches"`,
		`curl -H "Authorization: Bearer <TOKEN>" -X POST "` + u + `/xray/api/v1/summary/artifact" -H "Content-Type: application/json" -d '{"paths":["default/libs-release-local/com/acme/payments/1.0.0/payments-1.0.0.jar"]}'`,
	}, "\n\n")
}
