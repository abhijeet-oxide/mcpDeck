package scenarios

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/e2e/mockenv"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// The harness drives a real mcpdeck binary the way a new user does: the
// CLI for import, toolkit and tool steps, the same HTTP API the UI calls
// for everything else, and a real MCP client for the agent's side.

var reportPath = flag.String("report", "", "write a JSON pass/fail report of every scenario to this file")

type harness struct {
	bin, data string
	logFile   string
	port      int
	base      string
	cliToken  string
	env       *mockenv.Env
	cmd       *exec.Cmd
	mu        sync.Mutex
	results   []result
}

type result struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Passed   bool   `json:"passed"`
	Failure  string `json:"failure,omitempty"`
}

var h *harness

func TestMain(m *testing.M) {
	flag.Parse()
	// The OS matrix in CI runs unit tests only; the scenarios job runs these.
	if os.Getenv("MCPDECK_SKIP_SCENARIOS") != "" {
		fmt.Println("scenarios skipped (MCPDECK_SKIP_SCENARIOS is set)")
		os.Exit(0)
	}
	code, err := run(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scenarios:", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func run(m *testing.M) (int, error) {
	tmp, err := os.MkdirTemp("", "mcpdeck-scenarios-")
	if err != nil {
		return 1, err
	}
	defer os.RemoveAll(tmp)
	bin := os.Getenv("MCPDECK_BIN")
	if bin == "" {
		bin = filepath.Join(tmp, "mcpdeck")
		if runtime.GOOS == "windows" {
			bin += ".exe"
		}
		build := exec.Command("go", "build", "-o", bin, "../../cmd/mcpdeck")
		build.Stdout, build.Stderr = os.Stderr, os.Stderr
		if err := build.Run(); err != nil {
			return 1, fmt.Errorf("build: %w", err)
		}
	}
	env, err := mockenv.Start()
	if err != nil {
		return 1, err
	}
	defer env.Close()
	h = &harness{bin: bin, data: filepath.Join(tmp, "data"), env: env}
	if err := h.start(); err != nil {
		return 1, err
	}
	defer h.stop()
	code := m.Run()
	if *reportPath != "" {
		h.writeReport(*reportPath)
	}
	return code, nil
}

func freePort() int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 47911
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}

