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
	"github.com/abhijeet-oxide/mcpdeck/internal/lint"
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
	DefaultMaxOutput = 4 << 20 // a safety cap; agents get outputs over deck.OutputBudget cut to fit
	memoryCap        = 256 << 20
	maxCallDepth     = 2000
	// MaxTimeout caps tool.timeout (TOOL-6).
	MaxTimeout = 300 * time.Second
	// ConfirmTimeout is how long confirm() waits for the person (CONF-1).
	ConfirmTimeout = 5 * time.Minute
)

// Definition is the `tool` export (TOOL-1).
type Definition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	UseWhen     string          `json:"useWhen"`
	DontUseWhen string          `json:"dontUseWhen"`
	InputSchema json.RawMessage `json:"inputSchema"`
	// Timeout is the run limit in seconds for long jobs, up to MaxTimeout.
	Timeout float64 `json:"timeout,omitempty"`
	// Confirm makes each run's writes wait for the person (CONF-1).
	Confirm bool `json:"confirm,omitempty"`
	// Annotations are optional MCP hints. readOnlyHint and destructiveHint
	// are set from the requests the tool calls; a file may only narrow them.
	Annotations *lint.Annotations `json:"annotations,omitempty"`
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
	if d.Timeout < 0 || time.Duration(d.Timeout*float64(time.Second)) > MaxTimeout {
		return d, fmt.Errorf("tool.timeout is %v seconds; use 1 to %d", d.Timeout, int(MaxTimeout.Seconds()))
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
	// Files lists an application's files folder (HTTP-3); nil turns
	// files.list off.
	Files FileLister
	// Confirm asks the person to confirm a write (CONF-1). Nil means the
	// person is running the tool themselves, which confirms it.
	Confirm func(ctx context.Context, title string, fields any) error
}

