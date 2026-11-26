package mockenv

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// kube is a Kubernetes API server subset over HTTPS: ServiceAccount bearer
// tokens, list pagination with limit/continue, verbose objects with
// managedFields (the classic "too big for an agent" response), pod logs as
// text, and a merge-patch scale subresource.

func pod(ns string, i int) map[string]any {
	app := []string{"checkout", "cart", "search"}[i%3]
	phase := "Running"
	restarts := 0
	if i%7 == 3 {
		phase, restarts = "CrashLoopBackOff", 14
	}
	mf := []any{}
	for j := 0; j < 4; j++ {
		mf = append(mf, map[string]any{"manager": "kube-controller-manager", "operation": "Update", "apiVersion": "v1", "time": "2026-11-20T10:00:00Z",
			"fieldsType": "FieldsV1", "fieldsV1": map[string]any{"f:metadata": map[string]any{"f:labels": map[string]any{".": map[string]any{}, "f:app": map[string]any{}, "f:pod-template-hash": map[string]any{}},
				"f:ownerReferences": map[string]any{".": map[string]any{}}}, "f:spec": map[string]any{"f:containers": map[string]any{"k:{\"name\":\"" + app + "\"}": map[string]any{".": map[string]any{}, "f:image": map[string]any{}}}}}})
	}
	return map[string]any{"metadata": map[string]any{"name": fmt.Sprintf("%s-7d9f8c6b5-%05d", app, i), "namespace": ns, "uid": fmt.Sprintf("uid-%d", i),
		"resourceVersion": strconv.Itoa(4400 + i), "creationTimestamp": "2026-11-20T10:00:00Z", "labels": map[string]any{"app": app, "pod-template-hash": "7d9f8c6b5"},
		"ownerReferences": []any{map[string]any{"apiVersion": "apps/v1", "kind": "ReplicaSet", "name": app + "-7d9f8c6b5", "uid": "rs", "controller": true}}, "managedFields": mf},
		"spec": map[string]any{"containers": []any{map[string]any{"name": app, "image": "registry.acme.example/" + app + ":1.4.2", "ports": []any{map[string]any{"containerPort": 8080, "protocol": "TCP"}},
			"resources": map[string]any{"limits": map[string]any{"cpu": "500m", "memory": "512Mi"}, "requests": map[string]any{"cpu": "100m", "memory": "128Mi"}},
			"env": []any{map[string]any{"name": "LOG_LEVEL", "value": "info"}}, "terminationMessagePath": "/dev/termination-log", "imagePullPolicy": "IfNotPresent"}},
			"restartPolicy": "Always", "dnsPolicy": "ClusterFirst", "serviceAccountName": "default", "nodeName": fmt.Sprintf("node-%d", i%4), "tolerations": []any{map[string]any{"key": "node.kubernetes.io/not-ready", "operator": "Exists", "effect": "NoExecute", "tolerationSeconds": 300}}},
		"status": map[string]any{"phase": map[bool]string{true: "Running", false: "Running"}[true], "podIP": fmt.Sprintf("10.0.%d.%d", i/250, i%250), "hostIP": "192.168.1.10",
			"containerStatuses": []any{map[string]any{"name": app, "ready": phase == "Running", "restartCount": restarts, "image": "registry.acme.example/" + app + ":1.4.2",
				"state": map[string]any{map[bool]string{true: "running", false: "waiting"}[phase == "Running"]: map[string]any{"reason": phase}}}},
			"conditions": []any{map[string]any{"type": "Ready", "status": map[bool]string{true: "True", false: "False"}[phase == "Running"]}}}}
}

