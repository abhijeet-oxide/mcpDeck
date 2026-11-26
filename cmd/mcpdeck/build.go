package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

// The builder commands: everything the UI does to set up an API and a
// toolkit, for people in a terminal and for agents (PAR-1). Each takes
// --json for machine-readable output.

type apiVar struct {
	ID     string `json:"id"`
	EnvID  string `json:"envId"`
	Name   string `json:"name"`
	Source string `json:"source"`
}

type apiDetailCLI struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	ActiveEnvID string            `json:"activeEnvId"`
	Variables   []apiVar          `json:"variables"`
	Requests    []json.RawMessage `json:"requests"`
}

func findAPI(cl *client, name string) (*apiDetailCLI, error) {
	var apis []struct{ ID, Name string }
	if err := cl.do("GET", "/api/apis", nil, &apis); err != nil {
		return nil, err
	}
	var names []string
	for _, a := range apis {
		if a.Name == name {
			var d apiDetailCLI
			if err := cl.do("GET", "/api/apis/"+a.ID, nil, &d); err != nil {
				return nil, err
			}
			return &d, nil
		}
		names = append(names, a.Name)
	}
	return nil, fmt.Errorf("no API named %q; APIs: %s", name, strings.Join(names, ", "))
}

func (d *apiDetailCLI) request(name string) (map[string]any, error) {
	var names []string
	for _, raw := range d.Requests {
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		if m["name"] == name {
			return m, nil
		}
		names = append(names, fmt.Sprint(m["name"]))
	}
	return nil, fmt.Errorf("%s has no request %q; requests: %s", d.Name, name, strings.Join(names, ", "))
}

func setVar(cl *client, a *apiDetailCLI, name, value, source string) error {
	in := map[string]any{"envId": a.ActiveEnvID, "name": name, "value": value, "source": source}
	for _, v := range a.Variables {
		if v.Name == name && v.EnvID == a.ActiveEnvID {
			in["id"] = v.ID
		}
	}
	return cl.do("PUT", "/api/apis/"+a.ID+"/variables", in, nil)
}

func varCmd(data *string) *cobra.Command {
	c := &cobra.Command{Use: "var", Short: "Set an API's plain variables (base URL, user names)"}
	c.AddCommand(&cobra.Command{Use: "set <api> <name> <value>", Short: "Set a plain variable in the API's active environment",
		Example: "  mcpdeck var set shop baseUrl https://shop.example.com", Args: cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			a, err := findAPI(cl, args[0])
			if err != nil {
				return err
			}
			if err := setVar(cl, a, args[1], args[2], "literal"); err != nil {
				return err
			}
			fmt.Printf("Set %s on %s.\n", args[1], args[0])
			return nil
		}})
	return c
}

func secretCmd(data *string) *cobra.Command {
	c := &cobra.Command{Use: "secret", Short: "Store an API's secrets encrypted; values are read from stdin, never arguments"}
	c.AddCommand(&cobra.Command{Use: "set <api> <name>", Short: "Store a secret variable (reads the value from stdin)",
		Example: "  printf %s \"$TOKEN\" | mcpdeck secret set github token", Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			a, err := findAPI(cl, args[0])
			if err != nil {
				return err
			}
			var value string
			if f, ok := any(os.Stdin).(*os.File); ok && term.IsTerminal(int(f.Fd())) {
				fmt.Fprintf(os.Stderr, "Value for %s (hidden): ", args[1])
				b, err := term.ReadPassword(int(f.Fd()))
				fmt.Fprintln(os.Stderr)
				if err != nil {
					return err
				}
				value = string(b)
			} else {
				line, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil && !errors.Is(err, io.EOF) {
					return err
				}
				value = strings.TrimRight(line, "\r\n")
			}
			if value == "" {
				return errors.New("the value is empty; pipe it on stdin")
			}
			if err := setVar(cl, a, args[1], value, "secret"); err != nil {
				return err
			}
			fmt.Printf("Stored %s for %s, encrypted. Tools and agents never see it.\n", args[1], args[0])
			return nil
		}})
	return c
}