func (h *harness) start() error {
	h.port = freePort()
	h.base = "http://127.0.0.1:" + strconv.Itoa(h.port)
	h.cmd = exec.Command(h.bin, "serve", "--no-open", "--port", strconv.Itoa(h.port), "--data", h.data)
	h.cmd.Env = append(os.Environ(), "MCPDECK_NO_KEYCHAIN=1")
	h.logFile = filepath.Join(filepath.Dir(h.data), "server.log")
	logf, err := os.Create(h.logFile)
	if err != nil {
		return err
	}
	h.cmd.Stdout, h.cmd.Stderr = logf, logf
	if err := h.cmd.Start(); err != nil {
		return err
	}
	for i := 0; i < 100; i++ {
		b, err := os.ReadFile(filepath.Join(h.data, "instance.json"))
		if err == nil {
			var in struct {
				CLIToken string `json:"cliToken"`
			}
			if json.Unmarshal(b, &in) == nil && in.CLIToken != "" {
				h.cliToken = in.CLIToken
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	b, _ := os.ReadFile(h.logFile)
	return fmt.Errorf("mcpdeck didn't start: %s", b)
}

func (h *harness) stop() {
	if h.cmd != nil && h.cmd.Process != nil {
		_ = h.cmd.Process.Kill()
		_, _ = h.cmd.Process.Wait()
	}
}

func (h *harness) writeReport(path string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sort.Slice(h.results, func(i, j int) bool { return h.results[i].Name < h.results[j].Name })
	pass := 0
	byCat := map[string][2]int{}
	for _, r := range h.results {
		c := byCat[r.Category]
		c[1]++
		if r.Passed {
			pass++
			c[0]++
		}
		byCat[r.Category] = c
	}
	b, _ := json.MarshalIndent(map[string]any{"passed": pass, "total": len(h.results), "byCategory": byCat, "scenarios": h.results}, "", "  ")
	_ = os.WriteFile(path, b, 0o644)
}

// ───────── CLI and HTTP API ─────────

// cli runs mcpdeck with this instance's data folder.
func (h *harness) cli(t T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(h.bin, append(args, "--data", h.data)...)
	cmd.Env = append(os.Environ(), "MCPDECK_NO_KEYCHAIN=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("mcpdeck %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

// cliCmd prepares a CLI command for steps that need stdin.
func (h *harness) cliCmd(args ...string) *exec.Cmd {
	cmd := exec.Command(h.bin, append(args, "--data", h.data)...)
	cmd.Env = append(os.Environ(), "MCPDECK_NO_KEYCHAIN=1")
	return cmd
}

func (h *harness) mustCLI(t T, args ...string) string {
	t.Helper()
	out, err := h.cli(t, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// api calls the HTTP API the UI uses.
func (h *harness) api(method, path string, in, out any) error {
	return h.apiAs("", method, path, in, out)
}

// apiAs calls the HTTP API as the agent behind the admin MCP server.
func (h *harness) apiAs(agentName, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, h.base+path, body)
	req.Header.Set("Authorization", "Bearer "+h.cliToken)
	req.Header.Set("Content-Type", "application/json")
	if agentName != "" {
		req.Header.Set("X-Mcpdeck-Agent", agentName)
	}
	res, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		var e struct{ Error, Message string }
		_ = json.Unmarshal(b, &e)
		return fmt.Errorf("%s %s: %d %s%s", method, path, res.StatusCode, e.Error, e.Message)
	}
	if out != nil && len(b) > 0 {
		return json.Unmarshal(b, out)
	}
	return nil
}

func (h *harness) mustAPI(t T, method, path string, in, out any) {
	t.Helper()
	if err := h.api(method, path, in, out); err != nil {
		t.Fatal(err)
	}
}

type apiDetail struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Auth     map[string]any `json:"auth"`
	Requests []reqRow       `json:"requests"`
	Vars     []struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Source string `json:"source"`
		IsSet  bool   `json:"isSet"`
	} `json:"variables"`
}

// v returns a variable's value (empty for secrets).
func (a apiDetail) v(name string) string {
	for _, x := range a.Vars {
		if x.Name == name {
			return x.Value
		}
	}
	return ""
}

type reqRow struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Method   string `json:"method"`
	URL      string `json:"url"`
	Body     string `json:"body"`
	BodyType string `json:"bodyType"`
	Access   string `json:"access"`
}

func (h *harness) apiByName(t T, name string) apiDetail {
	t.Helper()
	var list []struct{ ID, Name string }
	h.mustAPI(t, "GET", "/api/apis", nil, &list)
	for _, a := range list {
		if a.Name == name {
			var d apiDetail
			h.mustAPI(t, "GET", "/api/apis/"+a.ID, nil, &d)
			return d
		}
	}
	t.Fatalf("no API named %s", name)
	return apiDetail{}
}

func (a apiDetail) req(t T, name string) reqRow {
	t.Helper()
	for _, r := range a.Requests {
		if r.Name == name {
			return r
		}
	}
	var names []string
	for _, r := range a.Requests {
		names = append(names, r.Name)
	}
	t.Fatalf("API %s has no request %s (has %s)", a.Name, name, strings.Join(names, ", "))
	return reqRow{}
}

func (h *harness) setVar(t T, apiID, name, value string, secret bool) {
	t.Helper()
	src := "literal"
	if secret {
		src = "secret"
	}
	// Like the UI: edit the variable in the active environment.
	var d struct {
		ActiveEnvID string `json:"activeEnvId"`
		Variables   []struct {
			ID    string `json:"id"`
			EnvID string `json:"envId"`
			Name  string `json:"name"`
		} `json:"variables"`
	}
	h.mustAPI(t, "GET", "/api/apis/"+apiID, nil, &d)
	body := map[string]any{"name": name, "value": value, "source": src, "envId": d.ActiveEnvID}
	for _, v := range d.Variables {
		if v.Name == name && v.EnvID == d.ActiveEnvID {
			body["id"] = v.ID
		}
	}
	h.mustAPI(t, "PUT", "/api/apis/"+apiID+"/variables", body, nil)
}

func (h *harness) setAuth(t T, apiID string, auth map[string]any) {
	t.Helper()
	h.mustAPI(t, "PATCH", "/api/apis/"+apiID, map[string]any{"auth": auth}, nil)
}

func (h *harness) patchAPI(t T, apiID string, patch map[string]any) {
	t.Helper()
	h.mustAPI(t, "PATCH", "/api/apis/"+apiID, patch, nil)
}

// editRequest changes fields of a saved request, as the request editor does.
func (h *harness) editRequest(t T, id string, change map[string]any) {
	t.Helper()
	var cur map[string]any
	var all []struct{ ID string }
	h.mustAPI(t, "GET", "/api/apis", nil, &all)
	for _, a := range all {
		var d struct {
			Requests []map[string]any `json:"requests"`
		}
		h.mustAPI(t, "GET", "/api/apis/"+a.ID, nil, &d)
		for _, r := range d.Requests {
			if r["id"] == id {
				cur = r
			}
		}
	}
	if cur == nil {
		t.Fatalf("request %s not found", id)
	}
	for k, v := range change {
		cur[k] = v
	}
	h.mustAPI(t, "PUT", "/api/requests/"+id, cur, nil)
}

// importText imports a spec, collection or curl text into a new API with the CLI.
func (h *harness) importText(t T, apiName, text string, extra ...string) string {
	t.Helper()
	dir, _ := os.MkdirTemp("", "import")
	defer os.RemoveAll(dir)
	f := filepath.Join(dir, "import.txt")
	if err := os.WriteFile(f, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return h.mustCLI(t, append([]string{"import", f, "--api", apiName}, extra...)...)
}

func (h *harness) importURL(t T, apiName, url string) string {
	t.Helper()
	if strings.HasPrefix(url, "https://") {
		return h.mustCLI(t, "import", url, "--api", apiName, "--ca-file", h.caFile())
	}
	return h.mustCLI(t, "import", url, "--api", apiName)
}

var caOnce sync.Once
var caPath string

// caFile writes the mock systems' CA, as a person has their company CA.
func (h *harness) caFile() string {
	caOnce.Do(func() {
		caPath = filepath.Join(filepath.Dir(h.data), "company-ca.pem")
		_ = os.WriteFile(caPath, h.env.CACert, 0o600)
	})
	return caPath
}

// markRead marks a POST that only reads (a search, a query) as read, as a
// person does on the request's Access setting.
func (h *harness) markRead(t T, reqID, reason string) {
	t.Helper()
	h.mustAPI(t, "POST", "/api/requests/"+reqID+"/access", map[string]any{"access": "read", "reason": reason}, nil)
}

// toolkit creates a toolkit through the HTTP API the UI uses.
func (h *harness) toolkit(t T, name string, apis ...string) {
	t.Helper()
	scope := apis
	if scope == nil {
		scope = []string{}
	}
	h.mustAPI(t, "POST", "/api/toolkits", map[string]any{"name": name, "buildScope": scope}, nil)
}

type saveRes struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Version struct {
		ID          string `json:"id"`
		Access      string `json:"access"`
		TestResults []struct {
			Name   string `json:"name"`
			Passed bool   `json:"passed"`
			Error  string `json:"error"`
		} `json:"testResults"`
	} `json:"version"`
}

// saveTool saves a tool as the person (UI editor) and approves it when it
// waits for review, as a person would after reading it.
func (h *harness) saveTool(t T, toolkit, src string, tests ...map[string]any) saveRes {
	t.Helper()
	if tests == nil {
		tests = []map[string]any{}
	}
	var res saveRes
	h.mustAPI(t, "POST", "/api/toolkits/"+toolkit+"/save", map[string]any{"source": src, "tests": tests}, &res)
	for _, tr := range res.Version.TestResults {
		if !tr.Passed {
			t.Fatalf("tool test %q failed while saving: %s", tr.Name, tr.Error)
		}
	}
	if res.Status == "pending" {
		h.mustAPI(t, "POST", "/api/versions/"+res.Version.ID+"/approve", nil, nil)
	}
	return res
}

// connect gives an agent access and returns its token.
func (h *harness) connect(t T, toolkit, access string, build bool) string {
	t.Helper()
	var out struct {
		Result struct {
			Token string `json:"token"`
		} `json:"result"`
	}
	h.mustAPI(t, "POST", "/api/connect", map[string]any{"client": "claude-code", "toolkitId": toolkit, "access": access, "canBuild": build,
		"name": fmt.Sprintf("%s-%s-%d", toolkit, access, time.Now().UnixNano())}, &out)
	if out.Result.Token == "" {
		t.Fatal("connect returned no token")
	}
	return out.Result.Token
}

type bearerRT struct {
	tok string
	rt  http.RoundTripper
}

func (b bearerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.tok)
	return b.rt.RoundTrip(r)
}

type agent struct {
	t  *testing.T
	cs *sdk.ClientSession
	h  *harness
}

// agentFor connects a real MCP client to a toolkit with a fresh grant.
func (h *harness) agentFor(t *testing.T, toolkit, access string) *agent {
	t.Helper()
	return h.agentWithToken(t, toolkit, h.connect(t, toolkit, access, false), "")
}

func (h *harness) builderFor(t *testing.T, toolkit, access string) *agent {
	t.Helper()
	return h.agentWithToken(t, toolkit, h.connect(t, toolkit, access, true), "/build")
}

func (h *harness) agentWithToken(t *testing.T, toolkit, token, suffix string) *agent {
	t.Helper()
	cl := sdk.NewClient(&sdk.Implementation{Name: "scenario-agent", Version: "1"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cs, err := cl.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: h.base + "/mcp/" + toolkit + suffix,
		HTTPClient: &http.Client{Transport: bearerRT{token, http.DefaultTransport}}}, nil)
	if err != nil {
		t.Fatalf("MCP connect to %s: %v", toolkit, err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return &agent{t: t, cs: cs, h: h}
}

func (a *agent) tools() []string {
	a.t.Helper()
	res, err := a.cs.ListTools(context.Background(), nil)
	if err != nil {
		a.t.Fatal(err)
	}
	var out []string
	for _, x := range res.Tools {
		out = append(out, x.Name)
	}
	return out
}

// call calls a tool and returns its text; every result is leak-checked.
func (a *agent) call(name string, args any) (string, bool) {
	a.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	res, err := a.cs.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		if strings.Contains(err.Error(), "unknown tool") {
			return "this agent can't see tool " + name, true
		}
		a.t.Fatalf("call %s: %v", name, err)
	}
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	text := strings.Join(parts, "\n")
	a.h.noLeaks(a.t, name, text)
	return text, res.IsError
}

// ok calls a tool, fails on an error result, and decodes JSON output.
func (a *agent) ok(name string, args any) any {
	a.t.Helper()
	text, isErr := a.call(name, args)
	if isErr {
		a.t.Fatalf("%s returned an error: %s", name, text)
	}
	return decodeOut(text)
}

// fails calls a tool and expects an error result containing want.
func (a *agent) fails(name string, args any, want ...string) string {
	a.t.Helper()
	text, isErr := a.call(name, args)
	if !isErr {
		a.t.Fatalf("%s should have failed, got: %s", name, clip(text, 400))
	}
	for _, w := range want {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(w)) {
			a.t.Fatalf("%s error should mention %q, got: %s", name, w, clip(text, 400))
		}
	}
	return text
}

func decodeOut(text string) any {
	var v any
	if err := json.Unmarshal([]byte(text), &v); err == nil {
		return v
	}
	// Build tools prefix a summary line; the JSON follows.
	if i := strings.IndexAny(text, "{["); i >= 0 {
		if err := json.Unmarshal([]byte(text[i:]), &v); err == nil {
			return v
		}
	}
	return text
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// noLeaks fails when any credential or issued token appears in text.
func (h *harness) noLeaks(t T, where, text string) {
	t.Helper()
	secrets := append(mockenv.Canaries(), h.env.IssuedTokens()...)
	for _, s := range secrets {
		if s != "" && strings.Contains(text, s) {
			t.Fatalf("SECRET LEAK in %s: %q appeared in what the agent saw: %s", where, clip(s, 12), clip(text, 300))
		}
	}
}

// noLeaksInCallLog checks the recorded calls a person sees on Activity.
// lastCalls returns the newest recorded upstream calls, for debugging.
func (h *harness) lastCalls(t T, n int) string {
	var calls []map[string]any
	h.mustAPI(t, "GET", "/api/calls?all=1&limit="+strconv.Itoa(n), nil, &calls)
	return asJSON(calls)
}

func (h *harness) noLeaksInCallLog(t T) {
	t.Helper()
	var calls []map[string]any
	h.mustAPI(t, "GET", "/api/calls?all=1&limit=500", nil, &calls)
	b, _ := json.Marshal(calls)
	h.noLeaks(t, "the call log", string(b))
}

// ───────── catalog ─────────

type scenario struct {
	name, category string
	run            func(t *testing.T)
}

var catalog []scenario

func add(category, name string, run func(t *testing.T)) {
	catalog = append(catalog, scenario{name: category + "/" + name, category: category, run: run})
}

func TestScenarios(t *testing.T) {
	for _, s := range catalog {
		s := s
		t.Run(s.name, func(t *testing.T) {
			defer func() {
				r := result{Name: s.name, Category: s.category, Passed: !t.Failed()}
				if p := recover(); p != nil {
					r.Passed, r.Failure = false, fmt.Sprint(p)
					h.record(r)
					t.Errorf("scenario panicked: %v", p)
					return
				}
				h.record(r)
			}()
			s.run(t)
		})
	}
}

func (h *harness) record(r result) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.results = append(h.results, r)
}

var _ = errors.New

// T is the part of *testing.T the helpers use, so onboarding can record a
// failure without failing whichever scenario triggered it.
type T interface {
	Helper()
	Fatalf(format string, args ...any)
	Fatal(args ...any)
	Logf(format string, args ...any)
}
