// Package version provides build-time version information for BlackCat.
package version

import (
	"fmt"
	"runtime/debug"
)

// Version is the semantic version, set at build time via -X ldflags.
var Version = "dev"

// Commit is the VCS commit hash, set at build time via -X ldflags.
var Commit = "unknown"

// BuildDate is the build timestamp in RFC3339 format, set at build time via -X ldflags.
var BuildDate = ""

// Info returns a human-readable version string.
// It attempts to populate missing fields from runtime debug info if ldflags were not provided.
func Info() string {
	version := Version
	commit := Commit
	buildDate := BuildDate

	// If ldflags were not set, try to read from runtime debug info
	if version == "dev" && commit == "unknown" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" && setting.Value != "" {
					commit = setting.Value[:12] // truncate to short hash
					break
				}
			}
		}
	}

	// Format the output string
	if buildDate != "" {
		return fmt.Sprintf("%s (%s) built %s", version, commit, buildDate)
	}
	return fmt.Sprintf("%s (%s) built ", version, commit)
}