func requestCmd(data *string) *cobra.Command {
	c := &cobra.Command{Use: "request", Short: "Change an imported request"}
	update := func(args []string, change func(m map[string]any)) error {
		cl, err := localClient(*data)
		if err != nil {
			return err
		}
		a, err := findAPI(cl, args[0])
		if err != nil {
			return err
		}
		m, err := a.request(args[1])
		if err != nil {
			return err
		}
		change(m)
		return cl.do("PUT", "/api/requests/"+fmt.Sprint(m["id"]), m, nil)
	}
	c.AddCommand(&cobra.Command{Use: "set-body <api> <request> <body>", Short: "Set a request's body template ({{variables}} allowed)",
		Example: `  mcpdeck request set-body shop login '{"email": "{{email}}", "password": "{{password}}"}'`, Args: cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			err := update(args, func(m map[string]any) {
				m["body"] = args[2]
				if json.Valid([]byte(args[2])) {
					m["bodyType"] = "json"
				} else if m["bodyType"] == "none" || m["bodyType"] == "" {
					m["bodyType"] = "text"
				}
			})
			if err == nil {
				fmt.Printf("Updated %s.%s.\n", args[0], args[1])
			}
			return err
		}})
	var access, reason string
	ac := &cobra.Command{Use: "access <api> <request> --set read|write", Short: "Mark a request as read or write",
		Example: "  mcpdeck request access jfrog searchAql --set read --reason \"AQL only reads\"", Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			a, err := findAPI(cl, args[0])
			if err != nil {
				return err
			}
			m, err := a.request(args[1])
			if err != nil {
				return err
			}
			if err := cl.do("POST", "/api/requests/"+fmt.Sprint(m["id"])+"/access", map[string]any{"access": access, "reason": reason}, nil); err != nil {
				return err
			}
			fmt.Printf("%s.%s is %s.\n", args[0], args[1], access)
			return nil
		}}
	ac.Flags().StringVar(&access, "set", "", "read or write")
	ac.Flags().StringVar(&reason, "reason", "", "why, for the audit log")
	c.AddCommand(ac)
	var sendJSON bool
	send := &cobra.Command{Use: "send <api> <request>", Short: "Send one saved request now, with the API's auth, to check auth and variables",
		Example: "  mcpdeck request send shop listOrders\n  mcpdeck request send shop listOrders --json", Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			a, err := findAPI(cl, args[0])
			if err != nil {
				return err
			}
			m, err := a.request(args[1])
			if err != nil {
				return err
			}
			var out map[string]any
			if err := cl.do("POST", "/api/send", map[string]any{"apiId": a.ID, "requestId": m["id"]}, &out); err != nil {
				return err
			}
			body := fmt.Sprint(out["body"])
			if len(body) > 4000 {
				body = body[:4000] + "\n… (trimmed; open the request in mcpDeck for the full body)"
			}
			if sendJSON {
				printJSON(map[string]any{"status": out["status"], "error": out["error"], "body": body})
				return nil
			}
			if e, _ := out["error"].(string); e != "" {
				return fmt.Errorf("%s", e)
			}
			fmt.Printf("%v\n%s\n", out["status"], body)
			return nil
		}}
	send.Flags().BoolVar(&sendJSON, "json", false, "print JSON")
	c.AddCommand(send)
	return c
}

