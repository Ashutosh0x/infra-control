// Package cli provides the command-line interface for infractl.
//
// infractl is the operator-facing surface of infra-control. It reads Terraform
// and OpenTofu state, compares it against live infrastructure, scores what it
// finds, and reports the result in a form suited to either a terminal or a
// pipeline.
//
// Command implementations in this package follow three rules:
//
//   - Results go to stdout, progress and diagnostics go to stderr, so that
//     `infractl ... -o json | jq` works without filtering.
//   - Exit codes distinguish failure from findings. A scan that runs correctly
//     and detects drift exits 3, not 1.
//   - A command with no data source behind it returns an error saying so. It
//     never prints a placeholder result, because a placeholder is
//     indistinguishable from a real answer to whoever reads the output.
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/ashutosh0x/infra-control/pkg/version"
	"github.com/spf13/cobra"
)

var (
	cfgFile      string
	outputFormat string
	verbose      bool
	noColor      bool
	colorWhen    string
	asciiOnly    bool
	quiet        bool
	profile      string
)

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "infractl",
	Short: "Infrastructure control plane for drift detection, policy, and risk",
	Long: `infractl reads Terraform and OpenTofu state, compares it against live
infrastructure, and reports drift, policy violations, and risk.

Local analysis, no server required:
  infractl state inspect        Summarise a state file
  infractl drift scan           Compare state against a live snapshot
  infractl plan analyse         Score a plan for blast radius and destructive changes
  infractl risk assess          Score resources across security, reliability, cost, compliance

Output:
  Every command supports --output table|wide|json|yaml|csv|tsv|name|go-template.
  Drift additionally supports --output sarif for GitHub code scanning.
  Results go to stdout and progress goes to stderr, so piping into jq or a
  spreadsheet needs no filtering.

Exit codes:
  0  success, nothing found
  1  the command failed
  2  invalid arguments or flags
  3  success, but drift or policy violations were found
  4  required configuration or credentials are missing
  5  a required backend was unreachable

Configuration, highest precedence first:
  1. Command-line flags
  2. Environment variables, prefixed INFRACTL_ (--min-severity is INFRACTL_MIN_SEVERITY)
  3. The config file: --config, else ./.infractl.yaml, else $HOME/.infractl.yaml
  4. Flag defaults

  Use --profile to select a named block from the config file, so one file can
  hold settings for staging and production without either leaking into the other.`,
	Example: `  # Summarise what Terraform is managing
  infractl state inspect terraform.tfstate

  # Detect drift between state and a live snapshot, as JSON for a pipeline
  infractl drift scan --state terraform.tfstate --live live.json -o json

  # Fail a CI job when a plan destroys anything
  infractl plan analyse plan.json --fail-on destructive

  # Score every resource in state and show only the worst
  infractl risk assess --state terraform.tfstate --min-level high`,
	Version:       version.Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	// Config is resolved before the runtime is built, so that a format or
	// colour setting from the file is honoured by the renderer. Cobra runs this
	// hook before it validates required flags, which is what lets the config
	// file satisfy a flag the user did not type.
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if err := loadConfig(cmd); err != nil {
			return err
		}
		return initRuntime()
	},
}

// Execute runs the root command and translates the outcome into an exit status.
func Execute() {
	err := rootCmd.Execute()
	if err == nil {
		return
	}

	code := codeOf(err)

	// A usage error is the one case where the help text is worth printing:
	// the user got the invocation wrong and needs to see the correct shape.
	if code == ExitUsage {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		if cmd, _, findErr := rootCmd.Find(os.Args[1:]); findErr == nil && cmd != nil {
			fmt.Fprintln(os.Stderr, cmd.UsageString())
		}
		os.Exit(code)
	}

	// The renderer may not exist yet if the failure happened during setup.
	if rt != nil && rt.UI != nil {
		rt.UI.Failure("%v", err)
	} else if !quiet {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	os.Exit(code)
}

// findingsExit returns the exit code a scan should produce given what it found.
func findingsExit(found int, failOnFindings bool) error {
	if found > 0 && failOnFindings {
		return &exitError{code: ExitFindings, err: errors.New("findings detected")}
	}
	return nil
}

func init() {
	pf := rootCmd.PersistentFlags()

	// Configuration
	pf.StringVar(&cfgFile, "config", "", "config file (default ./.infractl.yaml, then $HOME/.infractl.yaml)")
	pf.StringVar(&profile, "profile", "", "named configuration profile to use")

	// Output control
	pf.StringVarP(&outputFormat, "output", "o", "table",
		"output format: table, wide, json, yaml, csv, tsv, name, sarif, go-template=TMPL")
	pf.BoolVarP(&verbose, "verbose", "v", false, "enable verbose diagnostic output on stderr")
	pf.StringVar(&colorWhen, "color", "auto", "when to use color: auto, always, never")
	pf.BoolVar(&noColor, "no-color", false, "disable color (shorthand for --color=never)")
	pf.BoolVar(&asciiOnly, "ascii", false, "use ASCII symbols instead of box-drawing characters")
	pf.BoolVarP(&quiet, "quiet", "q", false, "suppress progress output; results and errors still print")

	rootCmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")

	// Group commands so that `infractl --help` reads as a map of the tool rather
	// than one flat alphabetical list.
	rootCmd.AddGroup(
		&cobra.Group{ID: "analyse", Title: "Local analysis (no server required):"},
		&cobra.Group{ID: "platform", Title: "Control plane (requires a server):"},
		&cobra.Group{ID: "system", Title: "Configuration and diagnostics:"},
	)
}
