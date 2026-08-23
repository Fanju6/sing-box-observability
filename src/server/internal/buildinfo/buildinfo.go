package buildinfo

import "fmt"

// These values are replaced by release builds using -ldflags -X.
var (
	Version   = "0.1.0-dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func String() string {
	return fmt.Sprintf("sing-box-observability %s (commit %s, built %s)", Version, Commit, BuildTime)
}
