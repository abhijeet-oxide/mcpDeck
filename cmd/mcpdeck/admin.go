package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/abhijeet-oxide/mcpdeck/internal/importer"
	"github.com/abhijeet-oxide/mcpdeck/internal/version"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

// adminInstructions is all an agent needs to read before using the admin
// server; keep it short (AGENT-1). Measured by `make agent-context`.
const adminInstructions = `mcpDeck turns HTTP APIs into small MCP tools. This server sets mcpDeck up, with the same reach as the mcpdeck CLI.
Order: import_api, get_api, set_variable or ask_for_secret, set_auth, send_request (check it works), create_toolkit, generate_tool or save_tool, lint, connect_client.
Secrets: never ask the user to paste one into chat. Use secret_from_env or ask_for_secret.
A person approves write tools and widens access; pending_reviews lists what waits for them.`

type adminTool struct {
	name, desc string
	schema     string
	readOnly   bool
	run        func(cl *client, a map[string]any) (any, error)
}

func str(a map[string]any, k string) string { s, _ := a[k].(string); return s }

func adminTools() []adminTool {
	apiOf := func(cl *client, a map[string]any) (*apiDetailCLI, error) { return findAPI(cl, str(a, "api")) }
	return []adminTool{
		{"overview", "APIs, toolkits and agents set up so far, and what to do next.", `{}`, true,
			func(cl *client, _ map[string]any) (any, error) {
				var apis, kits, agents []map[string]any
				_ = cl.do("GET", "/api/apis", nil, &apis)
				_ = cl.do("GET", "/api/toolkits", nil, &kits)
				_ = cl.do("GET", "/api/agents", nil, &agents)
				names := func(xs []map[string]any) []any {
					out := []any{}
					for _, x := range xs {
						out = append(out, x["name"])
					}
					return out
				}
				next := "import_api"
				switch {
				case len(apis) > 0 && len(kits) == 0:
					next = "create_toolkit"
				case len(kits) > 0:
					next = "generate_tool, then connect_client"
				}
				return map[string]any{"apis": names(apis), "toolkits": names(kits), "agents": names(agents), "next": next}, nil
			}},
		{"import_api", "Import an OpenAPI or Swagger URL or text, a Postman collection, or curl commands into an API (created if new). Adds and updates requests, never deletes.",
			`{"type":"object","properties":{"source":{"type":"string","description":"URL, or the document or curl commands as text"},"api":{"type":"string","description":"API name tools will call it by, e.g. crm"},"ca_pem":{"type":"string","description":"company CA certificate (PEM) to trust"},"insecure":{"type":"boolean"}},"required":["source","api"]}`, false,
			func(cl *client, a map[string]any) (any, error) {
				body := map[string]any{"name": str(a, "api")}
				src := strings.TrimSpace(str(a, "source"))
				if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
					body["url"] = src
				} else {
					body["text"] = src
				}
				if pem := str(a, "ca_pem"); pem != "" {
					body["caCerts"] = pem
				}
				if b, _ := a["insecure"].(bool); b {
					body["insecure"] = true
				}
				if d, err := findAPI(cl, str(a, "api")); err == nil {
					body["apiId"] = d.ID
				}
				var rep map[string]any
				return rep, cl.do("POST", "/api/import", body, &rep)
			}},
		{"get_api", "One API: base URL, auth type, variables (secret values never shown) and requests with method, path and access.",
			`{"type":"object","properties":{"api":{"type":"string"}},"required":["api"]}`, true,
			func(cl *client, a map[string]any) (any, error) {
				d, err := apiOf(cl, a)
				if err != nil {
					return nil, err
				}
				var raw map[string]any
				if err := cl.do("GET", "/api/apis/"+d.ID, nil, &raw); err != nil {
					return nil, err
				}
				reqs := []string{}
				for _, rr := range d.Requests {
					var m map[string]any
					_ = json.Unmarshal(rr, &m)
					reqs = append(reqs, fmt.Sprintf("%v %v %v · %v", m["name"], m["method"], strings.TrimPrefix(fmt.Sprint(m["url"]), "{{baseUrl}}"), m["access"]))
				}
				vars := []string{}
				for _, v := range d.Variables {
					vars = append(vars, v.Name+" ("+v.Source+")")
				}
				return map[string]any{"name": d.Name, "auth": raw["auth"], "variables": vars, "requests": reqs}, nil
			}},
		{"set_variable", "Set a plain variable such as baseUrl or a user name. Not for secrets.",
			`{"type":"object","properties":{"api":{"type":"string"},"name":{"type":"string"},"value":{"type":"string"}},"required":["api","name","value"]}`, false,
			func(cl *client, a map[string]any) (any, error) {
				if importer.IsSecretName(str(a, "name")) {
					return nil, errors.New(str(a, "name") + " looks like a secret; use secret_from_env or ask_for_secret so its value stays out of the chat")
				}
				d, err := apiOf(cl, a)
				if err != nil {
					return nil, err
				}
				return "set", setVar(cl, d, str(a, "name"), str(a, "value"), "literal")
			}},
		{"secret_from_env", "Store a secret from an environment variable of this server's process. The value never enters the chat.",
			`{"type":"object","properties":{"api":{"type":"string"},"name":{"type":"string","description":"variable name, e.g. password"},"env":{"type":"string","description":"environment variable holding it"}},"required":["api","name","env"]}`, false,
			func(cl *client, a map[string]any) (any, error) {
				v := os.Getenv(str(a, "env"))
				if v == "" {
					return nil, fmt.Errorf("%s isn't set in mcpDeck's environment; use ask_for_secret instead", str(a, "env"))
				}
				d, err := apiOf(cl, a)
				if err != nil {
					return nil, err
				}
				return "stored encrypted as " + str(a, "name"), setVar(cl, d, str(a, "name"), v, "secret")
			}},
		{"ask_for_secret", "Tell the user how to enter a secret themselves. Returns the command and where in the UI.",
			`{"type":"object","properties":{"api":{"type":"string"},"name":{"type":"string"}},"required":["api","name"]}`, true,
			func(_ *client, a map[string]any) (any, error) {
				return map[string]any{"say_to_user": fmt.Sprintf("Please store %s for %s: run `mcpdeck secret set %s %s` and paste it when asked, or open mcpDeck › %s › Environments.",
					str(a, "name"), str(a, "api"), str(a, "api"), str(a, "name"), str(a, "api"))}, nil
			}},
		{"update_request", "Change a request: its body template ({{variables}} allowed) or whether it reads or writes.",
			`{"type":"object","properties":{"api":{"type":"string"},"request":{"type":"string"},"body":{"type":"string"},"access":{"type":"string","enum":["read","write"]},"reason":{"type":"string"}},"required":["api","request"]}`, false,
			func(cl *client, a map[string]any) (any, error) {
				d, err := apiOf(cl, a)
				if err != nil {
					return nil, err
				}
				m, err := d.request(str(a, "request"))
				if err != nil {
					return nil, err
				}
				if b, ok := a["body"].(string); ok {
					m["body"] = b
					if json.Valid([]byte(b)) {
						m["bodyType"] = "json"
					}
					if err := cl.do("PUT", "/api/requests/"+fmt.Sprint(m["id"]), m, nil); err != nil {
						return nil, err
					}
				}
				if acc := str(a, "access"); acc != "" {
					if err := cl.do("POST", "/api/requests/"+fmt.Sprint(m["id"])+"/access", map[string]any{"access": acc, "reason": str(a, "reason")}, nil); err != nil {
						return nil, err
					}
				}
				return "updated", nil
			}},
		{"set_auth", "Set how mcpDeck signs in. auth.type: bearer {token}, basic {username,password}, apikey {key,in,value}, login {steps:[{requestId,captures:[{name,from}]}], apply:[{key,value}], refreshRequestId, expiresFrom}, oauth2 {grant,tokenUrl,clientId,clientSecret,scope}, hmac {secret,stringToSign,apply}. Values are {{variables}}; captures are json:path, header:Name, cookie:name or regex:...; use request names for requestId.",
			`{"type":"object","properties":{"api":{"type":"string"},"auth":{"type":"object"}},"required":["api","auth"]}`, false,
			func(cl *client, a map[string]any) (any, error) {
				d, err := apiOf(cl, a)
				if err != nil {
					return nil, err
				}
				auth, _ := a["auth"].(map[string]any)
				ids := func(v any) any {
					if s, ok := v.(string); ok && s != "" {
						if m, err := d.request(s); err == nil {
							return m["id"]
						}
					}
					return v
				}
				for _, k := range []string{"loginRequestId", "refreshRequestId"} {
					if v, ok := auth[k]; ok {
						auth[k] = ids(v)
					}
				}
				if steps, ok := auth["steps"].([]any); ok {
					for _, st := range steps {
						if m, ok := st.(map[string]any); ok {
							m["requestId"] = ids(m["requestId"])
						}
					}
				}
				var out map[string]any
				if err := cl.do("PATCH", "/api/apis/"+d.ID, map[string]any{"auth": auth}, &out); err != nil {
					return nil, err
				}
				return "auth set; check it with send_request", nil
			}},
		{"send_request", "Send one saved request now and get status and a short body, to check auth and variables.",
			`{"type":"object","properties":{"api":{"type":"string"},"request":{"type":"string"}},"required":["api","request"]}`, false,
			func(cl *client, a map[string]any) (any, error) {
				d, err := apiOf(cl, a)
				if err != nil {
					return nil, err
				}
				m, err := d.request(str(a, "request"))
				if err != nil {
					return nil, err
				}
				var out map[string]any
				if err := cl.do("POST", "/api/send", map[string]any{"apiId": d.ID, "requestId": m["id"]}, &out); err != nil {
					return nil, err
				}
				body := fmt.Sprint(out["body"])
				if len(body) > 1500 {
					body = body[:1500] + "… (trimmed)"
				}
				return map[string]any{"status": out["status"], "error": out["error"], "body": body}, nil
			}},
		{"create_toolkit", "Create a toolkit: one MCP server whose tools may call these APIs.",
			`{"type":"object","properties":{"name":{"type":"string"},"apis":{"type":"array","items":{"type":"string"}},"description":{"type":"string"}},"required":["name","apis"]}`, false,
			func(cl *client, a map[string]any) (any, error) {
				in := map[string]any{"name": str(a, "name"), "buildScope": a["apis"]}
				if d := str(a, "description"); d != "" {
					in["description"] = d
				}
				var out map[string]any
				if err := cl.do("POST", "/api/toolkits", in, &out); err != nil {
					return nil, err
				}
				return map[string]any{"created": out["name"], "mcp_url": cl.base + "/mcp/" + fmt.Sprint(out["name"])}, nil
			}},
		{"list_toolkits", "Toolkits with their tool and agent counts.", `{}`, true,
			func(cl *client, _ map[string]any) (any, error) {
				var kits []map[string]any
				return kits, cl.do("GET", "/api/toolkits", nil, &kits)
			}},
		{"generate_tool", "Make and save a starter tool from one request (api.request). Write requests need allow_write.",
			`{"type":"object","properties":{"toolkit":{"type":"string"},"from":{"type":"string","description":"api.request"},"name":{"type":"string"},"description":{"type":"string"},"allow_write":{"type":"boolean"}},"required":["toolkit","from"]}`, false,
			func(cl *client, a map[string]any) (any, error) {
				var g struct {
					Source, Access, Name string
					Tests                json.RawMessage
				}
				if err := cl.do("POST", "/api/tools/generate", map[string]any{"from": str(a, "from"), "name": str(a, "name"), "description": str(a, "description")}, &g); err != nil {
					return nil, err
				}
				if w, _ := a["allow_write"].(bool); g.Access == "write" && !w {
					return nil, errors.New(str(a, "from") + " changes data; pass allow_write to make a write tool (a person must approve it)")
				}
				var out map[string]any
				if err := cl.do("POST", "/api/toolkits/"+str(a, "toolkit")+"/save", map[string]any{"source": g.Source, "tests": g.Tests}, &out); err != nil {
					return nil, err
				}
				return map[string]any{"name": g.Name, "status": out["status"], "message": out["message"], "source": g.Source, "lint": compactLint(out["lint"])}, nil
			}},
		{"save_tool", "Save TypeScript tool source as a new version, with optional tests. Review rules apply; lint runs.",
			`{"type":"object","properties":{"toolkit":{"type":"string"},"source":{"type":"string"},"tests":{"type":"array","items":{"type":"object"}}},"required":["toolkit","source"]}`, false,
			func(cl *client, a map[string]any) (any, error) {
				tests := a["tests"]
				if tests == nil {
					tests = []any{}
				}
				var out map[string]any
				if err := cl.do("POST", "/api/toolkits/"+str(a, "toolkit")+"/save", map[string]any{"source": str(a, "source"), "tests": tests}, &out); err != nil {
					return nil, err
				}
				return map[string]any{"status": out["status"], "message": out["message"], "warnings": out["warnings"], "lint": compactLint(out["lint"])}, nil
			}},
		{"test_tool", "Run unsaved tool source against the live APIs and get output, calls and size.",
			`{"type":"object","properties":{"toolkit":{"type":"string"},"source":{"type":"string"},"input":{"type":"object"}},"required":["toolkit","source"]}`, false,
			func(cl *client, a map[string]any) (any, error) {
				var out map[string]any
				if err := cl.do("POST", "/api/toolkits/"+str(a, "toolkit")+"/test", map[string]any{"source": str(a, "source"), "input": a["input"]}, &out); err != nil {
					return nil, err
				}
				return map[string]any{"output": out["output"], "error": out["error"], "outputTokens": out["outputTokens"], "calls": len(asSlice(out["calls"]))}, nil
			}},
		{"lint", "Check a toolkit, and optionally unsaved source, against MCP tool best practices. Each finding has a severity and a fix.",
			`{"type":"object","properties":{"toolkit":{"type":"string"},"source":{"type":"string"}},"required":["toolkit"]}`, true,
			func(cl *client, a map[string]any) (any, error) {
				var out map[string]any
				return out, cl.do("POST", "/api/toolkits/"+str(a, "toolkit")+"/lint", map[string]any{"draft": str(a, "source")}, &out)
			}},
		{"list_tools", "A toolkit's tools with access, status and version.",
			`{"type":"object","properties":{"toolkit":{"type":"string"}},"required":["toolkit"]}`, true,
			func(cl *client, a map[string]any) (any, error) {
				var tk struct {
					ToolRows []struct {
						Name   string         `json:"name"`
						Latest map[string]any `json:"latest"`
					} `json:"toolRows"`
				}
				if err := cl.do("GET", "/api/toolkits/"+str(a, "toolkit"), nil, &tk); err != nil {
					return nil, err
				}
				out := []map[string]any{}
				for _, t := range tk.ToolRows {
					out = append(out, map[string]any{"name": t.Name, "access": t.Latest["access"], "status": t.Latest["status"], "version": t.Latest["n"]})
				}
				return out, nil
			}},
		{"pending_reviews", "Tool versions and tool requests waiting for a person, so you can tell the user.",
			`{"type":"object","properties":{"toolkit":{"type":"string"}},"required":["toolkit"]}`, true,
			func(cl *client, a map[string]any) (any, error) {
				var tk map[string]any
				if err := cl.do("GET", "/api/toolkits/"+str(a, "toolkit"), nil, &tk); err != nil {
					return nil, err
				}
				pend := []any{}
				for _, v := range asSlice(tk["pendingList"]) {
					if m, ok := v.(map[string]any); ok {
						pend = append(pend, fmt.Sprintf("%v v%v (%v)", m["name"], m["n"], m["access"]))
					}
				}
				reqs := []any{}
				for _, v := range asSlice(tk["toolRequests"]) {
					if m, ok := v.(map[string]any); ok && m["status"] == "open" {
						reqs = append(reqs, m["description"])
					}
				}
				return map[string]any{"pending_versions": pend, "open_tool_requests": reqs, "how": "A person approves in mcpDeck › " + str(a, "toolkit") + " › Review, or with mcpdeck tool approve."}, nil
			}},
		{"connect_client", "Give an MCP client read access to a toolkit. write:true writes its config file; otherwise returns the URL and token to configure it.",
			`{"type":"object","properties":{"toolkit":{"type":"string"},"client":{"type":"string","enum":["claude-code","vscode","cursor","claude-desktop","other"]},"name":{"type":"string"},"write":{"type":"boolean"}},"required":["toolkit","client"]}`, false,
			func(cl *client, a map[string]any) (any, error) {
				var out struct {
					Result struct {
						Token string `json:"token"`
					} `json:"result"`
					PlanID string `json:"planId"`
				}
				if err := cl.do("POST", "/api/connect", map[string]any{"client": str(a, "client"), "toolkitId": str(a, "toolkit"), "access": "read", "name": str(a, "name"), "skills": true}, &out); err != nil {
					return nil, err
				}
				if w, _ := a["write"].(bool); w && str(a, "client") != "other" {
					var written struct{ Written []string }
					if err := cl.do("POST", "/api/plans/"+out.PlanID+"/apply", nil, &written); err != nil {
						return nil, err
					}
					return map[string]any{"written": written.Written}, nil
				}
				return map[string]any{"url": cl.base + "/mcp/" + str(a, "toolkit"), "token": out.Result.Token, "header": "Authorization: Bearer <token>"}, nil
			}},
	}
}

