package version

import (
	"strings"
	"testing"
)

func TestGetAlwaysReportsRuntimeFacts(t *testing.T) {
	// Go version and platform come from the runtime, so they are known even in
	// a binary that was never stamped. A bug report is much less useful without
	// them, so they must never be empty.
	info := Get()

	if info.GoVersion == "" || !strings.HasPrefix(info.GoVersion, "go") {
		t.Errorf("GoVersion = %q, want a go1.x string", info.GoVersion)
	}
	if !strings.Contains(info.Platform, "/") {
		t.Errorf("Platform = %q, want os/arch", info.Platform)
	}
}

func TestGetIsStableAcrossCalls(t *testing.T) {
	// The result is cached; two calls must not disagree.
	if Get() != Get() {
		t.Error("Get returned different values on successive calls")
	}
}

func TestShortSHA(t *testing.T) {
	cases := map[string]string{
		"ab2b0862f88c7cdcfcb4b99c2a900ef11caf0da2": "ab2b0862f88c",
		"short": "short",
		"":      "",
	}
	for input, want := range cases {
		if got := shortSHA(input); got != want {
			t.Errorf("shortSHA(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestStringNamesEveryField(t *testing.T) {
	s := Info{
		Version:   "v1.2.3",
		Commit:    "abc123",
		BuildDate: "2026-08-29",
		GoVersion: "go1.25.0",
		Platform:  "linux/amd64",
	}.String()

	for _, want := range []string{"v1.2.3", "abc123", "2026-08-29", "go1.25.0", "linux/amd64"} {
		if !strings.Contains(s, want) {
			t.Errorf("String() omitted %q: %s", want, s)
		}
	}
}

func TestIsReleaseRejectsUnstamped(t *testing.T) {
	// A test binary is never a tagged release, so this must be false here. The
	// point of the check is that a build reporting "dev" cannot be mistaken for
	// a release in a bug report.
	if got := Get().Version; got == "dev" && IsRelease() {
		t.Error("IsRelease must be false for an unstamped build")
	}
}
