// Package version provides build-time version information for infra-control.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
)

// These variables are set at build time via ldflags, which is how the Makefile
// and GoReleaser stamp a release.
//
// They are deliberately left at their placeholder values here rather than being
// filled in from the module version, because a build that was not stamped
// should say so rather than claim a version it does not have. Get falls back to
// the Go build info when they are unset, which covers `go install`.
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

// resolved caches the answer, since reading build info walks the binary's
// embedded metadata and the result cannot change during a run.
var (
	once     sync.Once
	resolved Info
)

// Get returns the current version info.
//
// Values stamped by ldflags win. Anything left unstamped is filled in from the
// build information the Go toolchain embeds, so a binary produced by
// `go install github.com/ashutosh0x/infra-control/cmd/infractl@v0.1.0` reports
// v0.1.0 rather than "dev". Without this fallback, every install that did not
// go through the Makefile would misreport its own version, which is exactly the
// information a bug report needs to be accurate.
func Get() Info {
	once.Do(func() {
		resolved = Info{
			Version:   Version,
			Commit:    Commit,
			BuildDate: BuildDate,
			GoVersion: runtime.Version(),
			Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		}

		info, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}

		if resolved.Version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			resolved.Version = info.Main.Version
		}

		// Track whether the commit came from build info rather than ldflags,
		// because only a derived commit should carry the dirty marker. A commit
		// the builder stamped explicitly is their claim to make, and appending
		// to it would contradict what they asked for.
		commitFromBuildInfo := false
		dirty := false

		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if resolved.Commit == "unknown" && setting.Value != "" {
					resolved.Commit = shortSHA(setting.Value)
					commitFromBuildInfo = true
				}
			case "vcs.time":
				if resolved.BuildDate == "unknown" && setting.Value != "" {
					resolved.BuildDate = setting.Value
				}
			case "vcs.modified":
				dirty = setting.Value == "true"
			}
		}

		// A binary built from a dirty tree is marked, so a version quoted in a
		// bug report cannot be mistaken for a clean build of that commit.
		if dirty && commitFromBuildInfo {
			resolved.Commit += "-dirty"
		}
	})

	return resolved
}

// shortSHA trims a full commit hash to the usual 12-character prefix.
func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// String returns a human-readable version string.
func (i Info) String() string {
	return fmt.Sprintf("infra-control %s (commit: %s, built: %s, go: %s, platform: %s)",
		i.Version, i.Commit, i.BuildDate, i.GoVersion, i.Platform)
}

// IsRelease reports whether this binary came from a tagged release rather than
// a local or untagged build.
func IsRelease() bool {
	v := Get().Version
	return v != "dev" && v != "(devel)" && strings.HasPrefix(v, "v")
}
