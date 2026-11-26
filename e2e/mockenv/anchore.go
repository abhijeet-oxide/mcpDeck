package mockenv

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// anchore is Anchore Enterprise's v2 API: HTTP basic auth on every call, or
// a Bearer token from POST /v2/oauth/token (password grant, client_id
// "anonymous"). Adding an image starts asynchronous analysis: the image
// goes not_analyzed → analyzing → analyzed over a few reads, and its
// vulnerabilities and policy check are refused until it is analyzed. The
// published spec is Swagger 2.0 with a relative basePath, as Anchore ships.

type ancImage struct {
	Digest, Tag, Status string
	polls               int
}

func (e *Env) anchore() http.Handler {
	mux := http.NewServeMux()
	var mu sync.Mutex
	images := []*ancImage{
		{Digest: "sha256:" + sha("nginx:1.27")[:64], Tag: "docker.io/library/nginx:1.27", Status: "analyzed"},
		{Digest: "sha256:" + sha("acme/payments:1.4.2")[:64], Tag: "registry.acme.example/payments:1.4.2", Status: "analyzed"},
		{Digest: "sha256:" + sha("acme/ledger:3.0.0")[:64], Tag: "registry.acme.example/ledger:3.0.0", Status: "analyzed"},
	}
	ok := func(r *http.Request) bool {
		if u, p, has := r.BasicAuth(); has {
			return u == AnchoreUser && p == AnchorePass
		}
		_, valid := e.tokens.valid("anchore", bearer(r), false)
		return valid
	}
	guard := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !ok(r) {
				writeJSON(w, 401, map[string]any{"message": "Unauthorized", "httpcode": 401, "detail": map[string]any{}})
				return
			}
			h(w, r)
		}
	}
	record := func(im *ancImage) map[string]any {
		parts := strings.SplitN(im.Tag, "/", 2)
		repoTag := parts[len(parts)-1]
		repo, tag, _ := strings.Cut(repoTag, ":")
		return map[string]any{"image_digest": im.Digest, "analysis_status": im.Status, "image_status": "active",
			"created_at": "2026-11-20T10:00:00Z", "analyzed_at": map[bool]any{true: "2026-11-20T10:03:00Z", false: nil}[im.Status == "analyzed"],
			"image_detail":  []any{map[string]any{"full_tag": im.Tag, "registry": parts[0], "repo": repo, "tag": tag, "full_digest": parts[0] + "/" + repo + "@" + im.Digest}},
			"image_content": map[string]any{"metadata": map[string]any{"arch": "amd64", "distro": "debian", "distro_version": "12", "image_size": 74000000, "layer_count": 7, "dockerfile_mode": "Guessed"}},
			"annotations":   map[string]any{}, "parent_digest": im.Digest, "image_type": "docker"}
	}
	find := func(d string) *ancImage {
		for _, im := range images {
			if im.Digest == d {
				return im
			}
		}
		return nil
	}
	mux.HandleFunc("POST /v2/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f := r.PostForm
		if f.Get("grant_type") != "password" || f.Get("client_id") != "anonymous" || f.Get("username") != AnchoreUser || f.Get("password") != AnchorePass {
			writeJSON(w, 401, map[string]any{"error": "invalid_grant"})
			return
		}
		e.inc("anchore.token")
		writeJSON(w, 200, map[string]any{"access_token": e.tokens.issue("anchore", AnchoreUser, e.ttl(), false), "token_type": "bearer",
			"expires_in": int(e.ttl().Seconds()), "refresh_token": e.tokens.issue("anchore", AnchoreUser, e.ttl()*24, true)})
	})
	mux.HandleFunc("GET /v2/images", guard(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var items []any
		for _, im := range images {
			items = append(items, record(im))
		}
		writeJSON(w, 200, map[string]any{"items": items})
	}))
	mux.HandleFunc("POST /v2/images", guard(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Source struct {
				Tag struct {
					PullString string `json:"pull_string"`
				} `json:"tag"`
			} `json:"source"`
		}
		if err := readJSON(r, &in); err != nil || in.Source.Tag.PullString == "" {
			writeJSON(w, 400, map[string]any{"message": "source.tag.pull_string is required", "httpcode": 400})
			return
		}
		mu.Lock()
		defer mu.Unlock()
		e.inc("anchore.add")
		d := "sha256:" + sha(in.Source.Tag.PullString)[:64]
		im := find(d)
		if im == nil {
			im = &ancImage{Digest: d, Tag: in.Source.Tag.PullString, Status: "not_analyzed"}
			images = append(images, im)
		}
		writeJSON(w, 200, record(im))
	}))
	mux.HandleFunc("GET /v2/images/{digest}", guard(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		im := find(r.PathValue("digest"))
		if im == nil {
			writeJSON(w, 404, map[string]any{"message": "image not found", "httpcode": 404})
			return
		}
		// Analysis advances as the image is polled.
		im.polls++
		switch {
		case im.Status == "not_analyzed":
			im.Status = "analyzing"
		case im.Status == "analyzing" && im.polls >= 3:
			im.Status = "analyzed"
		}
		writeJSON(w, 200, record(im))
	}))
	mux.HandleFunc("GET /v2/images/{digest}/vuln/{vtype}", guard(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		im := find(r.PathValue("digest"))
		mu.Unlock()
		if im == nil {
			writeJSON(w, 404, map[string]any{"message": "image not found", "httpcode": 404})
			return
		}
		if im.Status != "analyzed" {
			writeJSON(w, 404, map[string]any{"message": "image is not analyzed - analysis_status: " + im.Status, "httpcode": 404})
			return
		}
		sevs := []string{"Critical", "High", "High", "Medium", "Medium", "Low", "Negligible"}
		var vs []any
		n := 40 + len(im.Tag)%20
		for i := 0; i < n; i++ {
			sev := sevs[(i+len(im.Tag))%len(sevs)]
			fix := "None"
			if i%3 != 0 {
				fix = fmt.Sprintf("3.0.%d", i)
			}
			vs = append(vs, map[string]any{"vuln": fmt.Sprintf("CVE-2026-%04d", 5000+i), "severity": sev, "package": fmt.Sprintf("lib%d-%d", i%9, i),
				"package_name": fmt.Sprintf("lib%d", i%9), "package_version": "2.1." + fmt.Sprint(i), "package_type": "dpkg", "package_cpe": "None",
				"package_path": "pkgdb", "fix": fix, "url": fmt.Sprintf("https://security-tracker.debian.org/tracker/CVE-2026-%04d", 5000+i),
				"feed": "vulnerabilities", "feed_group": "debian:12", "will_not_fix": false,
				"nvd_data":    []any{map[string]any{"id": fmt.Sprintf("CVE-2026-%04d", 5000+i), "cvss_v3": map[string]any{"base_score": 9.8 - float64(i%7), "exploitability_score": 3.9, "impact_score": 5.9}}},
				"vendor_data": []any{}, "extra": map[string]any{"description": longText(25)}})
		}
		writeJSON(w, 200, map[string]any{"image_digest": im.Digest, "vulnerability_type": r.PathValue("vtype"), "vulnerabilities": vs})
	}))
	mux.HandleFunc("GET /v2/images/{digest}/check", guard(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		im := find(r.PathValue("digest"))
		mu.Unlock()
		if im == nil || im.Status != "analyzed" {
			writeJSON(w, 404, map[string]any{"message": "image not found or not analyzed", "httpcode": 404})
			return
		}
		tag := r.URL.Query().Get("tag")
		if tag == "" {
			writeJSON(w, 400, map[string]any{"message": "tag is required", "httpcode": 400})
			return
		}
		fail := strings.Contains(im.Tag, "nginx")
		status, action := "pass", "go"
		var findings []any
		if fail {
			status, action = "fail", "stop"
			findings = []any{
				map[string]any{"trigger_id": "CVE-2026-5000+lib0", "gate": "vulnerabilities", "trigger": "package", "action": "stop", "policy_id": "default",
					"message": "CRITICAL Vulnerability found in os package type (dpkg) - lib0 (fixed in: 3.0.1)(CVE-2026-5000)", "recommendation": "Upgrade lib0", "allowlisted": false},
				map[string]any{"trigger_id": "dockerfile-user", "gate": "dockerfile", "trigger": "effective_user", "action": "warn", "policy_id": "default",
					"message": "User root found as effective user, which is not on the allowed list", "recommendation": "", "allowlisted": false},
			}
		}
		writeJSON(w, 200, map[string]any{"image_digest": im.Digest, "evaluated_tag": tag, "evaluations": []any{
			map[string]any{"status": status, "final_action": action, "final_action_reason": "policy_evaluation", "evaluation_time": "2026-11-25T09:00:00Z",
				"policy_id": "default", "details": map[string]any{"findings": findings, "policy": map[string]any{"name": "Default bundle", "id": "default"}}}}})
	}))
	serveSpec(mux, "/v2/swagger.json", func(r *http.Request) map[string]any {
		return specDoc{title: "Anchore Enterprise API Server", desc: "Image analysis, vulnerabilities and policy evaluation.", server: "/v2",
			schemes:  map[string]any{"basicAuth": map[string]any{"type": "basic"}},
			security: []string{"basicAuth"},
			ops: []op{
				{method: "GET", path: "/images", id: "list_images", summary: "List all images", tag: "Images"},
				{method: "POST", path: "/images", id: "add_image", summary: "Add an image for analysis", tag: "Images",
					body: jsonBody(map[string]any{"source": map[string]any{"tag": map[string]any{"pull_string": "docker.io/library/nginx:latest"}}})},
				{method: "GET", path: "/images/{image_digest}", id: "get_image", summary: "Get an image by digest", tag: "Images", params: []param{pp("image_digest")}},
				{method: "GET", path: "/images/{image_digest}/vuln/{vtype}", id: "get_image_vulnerabilities_by_type", summary: "Get vulnerabilities by type", tag: "Images",
					params: []param{pp("image_digest"), pp("vtype")}},
				{method: "GET", path: "/images/{image_digest}/check", id: "get_image_policy_check_by_digest", summary: "Check policy evaluation status for image", tag: "Policy",
					params: []param{pp("image_digest"), qr("tag", "docker.io/library/nginx:latest")}},
			}}.v2()
	})
	return mux
}