func authCmd(data *string) *cobra.Command {
	c := &cobra.Command{Use: "auth", Short: "Set how mcpDeck signs in to an API"}
	var typ, refresh, expires, raw, grant, tokenURL, clientID, clientSecret, scope, token, key, in, value, user, pass, secret, sts, alg, enc string
	var reqs, captures, headers []string
	var asJSON bool
	set := &cobra.Command{Use: "set <api> --type <none|bearer|basic|apikey|login|oauth2|hmac>", Short: "Set an API's auth: tokens, login chains, OAuth 2.0 or HMAC signing",
		Example: `  mcpdeck auth set shop --type login --request login --capture json:access_token
  mcpdeck auth set legacy --type login --request loginPage --capture csrf=cookie:csrftoken --request login --header "X-CSRFToken: {{login.csrf}}"
  mcpdeck auth set billing --type oauth2 --grant client_credentials --token-url https://idp/token --client-id '{{clientId}}' --client-secret '{{clientSecret}}'
  mcpdeck auth set payments --type hmac --secret '{{signingKey}}' --string-to-sign '{{method}}\n{{path}}\n{{timestamp}}' --header 'X-Signature: {{sign.signature}}'`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			a, err := findAPI(cl, args[0])
			if err != nil {
				return err
			}
			auth := map[string]any{}
			if raw != "" {
				if err := json.Unmarshal([]byte(raw), &auth); err != nil {
					return fmt.Errorf("--auth-json: %w", err)
				}
			} else {
				auth["type"] = typ
				var apply []map[string]any
				for _, h := range headers {
					k, v, ok := strings.Cut(h, ":")
					if !ok {
						return fmt.Errorf("--header %q: use \"Name: value\"", h)
					}
					apply = append(apply, map[string]any{"key": strings.TrimSpace(k), "value": strings.TrimSpace(v)})
				}
				if len(apply) > 0 {
					auth["apply"] = apply
				}
				switch typ {
				case "none":
				case "bearer":
					auth["token"] = token
				case "basic":
					auth["username"], auth["password"] = user, pass
				case "apikey":
					auth["key"], auth["in"], auth["value"] = key, in, value
				case "login":
					if len(reqs) == 0 {
						return errors.New("a login needs --request <the request that signs in>")
					}
					// Captures follow the --request they come after; with one
					// request they all belong to it.
					steps, err := loginSteps(a, reqs, captures)
					if err != nil {
						return err
					}
					if len(steps) == 1 && len(captures) == 1 && !strings.Contains(captures[0], "=") && len(apply) == 0 {
						auth["loginRequestId"], auth["capture"] = steps[0]["requestId"], captures[0]
					} else {
						auth["steps"] = steps
					}
					if refresh != "" {
						m, err := a.request(refresh)
						if err != nil {
							return err
						}
						auth["refreshRequestId"] = m["id"]
					}
					if expires != "" {
						auth["expiresFrom"] = expires
					}
				case "oauth2":
					auth["grant"], auth["tokenUrl"], auth["clientId"], auth["clientSecret"], auth["scope"] = grant, tokenURL, clientID, clientSecret, scope
					if grant == "password" {
						auth["username"], auth["password"] = user, pass
					}
				case "hmac":
					auth["secret"], auth["stringToSign"], auth["algorithm"], auth["encoding"] = secret, strings.ReplaceAll(sts, `\n`, "\n"), alg, enc
				default:
					return fmt.Errorf("--type must be none, bearer, basic, apikey, login, oauth2 or hmac")
				}
			}
			var out map[string]any
			if err := cl.do("PATCH", "/api/apis/"+a.ID, map[string]any{"auth": auth}, &out); err != nil {
				return err
			}
			if asJSON {
				printJSON(map[string]any{"api": a.Name, "auth": out["auth"]})
				return nil
			}
			fmt.Printf("%s now uses %s auth. Try a request: mcpdeck send %s <request>\n", a.Name, auth["type"], a.Name)
			return nil
		}}
	f := set.Flags()
	f.StringVar(&typ, "type", "", "none, bearer, basic, apikey, login, oauth2 or hmac")
	f.StringArrayVar(&reqs, "request", nil, "login: a request that signs in, in order (repeat for several steps)")
	f.StringArrayVar(&captures, "capture", nil, "login: what to keep, like json:access_token or csrf=cookie:csrftoken (after its --request)")
	f.StringArrayVar(&headers, "header", nil, `headers to add once signed in, like "Authorization: Bearer {{login.token}}"`)
	f.StringVar(&refresh, "refresh-request", "", "login: request that refreshes with {{login.refresh_token}}")
	f.StringVar(&expires, "expires-from", "", "login: json:expires_in or a number of seconds")
	f.StringVar(&grant, "grant", "client_credentials", "oauth2: client_credentials or password")
	f.StringVar(&tokenURL, "token-url", "", "oauth2: token endpoint")
	f.StringVar(&clientID, "client-id", "{{clientId}}", "oauth2: client ID or {{variable}}")
	f.StringVar(&clientSecret, "client-secret", "{{clientSecret}}", "oauth2: a {{secret variable}}")
	f.StringVar(&scope, "scope", "", "oauth2: scopes, space separated")
	f.StringVar(&token, "token", "{{token}}", "bearer: a {{secret variable}}")
	f.StringVar(&key, "key", "", "apikey: header or query parameter name")
	f.StringVar(&in, "in", "header", "apikey: header or query")
	f.StringVar(&value, "value", "{{apiKey}}", "apikey: a {{secret variable}}")
	f.StringVar(&user, "username", "", "basic or oauth2 password grant: user name or {{variable}}")
	f.StringVar(&pass, "password", "{{password}}", "basic or oauth2 password grant: a {{secret variable}}")
	f.StringVar(&secret, "secret", "{{signingKey}}", "hmac: a {{secret variable}}")
	f.StringVar(&sts, "string-to-sign", "", `hmac: template, like "{{method}}\n{{path}}\n{{timestamp}}"`)
	f.StringVar(&alg, "algorithm", "sha256", "hmac: sha256, sha1 or sha512")
	f.StringVar(&enc, "encoding", "hex", "hmac: hex or base64")
	f.StringVar(&raw, "auth-json", "", "the whole auth object as JSON (overrides the other flags)")
	f.BoolVar(&asJSON, "json", false, "print JSON")
	c.AddCommand(set)
	return c
}