func (e *Env) kube() http.Handler {
	mux := http.NewServeMux()
	var mu sync.Mutex
	replicas := map[string]int{"checkout": 3, "cart": 2, "search": 4}
	guard := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if bearer(r) != KubeToken {
				writeJSON(w, 401, map[string]any{"kind": "Status", "apiVersion": "v1", "metadata": map[string]any{}, "status": "Failure", "message": "Unauthorized", "reason": "Unauthorized", "code": 401})
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /api/v1/namespaces/{ns}/pods", guard(func(w http.ResponseWriter, r *http.Request) {
		ns := r.PathValue("ns")
		lim, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		start := 0
		if c := r.URL.Query().Get("continue"); c != "" {
			b, err := base64.RawURLEncoding.DecodeString(c)
			if err != nil {
				writeJSON(w, 410, map[string]any{"kind": "Status", "status": "Failure", "message": "The provided continue parameter is too old", "reason": "Expired", "code": 410})
				return
			}
			start, _ = strconv.Atoi(strings.TrimPrefix(string(b), "start:"))
		}
		sel := strings.TrimPrefix(r.URL.Query().Get("labelSelector"), "app=")
		var all []map[string]any
		for i := 0; i < 23; i++ {
			p := pod(ns, i)
			if sel == "" || p["metadata"].(map[string]any)["labels"].(map[string]any)["app"] == sel {
				all = append(all, p)
			}
		}
		end := len(all)
		if lim > 0 && start+lim < end {
			end = start + lim
		}
		if start > len(all) {
			start = len(all)
		}
		meta := map[string]any{"resourceVersion": "4499"}
		if end < len(all) {
			meta["continue"] = base64.RawURLEncoding.EncodeToString([]byte("start:" + strconv.Itoa(end)))
			meta["remainingItemCount"] = len(all) - end
		}
		writeJSON(w, 200, map[string]any{"kind": "PodList", "apiVersion": "v1", "metadata": meta, "items": all[start:end]})
	}))
	mux.HandleFunc("GET /api/v1/namespaces/{ns}/pods/{name}/log", guard(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		tail, _ := strconv.Atoi(r.URL.Query().Get("tailLines"))
		if tail <= 0 {
			tail = 200
		}
		var b strings.Builder
		for i := 0; i < tail; i++ {
			lvl := "INFO"
			if i%17 == 0 {
				lvl = "ERROR"
			}
			fmt.Fprintf(&b, "2026-11-25T10:%02d:%02dZ %s %s handled request id=%d\n", i/60%60, i%60, lvl, r.PathValue("name"), i)
		}
		_, _ = w.Write([]byte(b.String()))
	}))
	mux.HandleFunc("GET /apis/apps/v1/namespaces/{ns}/deployments", guard(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var items []any
		for _, app := range []string{"cart", "checkout", "search"} {
			items = append(items, map[string]any{"metadata": map[string]any{"name": app, "namespace": r.PathValue("ns"), "generation": 7},
				"spec":   map[string]any{"replicas": replicas[app], "selector": map[string]any{"matchLabels": map[string]any{"app": app}}},
				"status": map[string]any{"replicas": replicas[app], "readyReplicas": replicas[app] - map[bool]int{true: 1}[app == "checkout"], "updatedReplicas": replicas[app]}})
		}
		writeJSON(w, 200, map[string]any{"kind": "DeploymentList", "apiVersion": "apps/v1", "metadata": map[string]any{}, "items": items})
	}))
	mux.HandleFunc("PATCH /apis/apps/v1/namespaces/{ns}/deployments/{name}/scale", guard(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/merge-patch+json" && ct != "application/strategic-merge-patch+json" {
			writeJSON(w, 415, map[string]any{"kind": "Status", "status": "Failure", "message": "the body of the request was in an unknown format - accepted media types include: application/json-patch+json, application/merge-patch+json, application/apply-patch+yaml", "code": 415})
			return
		}
		var in struct {
			Spec struct{ Replicas int } `json:"spec"`
		}
		_ = readJSON(r, &in)
		mu.Lock()
		defer mu.Unlock()
		if _, ok := replicas[r.PathValue("name")]; !ok {
			writeJSON(w, 404, map[string]any{"kind": "Status", "status": "Failure", "message": `deployments.apps "` + r.PathValue("name") + `" not found`, "reason": "NotFound", "code": 404})
			return
		}
		replicas[r.PathValue("name")] = in.Spec.Replicas
		e.inc("kube.scale")
		writeJSON(w, 200, map[string]any{"kind": "Scale", "apiVersion": "autoscaling/v1", "spec": map[string]any{"replicas": in.Spec.Replicas}})
	}))
	serveSpec(mux, "/openapi/v2", func(r *http.Request) map[string]any {
		return specDoc{title: "Kubernetes", desc: "", server: "/",
			schemes:  map[string]any{"BearerToken": map[string]any{"type": "apiKey", "name": "authorization", "in": "header", "description": "Bearer Token authentication"}},
			security: []string{"BearerToken"},
			ops: []op{
				{method: "GET", path: "/api/v1/namespaces/{namespace}/pods", id: "listCoreV1NamespacedPod", summary: "list or watch objects of kind Pod", tag: "core_v1",
					params: []param{pp("namespace"), q("limit", 0), q("continue", nil), q("labelSelector", nil)}},
				{method: "GET", path: "/api/v1/namespaces/{namespace}/pods/{name}/log", id: "readCoreV1NamespacedPodLog", summary: "read log of the specified Pod", tag: "core_v1",
					params: []param{pp("namespace"), pp("name"), q("tailLines", 0)}},
				{method: "GET", path: "/apis/apps/v1/namespaces/{namespace}/deployments", id: "listAppsV1NamespacedDeployment", summary: "list or watch objects of kind Deployment", tag: "apps_v1",
					params: []param{pp("namespace")}},
				{method: "PATCH", path: "/apis/apps/v1/namespaces/{namespace}/deployments/{name}/scale", id: "patchAppsV1NamespacedDeploymentScale", summary: "partially update scale of the specified Deployment", tag: "apps_v1",
					params: []param{pp("namespace"), pp("name")}, body: &body{ct: "application/merge-patch+json", example: map[string]any{"spec": map[string]any{"replicas": 3}}}},
			}}.v2()
	})
	return mux
}

