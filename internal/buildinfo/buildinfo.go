// Package buildinfo exposes release identity injected by the build pipeline.
package buildinfo

import "fmt"

// These values are set with -ldflags during a release build. Development and
// test builds intentionally retain their explicit fallback values.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// String returns one concise, support-friendly build identity line.
func String() string {
	return fmt.Sprintf("KoraDB %s (commit %s, built %s)", Version, Commit, BuildTime)
}