// loginSteps pairs captures with the --request before them, using the
// order the flags were given in os.Args.
func loginSteps(a *apiDetailCLI, reqs, captures []string) ([]map[string]any, error) {
	steps := make([]map[string]any, 0, len(reqs))
	for _, r := range reqs {
		m, err := a.request(r)
		if err != nil {
			return nil, err
		}
		steps = append(steps, map[string]any{"requestId": m["id"], "captures": []map[string]any{}})
	}
	step, ci := -1, 0
	for i := 0; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "--request" || strings.HasPrefix(arg, "--request="):
			step++
		case arg == "--capture" || strings.HasPrefix(arg, "--capture="):
			if ci >= len(captures) {
				continue
			}
			s := step
			if s < 0 {
				s = 0
			}
			if s >= len(steps) {
				s = len(steps) - 1
			}
			name, from, ok := strings.Cut(captures[ci], "=")
			if !ok || strings.Contains(name, ":") {
				from = captures[ci]
				name = from[strings.LastIndexAny(from, ":.")+1:]
			}
			steps[s]["captures"] = append(steps[s]["captures"].([]map[string]any), map[string]any{"name": name, "from": from})
			ci++
		}
	}
	return steps, nil
}

func toolkitCmd(data *string) *cobra.Command {
	c := &cobra.Command{Use: "toolkit", Short: "Create and list toolkits (the MCP servers agents connect to)"}
	var apis, desc string
	var asJSON bool
	create := &cobra.Command{Use: "create <name> --apis a,b", Short: "Create a toolkit whose tools may use these APIs",
		Example: "  mcpdeck toolkit create support --apis crm,billing", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			in := map[string]any{"name": args[0]}
			if desc != "" {
				in["description"] = desc
			}
			if apis != "" {
				in["buildScope"] = strings.Split(apis, ",")
			}
			var out map[string]any
			if err := cl.do("POST", "/api/toolkits", in, &out); err != nil {
				return err
			}
			if asJSON {
				printJSON(out)
				return nil
			}
			fmt.Printf("Created toolkit %s. Next: mcpdeck tool generate %s --from <api.request>\n", args[0], args[0])
			return nil
		}}
	create.Flags().StringVar(&apis, "apis", "", "APIs its tools may call, comma separated")
	create.Flags().StringVar(&desc, "description", "", "what the toolkit is for")
	create.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	var listJSON bool
	list := &cobra.Command{Use: "list", Short: "List toolkits", Example: "  mcpdeck toolkit list --json",
		RunE: func(*cobra.Command, []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			var kits []map[string]any
			if err := cl.do("GET", "/api/toolkits", nil, &kits); err != nil {
				return err
			}
			if listJSON {
				printJSON(kits)
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tTOOLS\tAGENTS")
			for _, k := range kits {
				fmt.Fprintf(w, "%v\t%v\t%v\n", k["name"], k["tools"], k["agents"])
			}
			return w.Flush()
		}}
	list.Flags().BoolVar(&listJSON, "json", false, "print JSON")
	c.AddCommand(create, list)
	return c
}

