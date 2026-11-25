package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type toolDetail struct {
	Name   string
	Latest struct {
		Source string
		Tests  json.RawMessage
		Status string
		N      int
	}
}

// pullCmd writes a toolkit's tools as files (FILE-1).
func pullCmd(data *string) *cobra.Command {
	return &cobra.Command{Use: "pull <toolkit> <dir>", Short: "Write a toolkit's tools as .ts files with types, for any editor or coding agent",
		Example: "  mcpdeck pull support ./tools && npx tsc --noEmit -p ./tools", Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			var tk struct {
				ToolRows []toolDetail `json:"toolRows"`
			}
			if err := cl.do("GET", "/api/toolkits/"+args[0], nil, &tk); err != nil {
				return err
			}
			dir := args[1]
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			for _, t := range tk.ToolRows {
				if err := os.WriteFile(filepath.Join(dir, t.Name+".ts"), []byte(t.Latest.Source), 0o644); err != nil {
					return err
				}
				if len(t.Latest.Tests) > 2 {
					_ = os.WriteFile(filepath.Join(dir, t.Name+".tests.json"), t.Latest.Tests, 0o644)
				}
				fmt.Printf("  %s.ts (v%d, %s)\n", t.Name, t.Latest.N, t.Latest.Status)
			}
			req, _ := newRawGet(cl, "/api/toolkits/"+args[0]+"/types.d.ts")
			_ = os.WriteFile(filepath.Join(dir, "mcpdeck.d.ts"), req, 0o644)
			tsconfig := `{"compilerOptions":{"target":"ES2020","module":"ESNext","strict":false,"noEmit":true,"skipLibCheck":true,"lib":["ES2020"]},"include":["*.ts"]}` + "\n"
			_ = os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfig), 0o644)
			fmt.Printf("Wrote %d tools, mcpdeck.d.ts and tsconfig.json to %s. Edit, then: mcpdeck push %s --toolkit %s\n", len(tk.ToolRows), dir, dir, args[0])
			return nil
		}}
}

var nameRe = regexp.MustCompile(`name:\s*["']([a-z][a-z0-9_]+)["']`)

// pushCmd saves changed files as new versions (FILE-1).
func pushCmd(data *string) *cobra.Command {
	var toolkit string
	c := &cobra.Command{Use: "push <dir> --toolkit <name>", Short: "Save changed tool files as new versions (tests run, approval rules apply)",
		Example: "  mcpdeck push ./tools --toolkit support", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if toolkit == "" {
				return fmt.Errorf("say which toolkit: --toolkit <name>")
			}
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			var tk struct {
				ToolRows []toolDetail `json:"toolRows"`
			}
			if err := cl.do("GET", "/api/toolkits/"+toolkit, nil, &tk); err != nil {
				return err
			}
			current := map[string]toolDetail{}
			for _, t := range tk.ToolRows {
				current[t.Name] = t
			}
			files, _ := filepath.Glob(filepath.Join(args[0], "*.ts"))
			changed := 0
			for _, f := range files {
				if strings.HasSuffix(f, ".d.ts") {
					continue
				}
				b, err := os.ReadFile(f)
				if err != nil {
					return err
				}
				src := string(b)
				m := nameRe.FindStringSubmatch(src)
				if m == nil {
					fmt.Printf("  skip %s: no tool name\n", filepath.Base(f))
					continue
				}
				cur, ok := current[m[1]]
				if ok && strings.TrimSpace(cur.Latest.Source) == strings.TrimSpace(src) {
					continue
				}
				tests := json.RawMessage("[]")
				if ok && len(cur.Latest.Tests) > 0 {
					tests = cur.Latest.Tests
				}
				if tb, err := os.ReadFile(strings.TrimSuffix(f, ".ts") + ".tests.json"); err == nil {
					tests = tb
				}
				var res struct {
					Message string
				}
				if err := cl.do("POST", "/api/toolkits/"+toolkit+"/save", map[string]any{"source": src, "tests": tests}, &res); err != nil {
					fmt.Printf("  %s: %v\n", filepath.Base(f), err)
					continue
				}
				changed++
				fmt.Println("  " + res.Message)
			}
			if changed == 0 {
				fmt.Println("Nothing changed.")
			}
			return nil
		}}
	c.Flags().StringVar(&toolkit, "toolkit", "", "toolkit name")
	return c
}