// graphqlAPI is a GraphQL endpoint: one POST URL, errors in a 200 body,
// Relay cursor connections, a mutation that writes.
func (e *Env) graphqlAPI() http.Handler {
	mux := http.NewServeMux()
	var mu sync.Mutex
	cancelled := map[string]bool{}
	mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
		if bearer(r) != GraphQLToken {
			writeJSON(w, 401, map[string]any{"errors": []any{map[string]any{"message": "Unauthenticated", "extensions": map[string]any{"code": "UNAUTHENTICATED"}}}})
			return
		}
		var in struct {
			Query         string         `json:"query"`
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := readJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]any{"errors": []any{map[string]any{"message": "Must provide query string."}}})
			return
		}
		qs := strings.Join(strings.Fields(in.Query), " ")
		switch {
		case strings.Contains(qs, "mutation") && strings.Contains(qs, "cancelOrder"):
			id, _ := in.Variables["id"].(string)
			mu.Lock()
			cancelled[id] = true
			mu.Unlock()
			e.inc("graphql.cancel")
			writeJSON(w, 200, map[string]any{"data": map[string]any{"cancelOrder": map[string]any{"order": map[string]any{"id": id, "status": "CANCELLED"}, "userErrors": []any{}}}})
		case strings.Contains(qs, "orders("):
			first := 10
			if f, ok := in.Variables["first"].(float64); ok {
				first = int(f)
			}
			start := 0
			if a, ok := in.Variables["after"].(string); ok && a != "" {
				b, _ := base64.StdEncoding.DecodeString(a)
				start, _ = strconv.Atoi(strings.TrimPrefix(string(b), "cursor:"))
				start++
			}
			var edges []any
			total := 31
			end := start + first
			if end > total {
				end = total
			}
			mu.Lock()
			for i := start; i < end; i++ {
				id := fmt.Sprintf("gid://shop/Order/%d", 5000+i)
				st := []string{"PAID", "FULFILLED", "PENDING"}[i%3]
				if cancelled[id] {
					st = "CANCELLED"
				}
				edges = append(edges, map[string]any{"cursor": base64.StdEncoding.EncodeToString([]byte("cursor:" + strconv.Itoa(i))),
					"node": map[string]any{"id": id, "name": fmt.Sprintf("#%d", 5000+i), "status": st, "totalPrice": map[string]any{"amount": fmt.Sprintf("%d.00", 20+i), "currencyCode": "USD"},
						"lineItems": map[string]any{"edges": []any{map[string]any{"node": map[string]any{"title": "Widget", "quantity": i%3 + 1}}}}}})
			}
			mu.Unlock()
			var endCursor any
			if len(edges) > 0 {
				endCursor = edges[len(edges)-1].(map[string]any)["cursor"]
			}
			writeJSON(w, 200, map[string]any{"data": map[string]any{"orders": map[string]any{"edges": edges, "pageInfo": map[string]any{"hasNextPage": end < total, "endCursor": endCursor}}}})
		default:
			writeJSON(w, 200, map[string]any{"errors": []any{map[string]any{"message": "Cannot query field on type \"QueryRoot\".", "locations": []any{map[string]any{"line": 1, "column": 3}}}}, "data": nil})
		}
	})
	return mux
}

