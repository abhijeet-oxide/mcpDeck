// Command mcpdeck runs mcpDeck: a local layer between AI agents and your APIs.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/version"
	"github.com/spf13/cobra"
)

func main() {
	if err := root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

func root() *cobra.Command {
	var o serveOpts
	var data string
	r := &cobra.Command{
		Use:           "mcpdeck",
		Short:         "mcpDeck: turn your APIs into small, safe MCP tools for agents",
		Example:       "  mcpdeck                 start and open the UI\n  mcpdeck dev             start with the demo world in throwaway data",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          func(*cobra.Command, []string) error { return serve(o) },
	}
	r.PersistentFlags().StringVar(&data, "data", "", "data folder (default: the per-user folder)")
	r.Flags().IntVar(&o.port, "port", 0, "port on 127.0.0.1 (default 47800, kept after the first run)")
	r.Flags().BoolVar(&o.noOpen, "no-open", false, "don't open the browser")
	cobra.OnInitialize(func() { o.data = data })

	var so serveOpts
	serveCmd := &cobra.Command{Use: "serve", Short: "Start the server", Example: "  mcpdeck serve --no-open",
		RunE: func(*cobra.Command, []string) error { so.data = data; return serve(so) }}
	serveCmd.Flags().IntVar(&so.port, "port", 0, "port on 127.0.0.1")
	serveCmd.Flags().BoolVar(&so.noOpen, "no-open", false, "don't open the browser")

	var do serveOpts
	devCmd := &cobra.Command{Use: "dev", Short: "Start with the demo world, a dev toolkit and a fixed-token agent, in throwaway data",
		Example: "  mcpdeck dev\n  mcpdeck dev --port 47900 --mock-port 9101",
		RunE: func(*cobra.Command, []string) error {
			do.dev, do.data = true, data
			if do.data == "" {
				tmp, err := os.MkdirTemp("", "mcpdeck-dev-")
				if err != nil {
					return err
				}
				defer os.RemoveAll(tmp)
				do.data = tmp
			}
			return serve(do)
		}}
	devCmd.Flags().IntVar(&do.port, "port", defaultPort, "port on 127.0.0.1")
	devCmd.Flags().IntVar(&do.mockPort, "mock-port", 9001, "first port of the four demo APIs (0 picks free ports)")
	devCmd.Flags().BoolVar(&do.noOpen, "no-open", false, "don't open the browser")
	devCmd.Flags().StringVar(&do.launchFile, "launch-file", "", "write the launch URL to this file")
	devCmd.Flags().StringVar(&do.returnURL, "return-url", "", "where Leave demo goes")
	_ = devCmd.Flags().MarkHidden("launch-file")
	_ = devCmd.Flags().MarkHidden("return-url")

	stopCmd := &cobra.Command{Use: "stop", Short: "Stop the running mcpDeck", Example: "  mcpdeck stop",
		RunE: func(*cobra.Command, []string) error {
			c, err := localClient(data)
			if err != nil {
				return err
			}
			if err := c.do("POST", "/api/shutdown", nil, nil); err != nil {
				return err
			}
			fmt.Println("mcpDeck stopped.")
			return nil
		}}

	versionCmd := &cobra.Command{Use: "version", Short: "Print version and build info", Example: "  mcpdeck version",
		Run: func(*cobra.Command, []string) { fmt.Println(version.String()) }}

	auditCmd := &cobra.Command{Use: "audit", Short: "Audit log"}
	auditCmd.AddCommand(&cobra.Command{Use: "verify", Short: "Check the audit log's hash chain", Example: "  mcpdeck audit verify",
		RunE: func(*cobra.Command, []string) error {
			dir := data
			if dir == "" {
				dir = dataDir()
			}
			ctx := context.Background()
			st, err := store.Open(ctx, dir)
			if err != nil {
				return err
			}
			defer st.Close()
			res, err := audit.New(st, clock.Real{}, clock.ULIDs{Clock: clock.Real{}}).Verify(ctx)
			if err != nil {
				return err
			}
			fmt.Println(res.Message)
			if !res.OK {
				os.Exit(2)
			}
			return nil
		}})

	r.AddCommand(serveCmd, devCmd, stopCmd, versionCmd, auditCmd, connectCmd(&data), agentsCmd(&data), importCmd(&data), stdioCmd(), pullCmd(&data), pushCmd(&data))
	return r
}

func connectCmd(data *string) *cobra.Command {
	var toolkit, access, scope, project string
	var build, noSkills, write, asJSON bool
	c := &cobra.Command{Use: "connect <claude|vscode|cursor|claude-desktop>", Short: "Add a toolkit to an agent's client",
		Example: "  mcpdeck connect claude --toolkit support --build --write\n  mcpdeck connect vscode --toolkit support --scope project --project .",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			client := map[string]string{"claude": "claude-code", "claude-code": "claude-code", "vscode": "vscode", "code": "vscode", "cursor": "cursor", "claude-desktop": "claude-desktop"}[args[0]]
			if client == "" {
				return fmt.Errorf("client must be claude, vscode, cursor or claude-desktop")
			}
			if toolkit == "" {
				return fmt.Errorf("say which toolkit: --toolkit <name>")
			}
			if project != "" {
				project, _ = filepath.Abs(project)
				if scope == "" {
					scope = "project"
				}
			}
			skills := !noSkills
			var out struct {
				Result struct {
					Token string `json:"token"`
					Agent struct{ Name string }
				}
				Plan struct {
					Command, Link, Snippet, SnippetPath, FirstPrompt string
					Files                                            []struct{ Path, Summary string }
					Notes                                            []string
				}
				PlanID string
			}
			err = cl.do("POST", "/api/connect", map[string]any{"client": client, "toolkitId": toolkit, "access": access, "canBuild": build,
				"scope": scope, "projectDir": project, "skills": skills}, &out)
			if err != nil {
				return err
			}
			var written struct{ Written []string }
			if write {
				if err := cl.do("POST", "/api/plans/"+out.PlanID+"/apply", nil, &written); err != nil {
					return err
				}
			}
			if asJSON {
				printJSON(map[string]any{"agent": out.Result.Agent.Name, "command": out.Plan.Command, "link": out.Plan.Link, "snippet": out.Plan.Snippet, "written": written.Written})
				return nil
			}
			fmt.Printf("Connected %s to %s. The token is shown once.\n\n", out.Result.Agent.Name, toolkit)
			if write {
				fmt.Println("Wrote:")
				for _, f := range written.Written {
					fmt.Println("  " + f)
				}
				fmt.Println("Backups of changed config files end in .mcpdeck-backup.")
			} else {
				fmt.Println("Run:\n\n" + out.Plan.Command + "\n")
				if out.Plan.Link != "" {
					fmt.Println("Or open:", out.Plan.Link)
				}
				fmt.Printf("\nOr add to %s:\n%s\n", out.Plan.SnippetPath, out.Plan.Snippet)
				fmt.Println("\nAdd --write to write the config, builder subagent and skills for you:")
				for _, f := range out.Plan.Files {
					fmt.Printf("  %s (%s)\n", f.Path, f.Summary)
				}
			}
			for _, n := range out.Plan.Notes {
				fmt.Println("\nNote: " + n)
			}
			fmt.Println("\nFirst prompt to try: " + out.Plan.FirstPrompt)
			return nil
		}}
	c.Flags().StringVar(&toolkit, "toolkit", "", "toolkit name")
	c.Flags().StringVar(&access, "access", "read", "read or read_write")
	c.Flags().BoolVar(&build, "build", false, "let the agent build tools (through a builder subagent)")
	c.Flags().StringVar(&scope, "scope", "", "user (default) or project")
	c.Flags().StringVar(&project, "project", "", "project folder for project scope")
	c.Flags().BoolVar(&noSkills, "no-skills", false, "don't install the mcpDeck skills")
	c.Flags().BoolVar(&write, "write", false, "write the config files (a backup is kept)")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}

