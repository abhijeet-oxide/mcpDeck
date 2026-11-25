// Package web embeds the built UI (UI-9).
package web

import "embed"

// Dist is the Vite build output. Run `make build` to fill it.
//
//go:embed all:dist
var Dist embed.FS