// flaky models what goes wrong in production: 429 with Retry-After,
// a 503 that clears on retry, a 500 HTML error page, a slow endpoint,
// an async export job (202 + Location, poll until done, then download
// a CSV), a PDF download and a response too big for any agent.
func (e *Env) flaky() http.Handler {
	mux := http.NewServeMux()
	var mu sync.Mutex
	jobs := map[string]int{}
	guard := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Api-Key") != FlakyKey {
				writeJSON(w, 401, map[string]any{"error": "missing or wrong X-Api-Key"})
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /v1/limited", guard(func(w http.ResponseWriter, r *http.Request) {
		n := e.inc("flaky.limited")
		if n%3 != 0 { // two 429s, then a success
			w.Header().Set("Retry-After", "1")
			writeJSON(w, 429, map[string]any{"error": "rate_limited", "message": "Too many requests"})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "attempt": n})
	}))
	mux.HandleFunc("GET /v1/unstable", guard(func(w http.ResponseWriter, r *http.Request) {
		n := e.inc("flaky.unstable")
		if n%2 == 1 {
			writeJSON(w, 503, map[string]any{"error": "unavailable"})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	}))
	// echo returns what it received, as debugging endpoints and some error
	// pages do; credentials in it must never reach an agent.
	mux.HandleFunc("GET /v1/echo", guard(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"headers": r.Header, "query": r.URL.Query()})
	}))
	mux.HandleFunc("GET /v1/broken", guard(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(500)
		_, _ = w.Write([]byte("<html><body><h1>500 Internal Server Error</h1><pre>java.lang.NullPointerException\n\tat com.acme.Report.render(Report.java:42)</pre></body></html>"))
	}))
	mux.HandleFunc("GET /v1/slow", guard(func(w http.ResponseWriter, r *http.Request) {
		ms, _ := strconv.Atoi(r.URL.Query().Get("ms"))
		if ms > 5000 {
			ms = 5000
		}
		time.Sleep(time.Duration(ms) * time.Millisecond)
		writeJSON(w, 200, map[string]any{"waited_ms": ms})
	}))
	mux.HandleFunc("POST /v1/exports", guard(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		id := fmt.Sprintf("exp_%d", len(jobs)+1)
		jobs[id] = 0
		mu.Unlock()
		w.Header().Set("Location", "/v1/exports/"+id)
		writeJSON(w, 202, map[string]any{"id": id, "status": "queued"})
	}))
	mux.HandleFunc("GET /v1/exports/{id}", guard(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n, ok := jobs[r.PathValue("id")]
		if ok {
			jobs[r.PathValue("id")] = n + 1
		}
		mu.Unlock()
		if !ok {
			writeJSON(w, 404, map[string]any{"error": "no such export"})
			return
		}
		if n < 2 {
			writeJSON(w, 200, map[string]any{"id": r.PathValue("id"), "status": "running", "progress": (n + 1) * 33})
			return
		}
		writeJSON(w, 200, map[string]any{"id": r.PathValue("id"), "status": "done", "progress": 100, "download_url": "/v1/exports/" + r.PathValue("id") + "/file"})
	}))
	mux.HandleFunc("GET /v1/exports/{id}/file", guard(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="export.csv"`)
		_, _ = w.Write([]byte("region,revenue\nemea,1200\namer,3400\napac,900\n"))
	}))
	mux.HandleFunc("GET /v1/files/{id}", guard(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		b := []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
		for i := 0; i < 4096; i++ {
			b = append(b, byte(i*13))
		}
		_, _ = w.Write(b)
	}))
	mux.HandleFunc("GET /v1/huge", guard(func(w http.ResponseWriter, r *http.Request) {
		var rows []any
		for i := 0; i < 6000; i++ {
			rows = append(rows, map[string]any{"id": i, "sku": fmt.Sprintf("SKU-%05d", i), "name": "Item " + strconv.Itoa(i), "price": float64(i) * 1.25,
				"description": longText(20), "warehouse": map[string]any{"id": i % 9, "name": "WH-" + strconv.Itoa(i%9), "address": "Somewhere"}})
		}
		writeJSON(w, 200, map[string]any{"items": rows, "count": len(rows)})
	}))
	serveSpec(mux, "/openapi.json", func(r *http.Request) map[string]any {
		return specDoc{title: "Reports API", desc: "", server: origin(r),
			schemes:  map[string]any{"apiKey": map[string]any{"type": "apiKey", "in": "header", "name": "X-Api-Key"}},
			security: []string{"apiKey"},
			ops: []op{
				{method: "GET", path: "/v1/limited", id: "getLimited", summary: "A rate-limited read"},
				{method: "GET", path: "/v1/unstable", id: "getUnstable", summary: "Sometimes unavailable"},
				{method: "GET", path: "/v1/echo", id: "echo", summary: "Echo the request"},
				{method: "GET", path: "/v1/broken", id: "getBroken", summary: "Always fails"},
				{method: "GET", path: "/v1/slow", id: "getSlow", summary: "Slow read", params: []param{q("ms", 100)}},
				{method: "POST", path: "/v1/exports", id: "startExport", summary: "Start a revenue export", body: jsonBody(map[string]any{"format": "csv"})},
				{method: "GET", path: "/v1/exports/{id}", id: "getExport", summary: "Export status", params: []param{pp("id")}},
				{method: "GET", path: "/v1/exports/{id}/file", id: "downloadExport", summary: "Download the export", params: []param{pp("id")}, respCT: "text/csv"},
				{method: "GET", path: "/v1/files/{id}", id: "getFile", summary: "Download a PDF", params: []param{pp("id")}, respCT: "application/pdf"},
				{method: "GET", path: "/v1/huge", id: "getCatalog", summary: "Whole catalog, unpaginated"},
			}}.v3()
	})
	return mux
}

// weather takes its key in the query string, like OpenWeatherMap.
func (e *Env) weather() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /data/2.5/weather", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("appid") != WeatherKey {
			writeJSON(w, 401, map[string]any{"cod": 401, "message": "Invalid API key. Please see https://openweathermap.org/faq#error401 for more info."})
			return
		}
		city := r.URL.Query().Get("q")
		if city == "" {
			writeJSON(w, 400, map[string]any{"cod": "400", "message": "Nothing to geocode"})
			return
		}
		writeJSON(w, 200, map[string]any{"coord": map[string]any{"lon": -0.13, "lat": 51.51}, "weather": []any{map[string]any{"id": 300, "main": "Drizzle", "description": "light intensity drizzle", "icon": "09d"}},
			"base": "stations", "main": map[string]any{"temp": 280.32, "feels_like": 278.1, "pressure": 1012, "humidity": 81}, "visibility": 10000,
			"wind": map[string]any{"speed": 4.1, "deg": 80}, "clouds": map[string]any{"all": 90}, "dt": 1795000000, "sys": map[string]any{"country": "GB", "sunrise": 1794990000, "sunset": 1795020000},
			"id": 2643743, "name": city, "cod": 200})
	})
	serveSpec(mux, "/openapi.json", func(r *http.Request) map[string]any {
		return specDoc{title: "Weather API", server: origin(r),
			schemes:  map[string]any{"appid": map[string]any{"type": "apiKey", "in": "query", "name": "appid"}},
			security: []string{"appid"},
			ops:      []op{{method: "GET", path: "/data/2.5/weather", id: "currentWeather", summary: "Current weather for a city", params: []param{qr("q", "London")}}}}.v3()
	})
	return mux
}