func toolCmd(data *string) *cobra.Command {
	c := &cobra.Command{Use: "tool", Short: "Generate, save and list tools"}
	var from, name, desc string
	var write, asJSON, dry bool
	gen := &cobra.Command{Use: "generate <toolkit> --from <api.request>", Short: "Make a starter tool from one request and save it (tests and lint run)",
		Example: "  mcpdeck tool generate support --from crm.getAccount --name get_account\n  mcpdeck tool generate support --from shop.cancelOrder --write",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			var g struct {
				Source, Access, Name string
				Tests                json.RawMessage
			}
			if err := cl.do("POST", "/api/tools/generate", map[string]any{"from": from, "name": name, "description": desc}, &g); err != nil {
				return err
			}
			if g.Access == "write" && !write {
				return fmt.Errorf("%s changes data; add --write to make a write tool (agents with read access won't see it)", from)
			}
			if dry {
				fmt.Print(g.Source)
				return nil
			}
			var out struct {
				Status  string         `json:"status"`
				Message string         `json:"message"`
				Version map[string]any `json:"version"`
			}
			if err := cl.do("POST", "/api/toolkits/"+args[0]+"/save", map[string]any{"source": g.Source, "tests": g.Tests}, &out); err != nil {
				return err
			}
			if asJSON {
				printJSON(map[string]any{"name": g.Name, "access": g.Access, "status": out.Status, "message": out.Message, "source": g.Source})
				return nil
			}
			fmt.Printf("Saved %s (%s) to %s: %s\n", g.Name, g.Access, args[0], out.Message)
			return nil
		}}
	gen.Flags().StringVar(&from, "from", "", "the request, as api.request")
	gen.Flags().StringVar(&name, "name", "", "tool name in snake_case (default from the request)")
	gen.Flags().StringVar(&desc, "description", "", "what the tool returns, one sentence")
	gen.Flags().BoolVar(&write, "write", false, "allow a request that changes data")
	gen.Flags().BoolVar(&dry, "print", false, "print the source instead of saving")
	gen.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	_ = gen.MarkFlagRequired("from")

	var file string
	var saveJSON bool
	save := &cobra.Command{Use: "save <toolkit> --file tool.ts", Short: "Save a tool file as a new version (tests and lint run; approval rules apply)",
		Example: "  mcpdeck tool save support --file get_account.ts", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			b, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			tests := json.RawMessage("[]")
			if tb, err := os.ReadFile(strings.TrimSuffix(file, ".ts") + ".tests.json"); err == nil {
				tests = tb
			}
			var out map[string]any
			if err := cl.do("POST", "/api/toolkits/"+args[0]+"/save", map[string]any{"source": string(b), "tests": tests}, &out); err != nil {
				return err
			}
			if saveJSON {
				printJSON(out)
				return nil
			}
			fmt.Println(out["message"])
			printLint(out["lint"])
			return nil
		}}
	save.Flags().StringVar(&file, "file", "", "the .ts file")
	save.Flags().BoolVar(&saveJSON, "json", false, "print JSON")
	_ = save.MarkFlagRequired("file")

	var listJSON bool
	list := &cobra.Command{Use: "list <toolkit>", Short: "List a toolkit's tools", Example: "  mcpdeck tool list support --json", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			var tk struct {
				ToolRows []struct {
					Name   string `json:"name"`
					Latest struct {
						Description string `json:"description"`
						Access      string `json:"access"`
						Status      string `json:"status"`
						N           int    `json:"n"`
					} `json:"latest"`
				} `json:"toolRows"`
			}
			if err := cl.do("GET", "/api/toolkits/"+args[0], nil, &tk); err != nil {
				return err
			}
			rows := []map[string]any{}
			for _, t := range tk.ToolRows {
				rows = append(rows, map[string]any{"name": t.Name, "description": t.Latest.Description, "access": t.Latest.Access, "status": t.Latest.Status, "version": t.Latest.N})
			}
			if listJSON {
				printJSON(rows)
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tACCESS\tSTATUS\tVERSION")
			for _, r := range rows {
				fmt.Fprintf(w, "%v\t%v\t%v\tv%v\n", r["name"], r["access"], r["status"], r["version"])
			}
			return w.Flush()
		}}
	list.Flags().BoolVar(&listJSON, "json", false, "print JSON")
	decide := func(approve bool) *cobra.Command {
		verb := map[bool]string{true: "approve", false: "reject"}[approve]
		return &cobra.Command{Use: verb + " <toolkit> <tool>", Short: map[bool]string{true: "Approve a tool's pending version so agents get it", false: "Reject a tool's pending version"}[approve],
			Example: "  mcpdeck tool " + verb + " support cancel_order", Args: cobra.ExactArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				cl, err := localClient(*data)
				if err != nil {
					return err
				}
				var tk struct {
					PendingList []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
						N    int    `json:"n"`
					} `json:"pendingList"`
				}
				if err := cl.do("GET", "/api/toolkits/"+args[0], nil, &tk); err != nil {
					return err
				}
				for _, v := range tk.PendingList {
					if v.Name == args[1] {
						if err := cl.do("POST", "/api/versions/"+v.ID+"/"+verb, nil, nil); err != nil {
							return err
						}
						fmt.Printf("%s %s v%d.\n", map[bool]string{true: "Approved", false: "Rejected"}[approve], v.Name, v.N)
						return nil
					}
				}
				return fmt.Errorf("%s has no pending version of %s", args[0], args[1])
			}}
	}
	var testFile, testInput string
	var testJSON bool
	test := &cobra.Command{Use: "test <toolkit> --file tool.ts [--input JSON]", Short: "Run a tool file against the live APIs without saving it: output, calls and size",
		Example: "  mcpdeck tool test pets --file find_pets.ts --input '{\"status\":\"sold\"}'", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			b, err := os.ReadFile(testFile)
			if err != nil {
				return err
			}
			var input any = map[string]any{}
			if testInput != "" {
				if err := json.Unmarshal([]byte(testInput), &input); err != nil {
					return fmt.Errorf("--input must be a JSON object: %w", err)
				}
			}
			var out map[string]any
			if err := cl.do("POST", "/api/toolkits/"+args[0]+"/test", map[string]any{"source": string(b), "input": input}, &out); err != nil {
				return err
			}
			res := map[string]any{"output": out["output"], "error": out["error"], "outputTokens": out["outputTokens"], "calls": len(asSlice(out["calls"]))}
			if testJSON {
				printJSON(res)
				return nil
			}
			if e, _ := out["error"].(string); e != "" {
				return fmt.Errorf("%s", e)
			}
			o, _ := json.MarshalIndent(out["output"], "", "  ")
			fmt.Printf("%s\n%s, %v output tokens\n", o, plural(len(asSlice(out["calls"])), "call"), out["outputTokens"])
			return nil
		}}
	test.Flags().StringVar(&testFile, "file", "", "the .ts file")
	test.Flags().StringVar(&testInput, "input", "", "tool input as JSON")
	test.Flags().BoolVar(&testJSON, "json", false, "print JSON")
	_ = test.MarkFlagRequired("file")
	c.AddCommand(gen, save, test, list, decide(true), decide(false))
	return c
}