func asSlice(v any) []any { s, _ := v.([]any); return s }

// mcpCmd runs the admin MCP server on stdio (PAR-1): an agent sets mcpDeck
// up with the same reach as the CLI, recorded as that agent, and with the
// decisions only a person makes held back.
func mcpCmd(data *string) *cobra.Command {
	var name string
	c := &cobra.Command{Use: "mcp", Short: "Admin MCP server on stdio: an agent imports APIs, sets auth, builds and tests tools (write tools still need a person)",
		Example: `  claude mcp add mcpdeck-admin -- mcpdeck mcp --agent "Claude Code"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			cl.agent = name
			s := newAdminServer(cl)
			return s.Run(cmd.Context(), &sdk.StdioTransport{})
		}}
	c.Flags().StringVar(&name, "agent", "Admin agent", "name the actions are recorded under")
	return c
}

func newAdminServer(cl *client) *sdk.Server {
	s := sdk.NewServer(&sdk.Implementation{Name: "mcpdeck-admin", Title: "mcpDeck admin", Version: version.Version}, &sdk.ServerOptions{Instructions: adminInstructions})
	for _, t := range adminTools() {
		t := t
		var sch map[string]any
		_ = json.Unmarshal([]byte(t.schema), &sch)
		if sch["type"] == nil {
			sch = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		destructive := false
		s.AddTool(&sdk.Tool{Name: t.name, Description: t.desc, InputSchema: sch,
			Annotations: &sdk.ToolAnnotations{ReadOnlyHint: t.readOnly, DestructiveHint: &destructive, OpenWorldHint: boolp(false)}},
			func(_ context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
				args := map[string]any{}
				if len(req.Params.Arguments) > 0 {
					_ = json.Unmarshal(req.Params.Arguments, &args)
				}
				out, err := t.run(cl, args)
				if err != nil {
					return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: err.Error()}}}, nil
				}
				text, ok := out.(string)
				if !ok {
					b, _ := json.Marshal(out)
					text = string(b)
				}
				return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: text}}}, nil
			})
	}
	return s
}

func boolp(b bool) *bool { return &b }

// compactLint keeps what an agent needs to act on a finding.
func compactLint(v any) []map[string]any {
	out := []map[string]any{}
	for _, f := range asSlice(v) {
		m, _ := f.(map[string]any)
		if m == nil {
			continue
		}
		out = append(out, map[string]any{"rule": m["rule"], "level": m["level"], "title": m["title"], "fix": m["fix"]})
	}
	return out
}
