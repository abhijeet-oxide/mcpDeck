// Command tokcount prints the tokens of files with the encoder mcpDeck
// reports everywhere (cl100k_base), for the context sizes in docs/agents.md.
//
//	go run ./tools/tokcount llms.txt AGENTS.md skills/*/SKILL.md
package main

import (
	"fmt"
	"os"

	"github.com/abhijeet-oxide/mcpdeck/internal/tokens"
)

func main() {
	total := 0
	for _, f := range os.Args[1:] {
		b, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		n := tokens.Count(string(b))
		total += n
		fmt.Printf("%6d  %s\n", n, f)
	}
	if len(os.Args) > 2 {
		fmt.Printf("%6d  total\n", total)
	}
}
