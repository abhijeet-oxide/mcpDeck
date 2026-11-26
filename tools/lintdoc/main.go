// Command lintdoc prints docs/lint-rules.md from the linter's rule catalog.
package main

import (
	"fmt"

	"github.com/abhijeet-oxide/mcpdeck/internal/lint"
)

func main() { fmt.Print(lint.Markdown()) }
