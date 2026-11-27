package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// recordCmd opens the browser for a person to click through a task once (REC-1).
func recordCmd(data *string) *cobra.Command {
	var asJSON bool
	c := &cobra.Command{Use: "record <api> <url>", Short: "Open the browser at a web app, click through a task once, and add its pages and forms as requests",
		Example: "  mcpdeck record expenses https://expenses.corp.example/OA_HTML/claims", Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			cl.timeout = 31 * time.Minute
			if !asJSON {
				fmt.Println("A browser window is opening. Sign in, do the task once, then close the window.")
			}
			var out map[string]any
			if err := cl.do("POST", "/api/record", map[string]any{"api": args[0], "url": args[1]}, &out); err != nil {
				return err
			}
			if asJSON {
				printJSON(out)
				return nil
			}
			reqs, _ := out["requests"].([]any)
			fmt.Printf("Recorded %d requests into %s:\n", len(reqs), args[0])
			for _, r := range reqs {
				m, _ := r.(map[string]any)
				fmt.Printf("  %-28v %-6v %v → %v\n", m["name"], m["method"], m["path"], m["status"])
			}
			fmt.Println("Next: write tools for them (Toolkits › New tool), or ask your agent to.")
			return nil
		}}
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}

// apiFilesCmd sets or lists an API's files folder (HTTP-3).
func apiFilesCmd(data *string) *cobra.Command {
	var folder string
	var off, asJSON bool
	c := &cobra.Command{Use: "files <api> [--folder <dir> | --off]", Short: "Pick the folder an API's tools may upload files from, or list it",
		Example: "  mcpdeck api files expenses --folder ~/Receipts\n  mcpdeck api files expenses --json", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			a, err := findAPI(cl, args[0])
			if err != nil {
				return err
			}
			if folder != "" || off {
				dir := ""
				if !off {
					if dir, err = filepath.Abs(expandHome(folder)); err != nil {
						return err
					}
				}
				if err := cl.do("PUT", "/api/apis/"+a.ID+"/files-folder", map[string]any{"dir": dir}, nil); err != nil {
					return err
				}
				if !asJSON {
					if off {
						fmt.Printf("%s tools can't upload files now.\n", a.Name)
					} else {
						fmt.Printf("%s tools may upload files from %s.\n", a.Name, dir)
					}
					return nil
				}
			}
			var files []map[string]any
			if err := cl.do("GET", "/api/apis/"+a.ID+"/files", nil, &files); err != nil {
				return err
			}
			if asJSON {
				printJSON(files)
				return nil
			}
			for _, f := range files {
				fmt.Printf("  %-40v %8v bytes  %v\n", f["name"], f["size"], f["modified"])
			}
			return nil
		}}
	c.Flags().StringVar(&folder, "folder", "", "the folder")
	c.Flags().BoolVar(&off, "off", false, "stop tools uploading files")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}

func expandHome(p string) string {
	if len(p) > 1 && p[0] == '~' && (p[1] == '/' || p[1] == '\\') {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

// toolkitFileCmds export and import toolkit files (EXP-1).
func toolkitFileCmds(data *string) []*cobra.Command {
	var out string
	export := &cobra.Command{Use: "export <toolkit> [--out file.mcpdeck]", Short: "Save a toolkit, its applications and tools as one file to share (no secrets)",
		Example: "  mcpdeck toolkit export expenses --out expenses.mcpdeck", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			var b json.RawMessage
			if err := cl.do("GET", "/api/toolkits/"+args[0]+"/export", nil, &b); err != nil {
				return err
			}
			if out == "" {
				out = args[0] + ".mcpdeck"
			}
			var pretty map[string]any
			_ = json.Unmarshal(b, &pretty)
			data, _ := json.MarshalIndent(pretty, "", "  ")
			if err := os.WriteFile(out, append(data, '\n'), 0o600); err != nil {
				return err
			}
			fmt.Printf("Saved %s. Others import it with: mcpdeck toolkit import %s\n", out, filepath.Base(out))
			return nil
		}}
	export.Flags().StringVar(&out, "out", "", "file to write (default <toolkit>.mcpdeck)")
	var asJSON bool
	imp := &cobra.Command{Use: "import <file.mcpdeck>", Short: "Add a shared toolkit: its applications and tools (write tools wait for your approval)",
		Example: "  mcpdeck toolkit import expenses.mcpdeck", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			raw, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			var b json.RawMessage = raw
			var res map[string]any
			if err := cl.do("POST", "/api/toolkit-import", b, &res); err != nil {
				return err
			}
			if asJSON {
				printJSON(res)
				return nil
			}
			fmt.Println(res["message"])
			return nil
		}}
	imp.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return []*cobra.Command{export, imp}
}