// FileLister lists an application's files folder.
type FileLister interface {
	ListFiles(ctx context.Context, apiID string) ([]proxy.FileInfo, error)
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	limit := env.Timeout
	timer := time.AfterFunc(limit, cancel)
	defer timer.Stop()
	var limitMu sync.Mutex
	getLimit := func() time.Duration { limitMu.Lock(); defer limitMu.Unlock(); return limit }
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

	confirmed := false
	startCall := func(c proxy.Call, label string, raw bool) goja.Value {
		c.Confirmed = c.Confirmed || confirmed
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
				ct := ""
				headers := map[string]any{}
				for k, vs := range r.Headers {
					headers[strings.ToLower(k)] = strings.Join(vs, ", ")
					if strings.EqualFold(k, "Content-Type") && len(vs) > 0 {
						ct = vs[0]
					}
				}
				reason := ""
				if status >= 400 && !r.Binary {
					reason = proxy.UpstreamMessage(r.Body, ct)
				}
				fn = func() {
					parsed, perr := jsonParse(goja.Undefined(), vm.ToValue(body))
					if perr != nil {
						parsed = vm.ToValue(body)
					}
					if raw {
						// .raw(): status, headers and body, no throw on 4xx/5xx.
						o := vm.NewObject()
						_ = o.Set("status", status)
						_ = o.Set("ok", status < 400)
						_ = o.Set("headers", headers)
						_ = o.Set("body", parsed)
						_ = resolve(o)
						return
					}
					if status >= 400 {
						msg := fmt.Sprintf("%s returned %d", label, status)
						if reason != "" {
							msg += ": " + reason
						}
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
		// headers is always headers, also next to flat params (FLOW-1).
		if h, ok := m["headers"].(map[string]any); ok {
			a.Headers = map[string]string{}
			for k, v := range h {
				a.Headers[k] = fmt.Sprint(v)
			}
		}
		if !structured {
			flat := map[string]any{}
			obj := v.ToObject(vm)
			for k, x := range m {
				if _, isHdr := x.(map[string]any); k == "headers" && isHdr {
					continue
				}
				// { limit: input.limit } with no limit given leaves it out,
				// like spreading the input would.
				if goja.IsUndefined(obj.Get(k)) {
					continue
				}
				flat[k] = x
			}
			a.Flat = flat
			return a
		}
		if p, ok := m["path"].(map[string]any); ok {
			a.Path = p
		}
		if q, ok := m["query"].(map[string]any); ok {
			a.Query = q
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
			mk := func(raw bool) func(goja.FunctionCall) goja.Value {
				return func(c goja.FunctionCall) goja.Value {
					call := env.Base
					call.APIID, call.RequestID, call.Args = cl.API.ID, r.ID, argsOf(c.Argument(0))
					call.EnvID = env.EnvByAPI[cl.API.ID]
					return startCall(call, cl.API.Name+"."+r.Name, raw)
				}
			}
			fv := vm.ToValue(mk(false)).ToObject(vm)
			_ = fv.Set("raw", mk(true))
			_ = obj.Set(r.Name, fv)
		}
		_ = vm.Set(cl.API.Name, obj)
		_, _ = vm.RunString("Object.freeze(" + cl.API.Name + ")")
	}
	rawCall := func(raw bool) func(goja.FunctionCall) goja.Value {
		return func(c goja.FunctionCall) goja.Value {
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
			return startCall(call, apiName+" "+strings.ToUpper(method)+" "+path, raw)
		}
	}
	callFn := vm.ToValue(rawCall(false)).ToObject(vm)
	_ = callFn.Set("raw", rawCall(true))
	_ = vm.Set("call", callFn)
	// sleep(ms) waits between polls; it counts against the timeout.
	_ = vm.Set("sleep", func(c goja.FunctionCall) goja.Value {
		ms := c.Argument(0).ToInteger()
		if ms < 0 {
			ms = 0
		}
		p, resolve, _ := vm.NewPromise()
		pending++
		go func() {
			select {
			case <-time.After(time.Duration(ms) * time.Millisecond):
			case <-ctx.Done():
			}
			settled <- settle{fn: func() { _ = resolve(goja.Undefined()) }}
		}()
		return vm.ToValue(p)
	})
	// html.forms / tables / text read server-rendered pages (TOOL-12).
	htmlObj := vm.NewObject()
	_ = htmlObj.Set("forms", func(c goja.FunctionCall) goja.Value {
		b, _ := json.Marshal(ParseForms(c.Argument(0).String()))
		v, _ := jsonParse(goja.Undefined(), vm.ToValue(string(b)))
		return v
	})
	_ = htmlObj.Set("tables", func(c goja.FunctionCall) goja.Value {
		b, _ := json.Marshal(ParseTables(c.Argument(0).String()))
		v, _ := jsonParse(goja.Undefined(), vm.ToValue(string(b)))
		return v
	})
	_ = htmlObj.Set("text", func(c goja.FunctionCall) goja.Value { return vm.ToValue(PageText(c.Argument(0).String())) })
	_ = vm.Set("html", htmlObj)
	_, _ = vm.RunString("Object.freeze(html)")

	// files.list(api) and files.attach(name): the application's files
	// folder, names only (HTTP-3).
	filesObj := vm.NewObject()
	_ = filesObj.Set("list", func(c goja.FunctionCall) goja.Value {
		p, resolve, reject := vm.NewPromise()
		apiName := c.Argument(0).String()
		a, ok := byName[apiName]
		if !ok || env.Files == nil {
			e, _ := vm.New(apiErrCtor, vm.ToValue(fmt.Sprintf("files.list(%q): no such API here, or files are off", apiName)), vm.ToValue(0), goja.Undefined(), vm.ToValue("bad_call"))
			_ = reject(e)
			return vm.ToValue(p)
		}
		pending++
		go func() {
			list, err := env.Files.ListFiles(ctx, a.ID)
			settled <- settle{fn: func() {
				if err != nil {
					code := "error"
					var pe *proxy.Error
					if errors.As(err, &pe) {
						code = pe.Code
					}
					e, _ := vm.New(apiErrCtor, vm.ToValue(err.Error()), vm.ToValue(0), goja.Undefined(), vm.ToValue(code))
					_ = reject(e)
					return
				}
				b, _ := json.Marshal(list)
				v, _ := jsonParse(goja.Undefined(), vm.ToValue(string(b)))
				_ = resolve(v)
			}}
		}()
		return vm.ToValue(p)
	})
	_ = filesObj.Set("attach", func(c goja.FunctionCall) goja.Value {
		o := vm.NewObject()
		_ = o.Set("$file", c.Argument(0).String())
		return o
	})
	_ = vm.Set("files", filesObj)
	_, _ = vm.RunString("Object.freeze(files)")

	// confirm({ title, fields }) waits for the person before a write (CONF-1).
	_ = vm.Set("confirm", func(c goja.FunctionCall) goja.Value {
		p, resolve, reject := vm.NewPromise()
		arg := c.Argument(0)
		title, fields := "Confirm", any(nil)
		if o, ok := arg.Export().(map[string]any); ok {
			if t, ok := o["title"].(string); ok && t != "" {
				title = t
			}
			fields = o["fields"]
		}
		if env.Confirm == nil {
			confirmed = true
			_ = resolve(true)
			return vm.ToValue(p)
		}
		pending++
		// Waiting for a person doesn't count against the run's timeout.
		limitMu.Lock()
		limit += ConfirmTimeout
		limitMu.Unlock()
		timer.Reset(getLimit() - time.Since(start))
		go func() {
			err := env.Confirm(ctx, title, fields)
			settled <- settle{fn: func() {
				if err != nil {
					e, _ := vm.New(apiErrCtor, vm.ToValue(err.Error()), vm.ToValue(0), goja.Undefined(), vm.ToValue("not_confirmed"))
					_ = reject(e)
					return
				}
				confirmed = true
				_ = resolve(true)
			}}
		}()
		return vm.ToValue(p)
	})

	// xml(text) parses XML or SOAP into plain objects (see ParseXML).
	_ = vm.Set("xml", func(c goja.FunctionCall) goja.Value {
		v, err := ParseXML(c.Argument(0).String())
		if err != nil {
			panic(vm.NewTypeError("xml(): " + err.Error()))
		}
		return vm.ToValue(v)
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
				vm.Interrupt(fmt.Sprintf("timed out after %s (limit %s; raise it with timeout in the tool export)", time.Since(start).Round(time.Millisecond), getLimit()))
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
	exportsObj := vm.Get("module").ToObject(vm).Get("exports").ToObject(vm)
	if tv := exportsObj.Get("tool"); tv != nil && !goja.IsUndefined(tv) && !goja.IsNull(tv) {
		if tov := tv.ToObject(vm).Get("timeout"); tov != nil && !goja.IsUndefined(tov) {
			if sec := tov.ToFloat(); sec > 0 {
				d := min(time.Duration(sec*float64(time.Second)), MaxTimeout)
				limitMu.Lock()
				limit = d
				limitMu.Unlock()
				timer.Reset(d - time.Since(start))
			}
		}
		// confirm: true makes the tool's writes wait for confirm() (CONF-1).
		if cv := tv.ToObject(vm).Get("confirm"); cv != nil && cv.ToBoolean() && env.Confirm != nil {
			env.Base.NeedConfirm = true
		}
	}
	fnv := exportsObj.Get("default")
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
				res.Error = fmt.Sprintf("timed out after %s; make fewer calls, run them with Promise.all, or raise timeout in the tool export (up to %d seconds)", getLimit(), int(MaxTimeout.Seconds()))
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

var callRe = regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*\.\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:\.\s*raw\s*)?\(`)
var rawCallRe = regexp.MustCompile(`\bcall(?:\s*\.\s*raw)?\(\s*["'\x60]([a-zA-Z0-9_]+)["'\x60]\s*,\s*["'\x60]([A-Za-z]+)\s+([^"'\x60]+)["'\x60]`)

// UsedCalls finds the API requests a tool's code calls, as "api.request" or
// "api METHOD /path", and the APIs involved.
func UsedCalls(src string, clients []Client) (calls []string, apis []string) {
	known := map[string]map[string]bool{}
	reqsOf := map[string][]store.Request{}
	for _, c := range clients {
		reqsOf[c.API.Name] = c.Requests
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
			// A raw call on a saved request counts as that request (TOOL-9).
			pathOnly, _, _ := strings.Cut(m[3], "?")
			if r, _ := proxy.MatchRequest(reqsOf[m[1]], m[2], pathOnly); r != nil {
				k = m[1] + "." + r.Name
			}
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
