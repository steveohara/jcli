// Package version exposes build information injected at compile time via
// -ldflags.  When the binary is built without ldflags (e.g. plain go run) the
// variables fall back to sensible defaults.
package version

import (
	"fmt"
	"runtime"
)

// Build information.  These variables are overwritten by the linker when the
// binary is built with:
//
//	-ldflags "-X 'github.com/steveohara/jcli/internal/version.Version=v1.2.3'
//	          -X 'github.com/steveohara/jcli/internal/version.Commit=abc1234'
//	          -X 'github.com/steveohara/jcli/internal/version.BuildDate=2026-05-01T00:00:00Z'"
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Info returns a multi-line string with all build metadata.
func Info() string {
	return fmt.Sprintf(
		"jcli %s\n  commit:     %s\n  built:      %s\n  go version: %s",
		Version, Commit, BuildDate, runtime.Version(),
	)
}
