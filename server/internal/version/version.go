// Package version reports build information for the Kirby server.
package version

import "fmt"

var (
	// Version is replaced by release builds.
	Version = "dev"
	// Commit is replaced by release builds.
	Commit = "unknown"
	// BuildDate is replaced by release builds.
	BuildDate = "unknown"
)

// String returns stable, human-readable build information.
// Parameters: none.
// Returns: the release version, source commit, and build time.
// Throws: none.
func String() string {
	return fmt.Sprintf("%s (commit=%s, built=%s)", Version, Commit, BuildDate)
}
