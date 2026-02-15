package version

import "fmt"

// Set via ldflags at build time.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func String() string {
	return fmt.Sprintf("machinemon %s (commit %s, built %s)", Version, Commit, BuildTime)
}
