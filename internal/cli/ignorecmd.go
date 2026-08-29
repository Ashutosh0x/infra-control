package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ashutosh0x/infra-control/internal/ignore"
	"github.com/ashutosh0x/infra-control/internal/ui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Suppression is only trustworthy because every rule carries a reason. That
// property survives contact with reality only if adding a rule is easier than
// living with the noise; hand-writing YAML is not, so people skip it and let
// the noise accumulate instead, which is the failure this whole design exists
// to prevent.
//
// This command writes the rule, keeps the reason mandatory, and refuses to
// prompt for it when nothing is attached to read the prompt.

var (
	ignoreReason     string
	ignoreExpires    string
	ignoreAttributes string
	ignoreFilePath   string
	ignoreDryRun     bool
)

var ignoreCmd = &cobra.Command{
	Use:     "ignore",
	Short:   "Manage drift suppression rules",
	GroupID: "analyse",
	Long: `Manage the rules in .infractl-ignore.yaml that suppress expected drift.

Every rule must state why the drift is expected. A rule may also carry an
expiry, after which it stops suppressing and the scan says so, which is what
stops a temporary exception becoming permanent.`,
}

var ignoreAddCmd = &cobra.Command{
	Use:   "add <address>",
	Short: "Add a suppression rule",
	Long: `Add a rule suppressing drift on a resource.

The reason is required. It is the only thing that makes the file reviewable
six months from now, and a rule without one is rejected rather than defaulted.

Scope the rule to specific attributes when only some of a resource's drift is
expected. An attribute-scoped rule suppresses a finding only when it covers
every changed attribute, so a resource where an expected attribute and an
unexpected one both moved is still reported.`,
	Example: `  # Suppress a whole resource
  infractl ignore add aws_instance.bastion --reason "Decommissioning, INFRA-4821"

  # Suppress only the attributes that move on their own
  infractl ignore add 'aws_autoscaling_group.web' \
    --attributes desired_capacity,min_size \
    --reason "Capacity is managed by the autoscaling policy"

  # Time-box the exception
  infractl ignore add aws_s3_bucket.legacy \
    --reason "Migration in progress" --expires 2026-12-31

  # See the rule without writing it
  infractl ignore add aws_vpc.main --reason "..." --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: runIgnoreAdd,
}

func runIgnoreAdd(cmd *cobra.Command, args []string) error {
	address := args[0]

	reason := strings.TrimSpace(ignoreReason)
	if reason == "" {
		// Never block waiting for input that cannot arrive. A pipeline that
		// hangs on a prompt is worse than one that fails saying what to pass.
		if !rt.UI.IsInteractive() {
			return failf(ExitUsage,
				"a reason is required.\n"+
					"  Pass --reason \"why this drift is expected\".\n"+
					"  Suppression without a stated reason is how an exception becomes permanent.")
		}

		prompted, err := promptForReason(address)
		if err != nil {
			return err
		}
		reason = prompted
	}

	if ignoreExpires != "" {
		if _, err := time.Parse("2006-01-02", ignoreExpires); err != nil {
			return failf(ExitUsage,
				"invalid --expires %q (want YYYY-MM-DD)", ignoreExpires)
		}
	}

	rule := ignore.Rule{
		Address:    address,
		Attributes: splitList(ignoreAttributes),
		Reason:     reason,
		Expires:    ignoreExpires,
	}

	path := ignoreFilePath
	if path == "" {
		cwd, err := os.Getwd()
		if err == nil {
			path = ignore.FindDefault(cwd)
		}
		if path == "" {
			path = ignore.DefaultFilename
		}
	}

	if ignoreDryRun {
		rendered, err := renderRule(rule)
		if err != nil {
			return err
		}
		rt.UI.Printf("%s\n", rt.UI.Apply(ui.StyleMuted, "# would append to "+path))
		rt.UI.Raw(rendered)
		return nil
	}

	if err := appendRule(path, rule); err != nil {
		return err
	}

	rt.UI.Success("Added a suppression rule for %s", address)
	rt.UI.Detail("%s", rule.Describe())

	// The file is re-read rather than trusted, so a rule that would be rejected
	// on the next scan is caught now instead of at the next run.
	rules, err := ignore.Load(path)
	if err != nil {
		return failf(ExitError,
			"the rule was written to %s but the file no longer parses: %w", path, err)
	}
	rt.UI.Detail("%s now holds %d active rule(s)", path, rules.Len())

	if !rt.Format.IsMachine() {
		rt.UI.Println()
		hint(cmd, [][2]string{
			{"Check the effect", "infractl drift scan ... "},
			{"See what it hides", "infractl drift scan ... --no-ignore"},
		})
	}
	return nil
}

// promptForReason asks for a reason on an interactive terminal.
func promptForReason(address string) (string, error) {
	rt.UI.Printf("Why is drift on %s expected?\n", rt.UI.Apply(ui.StyleBold, address))
	rt.UI.Printf("%s ", rt.UI.Apply(ui.StyleMuted, "reason:"))

	var line string
	if _, err := fmt.Fscanln(os.Stdin, &line); err != nil {
		return "", failf(ExitUsage, "no reason given")
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return "", failf(ExitUsage, "no reason given")
	}
	return line, nil
}

// renderRule formats a single rule as it would appear in the file.
func renderRule(rule ignore.Rule) (string, error) {
	encoded, err := yaml.Marshal([]ignore.Rule{rule})
	if err != nil {
		return "", failf(ExitError, "encode rule: %w", err)
	}
	return string(encoded), nil
}

// appendRule adds a rule to the ignore file, creating it when absent.
//
// The file is decoded and re-encoded rather than appended to as text, so that
// a malformed result is impossible and an existing file's rules are validated
// on the way through.
func appendRule(path string, rule ignore.Rule) error {
	file := ignore.File{Version: ignore.SupportedVersion}

	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if unmarshalErr := yaml.Unmarshal(existing, &file); unmarshalErr != nil {
			return failf(ExitError,
				"%s does not parse, so a rule cannot be added safely: %w", path, unmarshalErr)
		}
		if file.Version == 0 {
			file.Version = ignore.SupportedVersion
		}
	case !os.IsNotExist(err):
		return failf(ExitError, "read %s: %w", path, err)
	}

	for _, present := range file.Rules {
		if present.Address == rule.Address && sameAttributes(present.Attributes, rule.Attributes) {
			return failf(ExitUsage,
				"a rule for %s with those attributes already exists in %s:\n  %s",
				rule.Address, path, present.Describe())
		}
	}

	file.Rules = append(file.Rules, rule)

	encoded, err := yaml.Marshal(file)
	if err != nil {
		return failf(ExitError, "encode %s: %w", path, err)
	}

	header := "# Drift suppression rules for infractl.\n" +
		"#\n" +
		"# Every rule states why the drift is expected. Rules with an expiry stop\n" +
		"# suppressing after that date, and the scan reports that they lapsed.\n" +
		"# Suppressed findings are always counted; `--no-ignore` shows them.\n\n"

	if err := os.WriteFile(path, append([]byte(header), encoded...), 0o600); err != nil {
		return failf(ExitError, "write %s: %w", path, err)
	}
	return nil
}

// sameAttributes reports whether two attribute scopes are equivalent.
func sameAttributes(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, item := range a {
		seen[item]++
	}
	for _, item := range b {
		seen[item]--
		if seen[item] < 0 {
			return false
		}
	}
	return true
}

var ignoreListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List suppression rules, including expired ones",
	Args:    cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		path := ignoreFilePath
		if path == "" {
			cwd, err := os.Getwd()
			if err == nil {
				path = ignore.FindDefault(cwd)
			}
		}

		rules, err := ignore.Load(path)
		if err != nil {
			return failf(ExitUsage, "%w", err)
		}

		table := ui.NewTable(
			ui.Column{Title: "ADDRESS", MinWidth: 20, Truncatable: true},
			ui.Column{Title: "ATTRIBUTES", MinWidth: 10, Truncatable: true},
			ui.Column{Title: "EXPIRES", MinWidth: 10},
			ui.Column{Title: "STATUS", MinWidth: 7},
			ui.Column{Title: "REASON", MinWidth: 16, Truncatable: true},
		)

		add := func(rule ignore.Rule, expired bool) {
			status := ui.Styled("active", ui.StyleSuccess)
			if expired {
				status = ui.Styled("expired", ui.StyleWarning)
			}
			attrs := strings.Join(rule.Attributes, ", ")
			if attrs == "" {
				attrs = "(all)"
			}
			expires := rule.Expires
			if expires == "" {
				expires = "never"
			}
			table.Row(ui.Text(rule.Address), ui.Text(attrs), ui.Text(expires), status, ui.Text(rule.Reason))
		}

		for _, rule := range rules.Active() {
			add(rule, false)
		}
		for _, rule := range rules.Expired() {
			add(rule, true)
		}

		return rt.write(ui.View{
			Data:  map[string]any{"active": rules.Active(), "expired": rules.Expired()},
			Table: table,
			Empty: "No suppression rules configured.",
		})
	},
}

func init() {
	rootCmd.AddCommand(ignoreCmd)
	ignoreCmd.AddCommand(ignoreAddCmd, ignoreListCmd)

	ignoreCmd.PersistentFlags().StringVar(&ignoreFilePath, "file", "",
		"ignore file to operate on (default: nearest .infractl-ignore.yaml)")

	f := ignoreAddCmd.Flags()
	f.StringVar(&ignoreReason, "reason", "", "why this drift is expected (required)")
	f.StringVar(&ignoreExpires, "expires", "", "date after which the rule stops applying (YYYY-MM-DD)")
	f.StringVar(&ignoreAttributes, "attributes", "",
		"limit the rule to these attribute paths (comma-separated); omit to cover the whole resource")
	f.BoolVar(&ignoreDryRun, "dry-run", false, "print the rule without writing it")
}
