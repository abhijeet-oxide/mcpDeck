package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// lintCmd checks a toolkit (and optionally an unsaved tool file) against
// the best-practice rules; it exits 1 on a blocking finding, for CI (LINT-1).
func lintCmd(data *string) *cobra.Command {
	var file string
	var asJSON, strict bool
	c := &cobra.Command{Use: "lint <toolkit> [--file tool.ts]", Short: "Check tools against MCP best practices (exit 1 on a blocking finding)",
		Example: "  mcpdeck lint support\n  mcpdeck lint support --file ./tools/get_account.ts --json", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			in := map[string]any{}
			if file != "" {
				b, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				in["draft"] = string(b)
			}
			var rep struct {
				Score      int    `json:"score"`
				ListTokens int    `json:"listTokens"`
				DraftError string `json:"draftError"`
				Findings   []struct {
					Rule, Level, Title, Detail, Fix, Tool, Source string
				} `json:"findings"`
				Draft []struct {
					Rule, Level, Title, Detail, Fix, Tool, Source string
				} `json:"draft"`
			}
			var raw json.RawMessage
			if err := cl.do("POST", "/api/toolkits/"+args[0]+"/lint", in, &raw); err != nil {
				return err
			}
			_ = json.Unmarshal(raw, &rep)
			all := append(rep.Findings, rep.Draft...)
			blocks, warns := 0, 0
			for _, f := range all {
				switch f.Level {
				case "block":
					blocks++
				case "warn":
					warns++
				}
			}
			if asJSON {
				fmt.Println(string(raw))
			} else {
				if rep.DraftError != "" {
					fmt.Println("error: " + rep.DraftError)
				}
				for _, f := range all {
					where := f.Tool
					if where == "" {
						where = args[0]
					}
					fmt.Printf("%-5s %s %s: %s\n", f.Level, f.Rule, where, f.Title)
					if f.Fix != "" {
						fmt.Printf("      fix: %s\n", f.Fix)
					}
					if f.Source != "" {
						fmt.Printf("      why: %s\n", f.Source)
					}
				}
				fmt.Printf("%d blocking, %s, score %d/100, tools/list %d tokens.\n", blocks, plural(warns, "warning"), rep.Score, rep.ListTokens)
			}
			if blocks > 0 || rep.DraftError != "" || (strict && warns > 0) {
				os.Exit(1)
			}
			return nil
		}}
	c.Flags().StringVar(&file, "file", "", "an unsaved tool file to check with the toolkit")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	c.Flags().BoolVar(&strict, "strict", false, "exit 1 on warnings too")
	return c
}

// plural writes "1 warning" or "2 warnings".
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
