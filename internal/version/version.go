// Package version holds build information set with -ldflags.
package version

import (
	"fmt"
	"runtime"
)

// Set at build time by GoReleaser.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String is the one-line build description.
func String() string {
	return fmt.Sprintf("mcpdeck %s (%s, %s) %s/%s", Version, Commit, Date, runtime.GOOS, runtime.GOARCH)
}
