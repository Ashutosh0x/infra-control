// Package version provides build-time version information for infra-control.
package version

import (
	"fmt"
	"runtime"
)

// These variables are set at build time via ldflags.
var (
	// Version is the semantic version of the build.
	Version = "dev"
	// Commit is the git commit SHA of the build.
	Commit = "unknown"
	// BuildDate is the ISO 8601 date of the build.
	BuildDate = "unknown"
)

// Info holds the complete version information.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// Get returns the current version info.
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a human-readable version string.
func (i Info) String() string {
	return fmt.Sprintf("infra-control %s (commit: %s, built: %s, go: %s, platform: %s)",
		i.Version, i.Commit, i.BuildDate, i.GoVersion, i.Platform)
}
