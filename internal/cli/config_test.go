package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// newConfigCmd builds a throwaway command carrying the flags a config file
// would realistically set.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "scan", RunE: func(*cobra.Command, []string) error { return nil }}
	cmd.Flags().String("state", "", "")
	cmd.Flags().String("min-severity", "info", "")
	cmd.Flags().String("fail-on", "none", "")
	return cmd
}

// writeConfig writes a config file into a temp dir and chdirs there, so the
// default search path finds it the way it would in a repository.
func writeConfig(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, ".infractl.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	return path
}

// resetConfigState clears the package-level flags the loader reads.
func resetConfigState(t *testing.T) {
	t.Helper()
	previousCfg, previousProfile := cfgFile, profile
	cfgFile, profile = "", ""
	t.Cleanup(func() { cfgFile, profile = previousCfg, previousProfile })
}

func TestConfigFileSuppliesUnsetFlags(t *testing.T) {
	resetConfigState(t)
	writeConfig(t, "state: from-config.tfstate\nmin-severity: high\n")

	cmd := newConfigCmd()
	if err := loadConfig(cmd); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if got, _ := cmd.Flags().GetString("state"); got != "from-config.tfstate" {
		t.Errorf("state = %q, want from-config.tfstate", got)
	}
	if got, _ := cmd.Flags().GetString("min-severity"); got != "high" {
		t.Errorf("min-severity = %q, want high", got)
	}
}

func TestExplicitFlagBeatsConfigFile(t *testing.T) {
	// A flag the user typed must survive. This is the property that lets a
	// pipeline override one setting without restating the whole file.
	resetConfigState(t)
	writeConfig(t, "min-severity: high\n")

	cmd := newConfigCmd()
	if err := cmd.Flags().Set("min-severity", "critical"); err != nil {
		t.Fatalf("set flag: %v", err)
	}

	if err := loadConfig(cmd); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if got, _ := cmd.Flags().GetString("min-severity"); got != "critical" {
		t.Errorf("min-severity = %q, want critical (the typed flag must win)", got)
	}
}

func TestEnvironmentBeatsConfigFile(t *testing.T) {
	resetConfigState(t)
	writeConfig(t, "min-severity: low\n")
	t.Setenv("INFRACTL_MIN_SEVERITY", "critical")

	cmd := newConfigCmd()
	if err := loadConfig(cmd); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if got, _ := cmd.Flags().GetString("min-severity"); got != "critical" {
		t.Errorf("min-severity = %q, want critical (env must beat the file)", got)
	}
}

func TestKebabFlagMapsToScreamingSnakeEnv(t *testing.T) {
	// --min-severity is INFRACTL_MIN_SEVERITY. Getting this mapping wrong makes
	// every multi-word setting silently unconfigurable from the environment.
	resetConfigState(t)
	t.Setenv("INFRACTL_FAIL_ON", "high")

	cmd := newConfigCmd()
	if err := loadConfig(cmd); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if got, _ := cmd.Flags().GetString("fail-on"); got != "high" {
		t.Errorf("fail-on = %q, want high", got)
	}
}

func TestProfileOverridesTopLevel(t *testing.T) {
	resetConfigState(t)
	writeConfig(t, `
min-severity: info
fail-on: none
profiles:
  production:
    min-severity: high
    fail-on: high
`)
	profile = "production"

	cmd := newConfigCmd()
	if err := loadConfig(cmd); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if got, _ := cmd.Flags().GetString("min-severity"); got != "high" {
		t.Errorf("min-severity = %q, want high from the profile", got)
	}
	if got, _ := cmd.Flags().GetString("fail-on"); got != "high" {
		t.Errorf("fail-on = %q, want high from the profile", got)
	}
}

func TestUnknownProfileIsAnError(t *testing.T) {
	// Silently ignoring a misspelled profile would run production settings
	// against staging, or the reverse.
	resetConfigState(t)
	writeConfig(t, "profiles:\n  production:\n    min-severity: high\n")
	profile = "no-such-profile"

	if err := loadConfig(newConfigCmd()); err == nil {
		t.Fatal("expected an error for an undefined profile")
	}
}

func TestMissingConfigFileIsNotAnError(t *testing.T) {
	// Most invocations have no config file, and requiring one would make the
	// simple case worse to serve the complex one.
	resetConfigState(t)

	dir := t.TempDir()
	previous, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	if err := loadConfig(newConfigCmd()); err != nil {
		t.Errorf("an absent config file should not error: %v", err)
	}
}

func TestExplicitlyNamedMissingConfigIsAnError(t *testing.T) {
	resetConfigState(t)
	cfgFile = filepath.Join(t.TempDir(), "nope.yaml")

	if err := loadConfig(newConfigCmd()); err == nil {
		t.Error("an explicitly named missing config file should error")
	}
}

func TestMalformedConfigIsAnError(t *testing.T) {
	// A file that exists but cannot be parsed was clearly meant to be used.
	resetConfigState(t)
	path := writeConfig(t, "min-severity: [unclosed\n")
	cfgFile = path

	if err := loadConfig(newConfigCmd()); err == nil {
		t.Error("a malformed config file should error rather than be skipped")
	}
}

// TestViperIsNotSharedBetweenLoads guards against a package-level viper, whose
// state would leak between the loads that the tests above perform.
func TestViperIsNotSharedBetweenLoads(t *testing.T) {
	resetConfigState(t)
	writeConfig(t, "min-severity: high\n")

	first := newConfigCmd()
	if err := loadConfig(first); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	// A fresh command with no config in scope must not inherit the previous value.
	resetConfigState(t)
	dir := t.TempDir()
	previous, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	second := newConfigCmd()
	if err := loadConfig(second); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if got, _ := second.Flags().GetString("min-severity"); got != "info" {
		t.Errorf("min-severity = %q, want the default; viper state leaked between loads", got)
	}

	_ = viper.New() // documents that each load builds its own instance
}
