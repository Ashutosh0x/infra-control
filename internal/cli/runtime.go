package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ashutosh0x/infra-control/internal/ui"
)

// Exit codes. These are part of the CLI's contract: scripts branch on them, so
// they may be added to but never reassigned.
//
// The drift-specific codes follow the convention set by diff(1) and adopted by
// most policy tools, where a non-zero exit distinguishes "the command worked and
// found something" from "the command failed".
const (
	// ExitOK means the command succeeded and found nothing that needs attention.
	ExitOK = 0
	// ExitError means the command failed. The reason is on stderr.
	ExitError = 1
	// ExitUsage means the arguments or flags were invalid.
	ExitUsage = 2
	// ExitFindings means the command succeeded and found drift, policy
	// violations, or risk above the configured threshold. CI pipelines gate on this.
	ExitFindings = 3
	// ExitConfig means required configuration or credentials are missing.
	ExitConfig = 4
	// ExitUnavailable means a required backend could not be reached.
	ExitUnavailable = 5
)

// exitError carries an exit code alongside an error so that Execute can map a
// failure onto the right status without every command calling os.Exit itself.
// Keeping os.Exit in one place is what makes the commands testable.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

// failf builds an error that exits with the given code.
func failf(code int, format string, args ...any) error {
	return &exitError{code: code, err: fmt.Errorf(format, args...)}
}

// codeOf extracts the exit code an error should produce.
func codeOf(err error) int {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return ExitError
}

// runtime holds everything a command needs at execution time. It is built once
// in the root command's PersistentPreRunE, after flags are parsed but before any
// subcommand runs.
type cliRuntime struct {
	// UI renders all output.
	UI *ui.Renderer
	// Format is the resolved output encoding.
	Format ui.Format
	// FormatArg carries the template for go-template output.
	FormatArg string
}

// rt is the process-wide runtime. A package-level value is acceptable here
// because cobra dispatches exactly one command per process.
var rt *cliRuntime

// initRuntime resolves the presentation flags into a runtime.
func initRuntime() error {
	format, formatArg, err := ui.ParseFormat(outputFormat)
	if err != nil {
		return failf(ExitUsage, "%w", err)
	}

	colorMode, err := ui.ParseColorMode(colorWhen)
	if err != nil {
		return failf(ExitUsage, "%w", err)
	}
	// The older --no-color boolean is kept as a shorthand because it is what
	// most CI configurations already set.
	if noColor {
		colorMode = ui.ColorNever
	}
	// Machine-readable output must never carry escape sequences, whatever the
	// colour flags say, or the consumer gets invalid JSON.
	if format.IsMachine() {
		colorMode = ui.ColorNever
	}

	rt = &cliRuntime{
		Format:    format,
		FormatArg: formatArg,
		UI: ui.New(ui.Options{
			Color: colorMode,
			// Progress chatter would interleave with a machine-readable payload,
			// so it is suppressed whenever the output is being parsed.
			Quiet: quiet || format.IsMachine(),
			ASCII: asciiOnly,
		}),
	}
	return nil
}

// write renders a view in the runtime's configured format.
func (r *cliRuntime) write(view ui.View) error {
	if err := r.UI.Write(view, r.Format, r.FormatArg); err != nil {
		return failf(ExitError, "%w", err)
	}
	return nil
}

// requireFile checks that a path exists and is readable before the command
// commits to any work, so the user gets one clear error rather than a failure
// part-way through a scan.
func requireFile(path, purpose string) error {
	if path == "" {
		return failf(ExitUsage, "no %s given", purpose)
	}

	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return failf(ExitUsage, "%s not found: %s", purpose, path)
	}
	if err != nil {
		return failf(ExitError, "cannot read %s %s: %w", purpose, path, err)
	}
	if info.IsDir() {
		return failf(ExitUsage, "%s is a directory, not a file: %s", purpose, path)
	}
	return nil
}

// splitList parses a comma-separated flag value into trimmed, non-empty items.
func splitList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
