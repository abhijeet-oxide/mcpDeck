// Package tool runs TypeScript tools in a sandbox (TOOL-1, TOOL-2, TOOL-4).
//
// Source is stripped of types by esbuild and run in goja, a pure-Go JS
// engine with no I/O. The API clients and call() are the only way out, and
// they go through internal/proxy.
package tool

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/tokens"
	"github.com/dlclark/regexp2/v2"
	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/google/jsonschema-go/jsonschema"
)

//go:embed prelude.js
var prelude string

//go:embed pm.js
var pmPrelude string

// Limits (TOOL-6).
const (
	DefaultTimeout   = 10 * time.Second
	DefaultMaxOutput = 64 << 10
	memoryCap        = 256 << 20
	maxCallDepth     = 2000
)

// Definition is the `tool` export (TOOL-1).
type Definition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	UseWhen     string          `json:"useWhen"`
	DontUseWhen string          `json:"dontUseWhen"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Compile strips TypeScript types and makes a CommonJS module.
func Compile(src string) (string, error) {
	res := api.Transform(src, api.TransformOptions{Loader: api.LoaderTS, Format: api.FormatCommonJS, Target: api.ES2020, Sourcefile: "tool.ts"})
	if len(res.Errors) > 0 {
		var msgs []string
		for _, e := range res.Errors {
			loc := ""
			if e.Location != nil {
				loc = fmt.Sprintf("line %d:%d: ", e.Location.Line, e.Location.Column)
			}
			msgs = append(msgs, loc+e.Text)
		}
		return "", fmt.Errorf("syntax error: %s", strings.Join(msgs, "; "))
	}
	return string(res.Code), nil
}

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

// Load reads the tool definition from source without calling any API.
func Load(src string) (Definition, error) {
	js, err := Compile(src)
	if err != nil {
		return Definition{}, err
	}
	vm, err := newVM()
	if err != nil {
		return Definition{}, err
	}
	done := make(chan struct{})
	t := time.AfterFunc(time.Second, func() { vm.Interrupt("timeout") })
	defer t.Stop()
	defer close(done)
	if _, err := vm.RunString(js); err != nil {
		return Definition{}, fmt.Errorf("loading the tool file: %s", jsErr(err))
	}
	exp := vm.Get("module").ToObject(vm).Get("exports").ToObject(vm)
	tv := exp.Get("tool")
	if tv == nil || goja.IsUndefined(tv) {
		return Definition{}, errors.New(`the file must export const tool = { name, description, inputSchema }`)
	}
	b, err := json.Marshal(tv.Export())
	if err != nil {
		return Definition{}, err
	}
	var d Definition
	if err := json.Unmarshal(b, &d); err != nil {
		return Definition{}, fmt.Errorf("tool export: %w", err)
	}
	if fn := exp.Get("default"); fn == nil || goja.IsUndefined(fn) {
		return d, errors.New("the file must have `export default async function (input) { ... }`")
	}
	if !nameRe.MatchString(d.Name) {
		return d, fmt.Errorf("tool name %q must be snake_case, 3–64 characters, starting with a letter", d.Name)
	}
	if strings.TrimSpace(d.Description) == "" {
		return d, errors.New("tool.description is empty; say what the tool returns in one sentence")
	}
	if len(d.InputSchema) == 0 || string(d.InputSchema) == "null" {
		d.InputSchema = json.RawMessage(`{"type":"object","properties":{}}`)
	}
	var sch map[string]any
	if err := json.Unmarshal(d.InputSchema, &sch); err != nil || sch["type"] != "object" {
		return d, errors.New(`tool.inputSchema must be a JSON Schema with "type": "object"`)
	}
	return d, nil
}

// ValidateInput checks input against the tool's input schema.
func ValidateInput(schema json.RawMessage, input any) error {
	var s jsonschema.Schema
	if err := json.Unmarshal(schema, &s); err != nil {
		return fmt.Errorf("input schema: %w", err)
	}
	rs, err := s.Resolve(nil)
	if err != nil {
		return fmt.Errorf("input schema: %w", err)
	}
	// Normalise through JSON so numbers are float64 and maps are map[string]any.
	b, _ := json.Marshal(input)
	var inst any
	_ = json.Unmarshal(b, &inst)
	if inst == nil {
		inst = map[string]any{}
	}
	if err := rs.Validate(inst); err != nil {
		return fmt.Errorf("input doesn't match the tool's inputSchema: %v. Fix the input and call once more", err)
	}
	return nil
}

// regexOnce caps backtracking regex matches (ReDoS) for every runtime.
var regexOnce sync.Once

func newVM() (*goja.Runtime, error) {
	regexOnce.Do(func() { regexp2.DefaultMatchTimeout = 2 * time.Second })
	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	vm.SetMaxCallStackSize(maxCallDepth)
	if _, err := vm.RunString(prelude); err != nil {
		return nil, err
	}
	return vm, nil
}

func jsErr(err error) string {
	var ex *goja.Exception
	if errors.As(err, &ex) {
		v := ex.Value()
		if o, ok := v.(*goja.Object); ok {
			if m := o.Get("message"); m != nil && !goja.IsUndefined(m) {
				return m.String()
			}
		}
		return v.String()
	}
	var ie *goja.InterruptedError
	if errors.As(err, &ie) {
		return fmt.Sprint(ie.Value())
	}
	var so *goja.StackOverflowError
	if errors.As(err, &so) {
		return "call stack too deep (recursion limit)"
	}
	return err.Error()
}

// Client is one API and its requests, exposed as a global in tool code.
type Client struct {
	API      store.API
	Requests []store.Request
}

// Sender is the proxy.
type Sender interface {
	Do(ctx context.Context, c proxy.Call) (*proxy.Response, error)
}

// Env is what a run may use.
type Env struct {
	Clients   []Client
	Sender    Sender
	Base      proxy.Call // Access, Actor, SessionID, ParentID, ToolkitID, Examples
	EnvByAPI  map[string]string
	Vars      map[string]string
	Timeout   time.Duration
	MaxOutput int
}

// Upstream is one API call made during a run (TOOL-7 timeline).
type Upstream struct {
	CallID      string `json:"callId"`
	Name        string `json:"name"`
	Method      string `json:"method"`
	URL         string `json:"url"`
	Status      int    `json:"status"`
	StartMS     int64  `json:"startMs"`
	DurationMS  int64  `json:"durationMs"`
	Bytes       int    `json:"bytes"`
	Tokens      int    `json:"tokens"`
	Error       string `json:"error,omitempty"`
	FromExample bool   `json:"fromExample,omitempty"`
}

// Result of one run.
type Result struct {
	Output         any        `json:"output"`
	OutputJSON     string     `json:"outputJson"`
	Console        []string   `json:"console"`
	Calls          []Upstream `json:"calls"`
	Error          string     `json:"error,omitempty"`
	ErrorCode      string     `json:"errorCode,omitempty"`
	DurationMS     int64      `json:"durationMs"`
	OutputBytes    int        `json:"outputBytes"`
	OutputTokens   int        `json:"outputTokens"`
	UpstreamBytes  int        `json:"upstreamBytes"`
	UpstreamTokens int        `json:"upstreamTokens"`
}

type settle struct {
	fn func()
}

// Run calls the tool's default export with input.
func Run(ctx context.Context, src string, input any, env Env) Result {
	js, err := Compile(src)
	if err != nil {
		return Result{Error: err.Error(), Console: []string{}, Calls: []Upstream{}}
	}
	return runJS(ctx, js, input, env)
}

// RunScript runs exploratory code for run_script: a module with a default
// export, or plain statements whose last `return` is the result.
func RunScript(ctx context.Context, src string, input any, env Env) Result {
	if !regexp.MustCompile(`(?m)^\s*export\s+default\b`).MatchString(src) {
		src = "export default async function (input: any) {\n" + src + "\n}"
	}
	return Run(ctx, src, input, env)
}

func runJS(ctx context.Context, js string, input any, env Env) Result {
	res := Result{Console: []string{}, Calls: []Upstream{}}
	if env.Timeout <= 0 {
		env.Timeout = DefaultTimeout
	}
	if env.MaxOutput <= 0 {
		env.MaxOutput = DefaultMaxOutput
	}
	ctx, cancel := context.WithTimeout(ctx, env.Timeout)
	defer cancel()
	start := time.Now()
	vm, err := newVM()
	if err != nil {
		res.Error = err.Error()
		return res
	}
	jsonParse, _ := goja.AssertFunction(vm.Get("JSON").ToObject(vm).Get("parse"))
	apiErrCtor := vm.Get("ApiError").ToObject(vm)

	var mu sync.Mutex
	settled := make(chan settle, 64)
	pending := 0

	logf := func(prefix string) func(goja.FunctionCall) goja.Value {
		return func(c goja.FunctionCall) goja.Value {
			parts := make([]string, 0, len(c.Arguments))
			for _, a := range c.Arguments {
				if o, ok := a.(*goja.Object); ok && o.ClassName() != "Function" {
					b, err := json.Marshal(o.Export())
					if err == nil {
						parts = append(parts, string(b))
						continue
					}
				}
				parts = append(parts, a.String())
			}
			if len(res.Console) < 200 {
				res.Console = append(res.Console, prefix+strings.Join(parts, " "))
			}
			return goja.Undefined()
		}
	}
	console := vm.NewObject()
	_ = console.Set("log", logf(""))
	_ = console.Set("info", logf(""))
	_ = console.Set("warn", logf("warn: "))
	_ = console.Set("error", logf("error: "))
	_ = vm.Set("console", console)

	varsObj := vm.NewObject()
	for k, v := range env.Vars {
		_ = varsObj.Set(k, v)
	}
	_, _ = vm.RunString("") // ensure jobs settle
	_ = vm.Set("vars", varsObj)
	_, _ = vm.RunString(`Object.freeze(vars)`)

	startCall := func(c proxy.Call, label string) goja.Value {
		p, resolve, reject := vm.NewPromise()
		pending++
		go func() {
			t0 := time.Since(start).Milliseconds()
			r, err := env.Sender.Do(ctx, c)
			up := Upstream{Name: label, StartMS: t0}
			var fn func()
			if err != nil {
				up.Error = err.Error()
				code := "error"
				var pe *proxy.Error
				if errors.As(err, &pe) {
					code = pe.Code
				}
				fn = func() {
					e, _ := vm.New(apiErrCtor, vm.ToValue(err.Error()), vm.ToValue(0), goja.Undefined(), vm.ToValue(code))
					_ = reject(e)
				}
			} else {
				up.CallID, up.Method, up.URL, up.Status = r.CallID, r.Method, r.URL, r.Status
				up.DurationMS, up.Bytes, up.Tokens, up.FromExample = r.DurationMS, r.Size, r.Tokens, r.FromExample
				if up.DurationMS == 0 {
					up.DurationMS = time.Since(start).Milliseconds() - t0
				}
				body := string(r.Body)
				status := r.Status
				fn = func() {
					parsed, perr := jsonParse(goja.Undefined(), vm.ToValue(body))
					if perr != nil {
						parsed = vm.ToValue(body)
					}
					if status >= 400 {
						msg := fmt.Sprintf("%s returned %d", label, status)
						e, _ := vm.New(apiErrCtor, vm.ToValue(msg), vm.ToValue(status), parsed, vm.ToValue("http_error"))
						_ = reject(e)
						return
					}
					_ = resolve(parsed)
				}
			}
			mu.Lock()
			res.Calls = append(res.Calls, up)
			mu.Unlock()
			settled <- settle{fn: fn}
		}()
		return vm.ToValue(p)
	}
	argsOf := func(v goja.Value) proxy.Args {
		var a proxy.Args
		if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
			return a
		}
		m, ok := v.Export().(map[string]any)
		if !ok {
			return a
		}
		structured := len(m) > 0
		for k := range m {
			if k != "path" && k != "query" && k != "headers" && k != "body" {
				structured = false
			}
		}
		if !structured {
			a.Flat = m
			return a
		}
		if p, ok := m["path"].(map[string]any); ok {
			a.Path = p
		}
		if q, ok := m["query"].(map[string]any); ok {
			a.Query = q
		}
		if h, ok := m["headers"].(map[string]any); ok {
			a.Headers = map[string]string{}
			for k, v := range h {
				a.Headers[k] = fmt.Sprint(v)
			}
		}
		if b, ok := m["body"]; ok {
			a.Body, a.HasBody = b, true
		}
		return a
	}
	byName := map[string]store.API{}
	for _, cl := range env.Clients {
		cl := cl
		byName[cl.API.Name] = cl.API
		obj := vm.NewObject()
		for _, r := range cl.Requests {
			r := r
			_ = obj.Set(r.Name, func(c goja.FunctionCall) goja.Value {
				call := env.Base
				call.APIID, call.RequestID, call.Args = cl.API.ID, r.ID, argsOf(c.Argument(0))
				call.EnvID = env.EnvByAPI[cl.API.ID]
				return startCall(call, cl.API.Name+"."+r.Name)
			})
		}
		_ = vm.Set(cl.API.Name, obj)
		_, _ = vm.RunString("Object.freeze(" + cl.API.Name + ")")
	}
	_ = vm.Set("call", func(c goja.FunctionCall) goja.Value {
		apiName := c.Argument(0).String()
		route := strings.TrimSpace(c.Argument(1).String())
		a, ok := byName[apiName]
		method, path, _ := strings.Cut(route, " ")
		if !ok || path == "" {
			p, _, reject := vm.NewPromise()
			names := make([]string, 0, len(byName))
			for n := range byName {
				names = append(names, n)
			}
			sort.Strings(names)
			msg := fmt.Sprintf(`call(%q, %q): use call("<api>", "METHOD /path"); APIs here: %s`, apiName, route, strings.Join(names, ", "))
			e, _ := vm.New(apiErrCtor, vm.ToValue(msg), vm.ToValue(0), goja.Undefined(), vm.ToValue("bad_call"))
			_ = reject(e)
			return vm.ToValue(p)
		}
		call := env.Base
		call.APIID, call.Method, call.Path, call.Args = a.ID, strings.ToUpper(method), path, argsOf(c.Argument(2))
		call.EnvID = env.EnvByAPI[a.ID]
		return startCall(call, apiName+" "+strings.ToUpper(method)+" "+path)
	})

	// Watchdogs: timeout and a coarse memory cap (TOOL-6).
	stopWatch := make(chan struct{})
	defer close(stopWatch)
	go func() {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		base := ms.HeapAlloc
		tick := time.NewTicker(25 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stopWatch:
				return
			case <-ctx.Done():
				vm.Interrupt(fmt.Sprintf("timed out after %s (limit %s)", time.Since(start).Round(time.Millisecond), env.Timeout))
				return
			case <-tick.C:
				runtime.ReadMemStats(&ms)
				if ms.HeapAlloc > base+memoryCap {
					vm.Interrupt("memory limit reached; return less data")
					return
				}
			}
		}
	}()

	fail := func(err error) Result {
		res.Error = jsErr(err)
		res.DurationMS = time.Since(start).Milliseconds()
		return drain(res, &mu, settled, &pending)
	}
	if _, err := vm.RunString(js); err != nil {
		return fail(err)
	}
	fnv := vm.Get("module").ToObject(vm).Get("exports").ToObject(vm).Get("default")
	fn, ok := goja.AssertFunction(fnv)
	if !ok {
		res.Error = "the file has no default export function"
		return res
	}
	if input == nil {
		input = map[string]any{}
	}
	inJSON, _ := json.Marshal(input)
	inVal, _ := jsonParse(goja.Undefined(), vm.ToValue(string(inJSON)))
	out, err := fn(goja.Undefined(), inVal)
	if err != nil {
		return fail(err)
	}
	if p, ok := out.Export().(*goja.Promise); ok {
		for p.State() == goja.PromiseStatePending {
			if pending == 0 {
				res.Error = "the tool's promise never settled; await every call"
				res.DurationMS = time.Since(start).Milliseconds()
				return res
			}
			select {
			case s := <-settled:
				pending--
				s.fn()
				if _, err := vm.RunString("void 0"); err != nil {
					return fail(err)
				}
			case <-ctx.Done():
				vm.Interrupt("timeout")
				res.Error = fmt.Sprintf("timed out after %s; make fewer calls or run them with Promise.all", env.Timeout)
				res.DurationMS = time.Since(start).Milliseconds()
				return drain(res, &mu, settled, &pending)
			}
		}
		if p.State() == goja.PromiseStateRejected {
			rv := p.Result()
			msg := rv.String()
			if o, ok := rv.(*goja.Object); ok {
				if m := o.Get("message"); m != nil && !goja.IsUndefined(m) {
					msg = m.String()
				}
				if c := o.Get("code"); c != nil && !goja.IsUndefined(c) {
					res.ErrorCode = c.String()
				}
			}
			res.Error = msg
			res.DurationMS = time.Since(start).Milliseconds()
			return drain(res, &mu, settled, &pending)
		}
		out = p.Result()
	}
	res.DurationMS = time.Since(start).Milliseconds()
	res = drain(res, &mu, settled, &pending)
	var exported any
	if out != nil && !goja.IsUndefined(out) {
		exported = out.Export()
	}
	b, err := json.Marshal(exported)
	if err != nil {
		res.Error = "the return value can't be turned into JSON: " + err.Error()
		return res
	}
	res.OutputJSON = string(b)
	_ = json.Unmarshal(b, &res.Output)
	res.OutputBytes = len(b)
	res.OutputTokens = tokens.Count(res.OutputJSON)
	if res.OutputBytes > env.MaxOutput {
		res.Error = fmt.Sprintf("output is %d KB, over the %d KB limit; return fewer fields or cap lists", res.OutputBytes>>10, env.MaxOutput>>10)
		res.Output, res.OutputJSON = nil, ""
	}
	return res
}

// drain waits briefly for in-flight calls so their timeline entries land,
// then totals upstream sizes.
func drain(res Result, mu *sync.Mutex, settled chan settle, pending *int) Result {
	deadline := time.After(2 * time.Second)
	for *pending > 0 {
		select {
		case <-settled:
			*pending--
		case <-deadline:
			*pending = 0
		}
	}
	mu.Lock()
	defer mu.Unlock()
	sort.Slice(res.Calls, func(i, j int) bool { return res.Calls[i].StartMS < res.Calls[j].StartMS })
	for _, c := range res.Calls {
		res.UpstreamBytes += c.Bytes
		res.UpstreamTokens += c.Tokens
	}
	return res
}

// RunTests runs each test's input through the tool, then its pm.test script
// with `output` and `input` set (TOOL-8).
func RunTests(ctx context.Context, src string, tests []store.ToolTest, env Env) []store.TestResult {
	out := make([]store.TestResult, 0, len(tests))
	for _, t := range tests {
		var input any
		_ = json.Unmarshal(t.Input, &input)
		r := Run(ctx, src, input, env)
		tr := store.TestResult{Name: t.Name, Tokens: r.OutputTokens, Duration: r.DurationMS}
		if r.Error != "" {
			tr.Error = r.Error
			out = append(out, tr)
			continue
		}
		asserts, err := runPM(t.Script, r.OutputJSON, string(t.Input))
		tr.Asserts = asserts
		tr.Passed = err == nil
		if err != nil {
			tr.Error = err.Error()
		}
		for _, a := range asserts {
			if !a.Passed {
				tr.Passed = false
				if tr.Error == "" {
					tr.Error = a.Name + ": " + a.Error
				}
			}
		}
		out = append(out, tr)
	}
	return out
}

func runPM(script, outputJSON, inputJSON string) ([]store.Assert, error) {
	if strings.TrimSpace(script) == "" {
		return nil, nil
	}
	js, err := Compile(script)
	if err != nil {
		return nil, err
	}
	vm, err := newVM()
	if err != nil {
		return nil, err
	}
	t := time.AfterFunc(2*time.Second, func() { vm.Interrupt("test script timed out") })
	defer t.Stop()
	if _, err := vm.RunString(pmPrelude); err != nil {
		return nil, err
	}
	if outputJSON == "" {
		outputJSON = "null"
	}
	if inputJSON == "" {
		inputJSON = "{}"
	}
	if _, err := vm.RunString("var output = " + outputJSON + "; var input = " + inputJSON + ";"); err != nil {
		return nil, err
	}
	if _, err := vm.RunString(js); err != nil {
		return nil, fmt.Errorf("test script: %s", jsErr(err))
	}
	b, _ := json.Marshal(vm.Get("__results").Export())
	var as []store.Assert
	_ = json.Unmarshal(b, &as)
	return as, nil
}

var callRe = regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*\.\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)
var rawCallRe = regexp.MustCompile(`\bcall\(\s*["'\x60]([a-zA-Z0-9_]+)["'\x60]\s*,\s*["'\x60]([A-Za-z]+)\s+([^"'\x60]+)["'\x60]`)

// UsedCalls finds the API requests a tool's code calls, as "api.request" or
// "api METHOD /path", and the APIs involved.
func UsedCalls(src string, clients []Client) (calls []string, apis []string) {
	known := map[string]map[string]bool{}
	for _, c := range clients {
		m := map[string]bool{}
		for _, r := range c.Requests {
			m[r.Name] = true
		}
		known[c.API.Name] = m
	}
	seen := map[string]bool{}
	apiSeen := map[string]bool{}
	for _, m := range callRe.FindAllStringSubmatch(src, -1) {
		if reqs, ok := known[m[1]]; ok && reqs[m[2]] {
			k := m[1] + "." + m[2]
			if !seen[k] {
				seen[k] = true
				calls = append(calls, k)
			}
			apiSeen[m[1]] = true
		}
	}
	for _, m := range rawCallRe.FindAllStringSubmatch(src, -1) {
		if _, ok := known[m[1]]; ok {
			k := m[1] + " " + strings.ToUpper(m[2]) + " " + m[3]
			if !seen[k] {
				seen[k] = true
				calls = append(calls, k)
			}
			apiSeen[m[1]] = true
		}
	}
	for a := range apiSeen {
		apis = append(apis, a)
	}
	sort.Strings(calls)
	sort.Strings(apis)
	return calls, apis
}