func agentsCmd(data *string) *cobra.Command {
	var asJSON bool
	c := &cobra.Command{Use: "agents [revoke <name>]", Short: "List or revoke agents", Example: "  mcpdeck agents\n  mcpdeck agents revoke \"Cursor\"",
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			var agents []struct {
				ID, Name, ClientLabel, TokenPrefix string
				RevokedAt                          *string
				Connected                          bool
				CallsToday                         int
				Grants                             []struct {
					ToolkitName, Access string
					CanBuild            bool
				}
			}
			if err := cl.do("GET", "/api/agents", nil, &agents); err != nil {
				return err
			}
			if len(args) >= 2 && args[0] == "revoke" {
				name := strings.Join(args[1:], " ")
				for _, a := range agents {
					if a.Name == name && a.RevokedAt == nil {
						if err := cl.do("POST", "/api/agents/"+a.ID+"/revoke", nil, nil); err != nil {
							return err
						}
						fmt.Printf("Revoked %s. Its token is refused from now on.\n", name)
						return nil
					}
				}
				return fmt.Errorf("no active agent named %q", name)
			}
			if asJSON {
				printJSON(agents)
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tCLIENT\tTOOLKITS\tCALLS TODAY\tSTATE")
			for _, a := range agents {
				var tks []string
				for _, g := range a.Grants {
					s := g.ToolkitName + ":" + strings.ReplaceAll(g.Access, "read_write", "rw")
					if g.CanBuild {
						s += "+build"
					}
					tks = append(tks, s)
				}
				state := "idle"
				if a.RevokedAt != nil {
					state = "revoked"
				} else if a.Connected {
					state = "connected"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", a.Name, a.ClientLabel, strings.Join(tks, ", "), a.CallsToday, state)
			}
			return w.Flush()
		}}
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}

func importCmd(data *string) *cobra.Command {
	var apiName, envFile string
	var asJSON bool
	c := &cobra.Command{Use: "import <url-or-file> --api <name>", Short: "Import a Swagger page, OpenAPI, Postman collection or curl file into an API",
		Example: "  mcpdeck import https://petstore3.swagger.io/ --api petstore\n  mcpdeck import support.postman_collection.json --api support --env demo.postman_environment.json",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if apiName == "" {
				return fmt.Errorf("say which API to import into: --api <name> (created if it doesn't exist)")
			}
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			body := map[string]any{"name": apiName}
			if strings.HasPrefix(args[0], "http://") || strings.HasPrefix(args[0], "https://") {
				body["url"] = args[0]
			} else {
				b, err := os.ReadFile(args[0])
				if err != nil {
					return err
				}
				body["text"] = string(b)
			}
			if envFile != "" {
				b, err := os.ReadFile(envFile)
				if err != nil {
					return err
				}
				body["environment"] = string(b)
			}
			var apis []struct{ ID, Name string }
			_ = cl.do("GET", "/api/apis", nil, &apis)
			for _, a := range apis {
				if a.Name == apiName {
					body["apiId"] = a.ID
				}
			}
			var rep struct {
				APIName                               string
				Added, Updated, Credentials, Warnings []string
			}
			if err := cl.do("POST", "/api/import", body, &rep); err != nil {
				return err
			}
			if asJSON {
				printJSON(rep)
				return nil
			}
			fmt.Printf("%s: %d added, %d updated, nothing deleted.\n", rep.APIName, len(rep.Added), len(rep.Updated))
			for _, n := range rep.Added {
				fmt.Println("  + " + rep.APIName + "." + n)
			}
			for _, n := range rep.Updated {
				fmt.Println("  ~ " + rep.APIName + "." + n)
			}
			if len(rep.Credentials) > 0 {
				fmt.Println("Credentials moved into encrypted storage: " + strings.Join(rep.Credentials, ", "))
			}
			for _, w := range rep.Warnings {
				fmt.Println("Note: " + w)
			}
			return nil
		}}
	c.Flags().StringVar(&apiName, "api", "", "API name, as tools call it (crm, billing…)")
	c.Flags().StringVar(&envFile, "env", "", "Postman environment file")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}