// helpCmd prints every command; --json gives agents the whole CLI in one
// small document (AGENT-3).
func helpCmd(root *cobra.Command) *cobra.Command {
	var asJSON bool
	c := &cobra.Command{Use: "help [command]", Short: "Help about any command (--json for all commands at once)",
		RunE: func(_ *cobra.Command, args []string) error {
			if !asJSON {
				target, _, err := root.Find(args)
				if err != nil || target == nil {
					target = root
				}
				return target.Help()
			}
			printJSON(describe(root))
			return nil
		}}
	c.Flags().BoolVar(&asJSON, "json", false, "print every command as JSON")
	return c
}

type cmdDoc struct {
	Name    string   `json:"name"`
	Use     string   `json:"use"`
	Short   string   `json:"short"`
	Example string   `json:"example,omitempty"`
	Flags   []string `json:"flags,omitempty"`
}

func describe(root *cobra.Command) []cmdDoc {
	var out []cmdDoc
	var walk func(c *cobra.Command, prefix string)
	walk = func(c *cobra.Command, prefix string) {
		for _, sub := range c.Commands() {
			if sub.Hidden || sub.Name() == "completion" || sub.Name() == "help" {
				continue
			}
			name := strings.TrimSpace(prefix + " " + sub.Name())
			if sub.Runnable() {
				d := cmdDoc{Name: name, Use: "mcpdeck " + strings.TrimSpace(prefix+" "+sub.Use), Short: sub.Short, Example: strings.TrimSpace(sub.Example)}
				sub.LocalFlags().VisitAll(func(f *pflag.Flag) {
					if !f.Hidden {
						d.Flags = append(d.Flags, "--"+f.Name+": "+f.Usage)
					}
				})
				out = append(out, d)
			}
			walk(sub, name)
		}
	}
	walk(root, "")
	return out
}

// printLint prints save-time findings, one per line with its fix.
func printLint(v any) {
	l, _ := v.([]any)
	for _, f := range l {
		m, _ := f.(map[string]any)
		if m == nil {
			continue
		}
		fmt.Printf("%-5v %v: %v\n      fix: %v\n", m["level"], m["rule"], m["title"], m["fix"])
	}
}
