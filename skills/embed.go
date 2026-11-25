// Package skills embeds the agent skills that `mcpdeck connect` installs
// (SKL-1, SKL-2).
package skills

import "embed"

// FS holds each skill's SKILL.md and scripts.
//
//go:embed mcpdeck-tools mcpdeck-request-tool mcpdeck-trim-output mcpdeck-build-tool
var FS embed.FS

// Names are the bundled skills, in install order.
var Names = []string{"mcpdeck-tools", "mcpdeck-request-tool", "mcpdeck-trim-output", "mcpdeck-build-tool"}
