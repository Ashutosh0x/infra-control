package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ashutosh0x/infra-control/internal/ignore"
	"github.com/ashutosh0x/infra-control/internal/terraform"
	"github.com/ashutosh0x/infra-control/internal/ui"
	"github.com/ashutosh0x/infra-control/pkg/types"
	"github.com/ashutosh0x/infra-control/pkg/version"
	"github.com/spf13/cobra"
)

var (
	driftStatePath  string
	driftLivePath   string
	driftMinSeverit string
	driftFailOn     string
	driftShowDiff   bool
	driftIncludeUnm bool
	driftIgnorePath string
	driftNoIgnore   bool
	driftFix        bool
	driftEmitImport string
)

var driftCmd = &cobra.Command{
	Use:     "drift",
	Short:   "Detect and report infrastructure drift",
	GroupID: "analyse",
	Long: `Detect drift by comparing Terraform or OpenTofu state against the live
state of your infrastructure.

The comparison runs locally. It reads a state file and a live snapshot, matches
resources by Terraform address, and reports three kinds of divergence:

  modified          the resource exists in both, but attributes disagree
  missing in live   state records a resource that no longer exists
  unmanaged         a live resource that Terraform does not track

Each finding is scored by severity, weighted towards changes that affect
security posture: encryption, public access, IAM, and network rules score higher
than tag or description edits.`,
}

// liveSnapshot is the on-disk format for the live infrastructure read that
// `drift scan` compares against.
//
// Keeping this a plain file rather than a live API call is deliberate: it lets
// drift detection run in a pipeline that has no cloud credentials, and lets the
// snapshot be produced by whatever tool already has read access.
type liveSnapshot struct {
	// CapturedAt records when the snapshot was taken. Drift found against a
	// stale snapshot is reported with the age so it can be judged.
	CapturedAt time.Time `json:"captured_at"`
	// Provider names the cloud the snapshot came from.
	Provider string `json:"provider"`
	// Resources maps Terraform address to the live attributes of that resource.
	Resources map[string]map[string]any `json:"resources"`
}

var driftScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Compare state against a live snapshot and report drift",
	Long: `Compare a Terraform or OpenTofu state file against a snapshot of live
infrastructure, and report every divergence.

The live snapshot is a JSON file mapping Terraform resource addresses to the
attributes observed on the real resource:

  {
    "captured_at": "2026-08-29T09:00:00Z",
    "provider": "aws",
    "resources": {
      "aws_s3_bucket.assets": {
        "bucket": "prod-assets",
        "acl": "public-read"
      }
    }
  }

Attributes that no cloud API returns, and provider bookkeeping fields such as
id, arn, and tags_all, are excluded from the comparison so that they do not
report as drift on every run.`,
	Example: `  # Report drift as a table
  infractl drift scan --state terraform.tfstate --live live.json

  # Show the property-level diff for every finding
  infractl drift scan --state terraform.tfstate --live live.json --show-diff

  # Fail CI when anything at high severity or above is found
  infractl drift scan --state terraform.tfstate --live live.json --fail-on high

  # Emit JSON for a pipeline
  infractl drift scan --state terraform.tfstate --live live.json -o json`,
	Args: cobra.NoArgs,
	RunE: runDriftScan,
}

// driftFinding is one reported divergence, and the unit of both the table and
// the JSON output.
type driftFinding struct {
	Address  string               `json:"address"`
	Type     string               `json:"resource_type"`
	Kind     string               `json:"kind"`
	Severity types.DriftSeverity  `json:"severity"`
	Score    float64              `json:"score"`
	Changes  []types.PropertyDiff `json:"changes,omitempty"`
}

// suppression records that an ignore rule hid a finding.
type suppression struct {
	finding driftFinding
	rule    ignore.Rule
}

// driftReport is the full result of a scan.
type driftReport struct {
	ScannedAt       time.Time      `json:"scanned_at"`
	StateFile       string         `json:"state_file"`
	LiveFile        string         `json:"live_file"`
	SnapshotAge     string         `json:"snapshot_age,omitempty"`
	StateResources  int            `json:"state_resources"`
	LiveResources   int            `json:"live_resources"`
	Findings        []driftFinding `json:"findings"`
	CountsBySeverit map[string]int `json:"counts_by_severity"`
	// Suppressed counts findings hidden by ignore rules. It is always reported,
	// so that suppression is visible rather than silent.
	Suppressed int `json:"suppressed"`
	// SuppressedBy maps each rule that fired to how many findings it hid.
	SuppressedBy map[string]int `json:"suppressed_by,omitempty"`
	// Coverage reports how much of the live estate Terraform actually manages.
	Coverage coverageReport `json:"coverage"`
}

// coverageReport answers "how much of what exists is under Terraform".
//
// It is the number that makes an unmanaged finding mean something. One
// untracked security group is a curiosity; eighty of them, against four hundred
// managed resources, is a different conversation. It is also the figure a team
// can track over time to show the gap closing.
//
// The denominator is the union of what state manages and what the snapshot
// observed, because a resource in state but absent live still counts as
// something Terraform is trying to manage.
type coverageReport struct {
	Managed   int     `json:"managed"`
	Unmanaged int     `json:"unmanaged"`
	Total     int     `json:"total"`
	Percent   float64 `json:"percent"`
	// Partial marks a snapshot that cannot see unmanaged resources at all, so
	// a reader does not mistake "none found" for "none exist".
	Partial bool `json:"partial"`
}

func runDriftScan(cmd *cobra.Command, _ []string) error {
	if err := requireFile(driftStatePath, "state file (--state)"); err != nil {
		return err
	}
	if err := requireFile(driftLivePath, "live snapshot (--live)"); err != nil {
		return err
	}

	minSeverity, err := parseSeverity(driftMinSeverit)
	if err != nil {
		return err
	}
	failAt, failSet, err := parseFailOn(driftFailOn)
	if err != nil {
		return err
	}

	spinner := rt.UI.Spin("Reading Terraform state")
	state, err := terraform.ParseStateFile(driftStatePath)
	if err != nil {
		spinner.Fail("Could not read state")
		return failf(ExitError, "%w", err)
	}
	managed := state.ManagedResources()

	spinner.Update("Reading live snapshot")
	snapshot, err := readLiveSnapshot(driftLivePath)
	if err != nil {
		spinner.Fail("Could not read live snapshot")
		return failf(ExitError, "%w", err)
	}

	spinner.Update("Comparing %d managed resources", len(managed))
	findings := compareForDrift(managed, snapshot, driftIncludeUnm)

	spinner.Update("Applying ignore rules")
	rules, err := loadIgnoreRules()
	if err != nil {
		spinner.Fail("Could not read ignore rules")
		return err
	}

	findings, suppressed := applyIgnoreRules(findings, rules)
	spinner.Stop()

	// An expired rule is reported rather than silently dropped: it stopped
	// suppressing, so the findings it used to hide are about to reappear, and
	// the user needs to know why.
	for _, rule := range rules.Expired() {
		rt.UI.Warn("Ignore rule expired and no longer applies: %s", rule.Describe())
	}

	// Filter by severity, then order worst-first so the most urgent finding is
	// the first thing on screen.
	filtered := findings[:0]
	for _, f := range findings {
		if severityRank(f.Severity) >= severityRank(minSeverity) {
			filtered = append(filtered, f)
		}
	}
	findings = filtered

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Score != findings[j].Score {
			return findings[i].Score > findings[j].Score
		}
		return findings[i].Address < findings[j].Address
	})

	report := driftReport{
		ScannedAt:       time.Now().UTC(),
		StateFile:       driftStatePath,
		LiveFile:        driftLivePath,
		StateResources:  len(managed),
		LiveResources:   len(snapshot.Resources),
		Findings:        findings,
		CountsBySeverit: countBySeverity(findings),
		Suppressed:      len(suppressed),
		SuppressedBy:    countByRule(suppressed),
		Coverage:        computeCoverage(managed, snapshot, driftIncludeUnm),
	}
	if !snapshot.CapturedAt.IsZero() {
		report.SnapshotAge = ui.HumanDuration(time.Since(snapshot.CapturedAt))
	}

	if err := rt.write(driftView(report)); err != nil {
		return err
	}

	// A snapshot old enough to mislead is worth saying out loud, because drift
	// found against stale data may already have been fixed.
	if !snapshot.CapturedAt.IsZero() && time.Since(snapshot.CapturedAt) > 24*time.Hour {
		rt.UI.Warn("Live snapshot is %s old; findings may be stale.", report.SnapshotAge)
	}

	if !rt.Format.IsMachine() {
		printDriftSummary(report)
	}

	remediation := buildRemediation(report, snapshot)

	if driftEmitImport != "" {
		if len(remediation.Imports) == 0 {
			rt.UI.Warn("No unmanaged resources to import; %s not written.", driftEmitImport)
		} else if err := os.WriteFile(driftEmitImport,
			[]byte(renderImportHCL(remediation.Imports)), 0o600); err != nil {
			return failf(ExitError, "write import blocks to %s: %w", driftEmitImport, err)
		} else {
			rt.UI.Success("Wrote %d import block(s) to %s", len(remediation.Imports), driftEmitImport)
		}
	}

	if driftFix && !rt.Format.IsMachine() {
		printRemediation(remediation)
	}

	if !rt.Format.IsMachine() && !driftFix && len(report.Findings) > 0 {
		rt.UI.Println()
		hint(cmd, [][2]string{
			{"How to resolve", "infractl drift scan ... --fix"},
			{"See the diff", "infractl drift scan ... --show-diff"},
		})
	}

	if failSet {
		for _, f := range findings {
			if severityRank(f.Severity) >= severityRank(failAt) {
				return findingsExit(1, true)
			}
		}
	}
	return nil
}

// readLiveSnapshot loads and validates the live infrastructure snapshot.
func readLiveSnapshot(path string) (*liveSnapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open live snapshot %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var snapshot liveSnapshot
	dec := json.NewDecoder(f)
	dec.UseNumber()
	if err := dec.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("parse live snapshot %s: %w", path, err)
	}

	if snapshot.Resources == nil {
		return nil, fmt.Errorf(
			"live snapshot %s has no \"resources\" object; "+
				"expected a map of Terraform address to live attributes", path)
	}
	return &snapshot, nil
}

// compareForDrift runs the state-to-live comparison and scores each result.
func compareForDrift(managed []terraform.StateResource, snapshot *liveSnapshot, includeUnmanaged bool) []driftFinding {
	var findings []driftFinding
	inState := make(map[string]struct{}, len(managed))

	for _, resource := range managed {
		inState[resource.Address] = struct{}{}

		live, present := snapshot.Resources[resource.Address]
		if !present {
			findings = append(findings, driftFinding{
				Address:  resource.Address,
				Type:     resource.Type,
				Kind:     "missing_in_live",
				Severity: types.DriftSeverityHigh,
				Score:    60,
			})
			continue
		}

		fieldDiffs := terraform.CompareAttributes(resource.Attributes, live)
		if len(fieldDiffs) == 0 {
			continue
		}

		changes := make([]types.PropertyDiff, len(fieldDiffs))
		for i, fd := range fieldDiffs {
			sensitive := terraform.IsSensitivePath(fd.Path)
			change := types.PropertyDiff{Path: fd.Path, Sensitive: sensitive}
			// A sensitive value is dropped rather than carried through and masked
			// at render time, so it cannot leak via `-o json`.
			if !sensitive {
				change.Expected = fd.Expected
				change.Actual = fd.Actual
			}
			changes[i] = change
		}

		score := scoreChanges(fieldDiffs)
		findings = append(findings, driftFinding{
			Address:  resource.Address,
			Type:     resource.Type,
			Kind:     "modified",
			Severity: severityForScore(score),
			Score:    score,
			Changes:  changes,
		})
	}

	if includeUnmanaged {
		for address := range snapshot.Resources {
			if _, managed := inState[address]; managed {
				continue
			}
			findings = append(findings, driftFinding{
				Address:  address,
				Type:     resourceTypeOf(address),
				Kind:     "unmanaged",
				Severity: types.DriftSeverityMedium,
				Score:    35,
			})
		}
	}

	return findings
}

// criticalPatterns, highPatterns, and mediumPatterns weight a changed attribute
// by the blast radius of getting it wrong. A public-access or encryption change
// is a security incident; a tag change is bookkeeping.
var (
	criticalPatterns = []string{
		"public_access", "public", "acl", "encryption", "encrypted", "kms_key",
		"iam_policy", "policy", "security_group", "network_acl", "ingress",
		"egress", "password", "secret", "credentials", "auth", "principal",
	}
	highPatterns = []string{
		"instance_type", "machine_type", "size", "availability_zone", "region",
		"vpc", "subnet", "route", "firewall", "port", "protocol", "cidr",
		"deletion_protection", "multi_az", "replica",
	}
	mediumPatterns = []string{
		"backup", "retention", "logging", "monitoring", "versioning",
		"lifecycle", "capacity", "min_size", "max_size",
	}
)

// scoreChanges converts a set of attribute changes into a 0..100 severity score.
func scoreChanges(diffs []terraform.FieldDiff) float64 {
	score := 0.0
	for _, diff := range diffs {
		path := strings.ToLower(diff.Path)
		switch {
		case matchesAny(path, criticalPatterns):
			score += 40
		case matchesAny(path, highPatterns):
			score += 25
		case matchesAny(path, mediumPatterns):
			score += 10
		default:
			score += 2
		}
	}
	if score > 100 {
		score = 100
	}
	return score
}

// matchesAny reports whether the path contains any of the given patterns.
func matchesAny(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// severityForScore maps a numeric score onto a severity band.
func severityForScore(score float64) types.DriftSeverity {
	switch {
	case score >= 80:
		return types.DriftSeverityCritical
	case score >= 50:
		return types.DriftSeverityHigh
	case score >= 25:
		return types.DriftSeverityMedium
	case score >= 10:
		return types.DriftSeverityLow
	default:
		return types.DriftSeverityInfo
	}
}

// severityRank orders severities for threshold comparisons.
func severityRank(s types.DriftSeverity) int {
	switch s {
	case types.DriftSeverityCritical:
		return 5
	case types.DriftSeverityHigh:
		return 4
	case types.DriftSeverityMedium:
		return 3
	case types.DriftSeverityLow:
		return 2
	case types.DriftSeverityInfo:
		return 1
	default:
		return 0
	}
}

// parseSeverity resolves a severity name, defaulting to the lowest band.
func parseSeverity(value string) (types.DriftSeverity, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info", "all":
		return types.DriftSeverityInfo, nil
	case "low":
		return types.DriftSeverityLow, nil
	case "medium":
		return types.DriftSeverityMedium, nil
	case "high":
		return types.DriftSeverityHigh, nil
	case "critical":
		return types.DriftSeverityCritical, nil
	default:
		return "", failf(ExitUsage,
			"invalid severity %q (want critical, high, medium, low, or info)", value)
	}
}

// parseFailOn resolves the --fail-on threshold, reporting whether one was set.
func parseFailOn(value string) (types.DriftSeverity, bool, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" || trimmed == "none" {
		return "", false, nil
	}
	if trimmed == "any" {
		return types.DriftSeverityInfo, true, nil
	}
	severity, err := parseSeverity(trimmed)
	if err != nil {
		return "", false, failf(ExitUsage,
			"invalid --fail-on %q (want none, any, critical, high, medium, low, or info)", value)
	}
	return severity, true, nil
}

// countBySeverity tallies findings per severity band.
func countBySeverity(findings []driftFinding) map[string]int {
	counts := map[string]int{}
	for _, f := range findings {
		counts[string(f.Severity)]++
	}
	return counts
}

// resourceTypeOf extracts a Terraform resource type from an address.
func resourceTypeOf(address string) string {
	if idx := strings.Index(address, "["); idx >= 0 {
		address = address[:idx]
	}
	parts := strings.Split(address, ".")
	if len(parts) < 2 {
		return address
	}
	return parts[len(parts)-2]
}

// driftView builds the table and machine payload for a report.
func driftView(report driftReport) ui.View {
	table := ui.NewTable(
		ui.Column{Title: "SEVERITY", MinWidth: 8},
		ui.Column{Title: "KIND", MinWidth: 9},
		ui.Column{Title: "ADDRESS", MinWidth: 20, Truncatable: true},
		ui.Column{Title: "TYPE", MinWidth: 12, Truncatable: true},
		ui.Column{Title: "CHANGES", Align: ui.AlignRight, MinWidth: 7},
	)

	names := make([]string, 0, len(report.Findings))
	for _, f := range report.Findings {
		changes := ""
		if f.Kind == "modified" {
			changes = fmt.Sprintf("%d", len(f.Changes))
		}
		table.Row(
			ui.Styled(strings.ToUpper(string(f.Severity)), ui.SeverityStyle(string(f.Severity))),
			ui.Text(strings.ReplaceAll(f.Kind, "_", " ")),
			ui.Text(f.Address),
			ui.Text(f.Type),
			ui.Text(changes),
		)
		names = append(names, f.Address)
	}

	return ui.View{
		Data:         report,
		Table:        table,
		Names:        names,
		Sarif:        driftSarifFindings(report),
		SarifTool:    "infractl",
		SarifVersion: version.Version,
		SarifURI:     "https://github.com/Ashutosh0x/infra-control",
		Empty:        fmt.Sprintf("No drift found. %d managed resources match the live snapshot.", report.StateResources),
	}
}

// driftSarifRules describes each finding kind once, so a GitHub alert carries
// an explanation rather than only a resource address.
var driftSarifRules = map[string][2]string{
	"modified": {
		"Resource modified outside Terraform",
		"A resource managed by Terraform has attributes that disagree with state. " +
			"Someone changed it outside the Terraform workflow, so the next apply will " +
			"either revert the change or fail.",
	},
	"missing_in_live": {
		"Resource missing from live infrastructure",
		"Terraform state records a resource that no longer exists. It was deleted " +
			"outside Terraform, and the next apply will try to recreate it.",
	},
	"unmanaged": {
		"Unmanaged resource",
		"A resource exists in the live environment that Terraform does not track. " +
			"It is not covered by review, policy, or the plan output, and nothing will " +
			"recreate it if it is lost.",
	},
}

// driftSarifFindings converts a report into SARIF findings.
func driftSarifFindings(report driftReport) []ui.SarifFinding {
	// A non-nil empty slice matters: it is how a clean scan produces a valid
	// SARIF document that tells GitHub the previous findings are fixed, rather
	// than an error saying this command has no SARIF form.
	findings := make([]ui.SarifFinding, 0, len(report.Findings))

	for _, f := range report.Findings {
		rule, known := driftSarifRules[f.Kind]
		if !known {
			rule = [2]string{"Infrastructure drift", "Live infrastructure disagrees with Terraform state."}
		}

		message := fmt.Sprintf("%s: %s", rule[0], f.Address)
		if len(f.Changes) > 0 {
			paths := make([]string, 0, len(f.Changes))
			for _, change := range f.Changes {
				paths = append(paths, change.Path)
			}
			message = fmt.Sprintf("%s changed outside Terraform: %s", f.Address, strings.Join(paths, ", "))
		}

		findings = append(findings, ui.SarifFinding{
			RuleID:          "drift-" + strings.ReplaceAll(f.Kind, "_", "-"),
			RuleName:        rule[0],
			RuleDescription: rule[1],
			Level:           ui.SeverityToSarifLevel(string(f.Severity)),
			Message:         message,
			File:            report.StateFile,
			// The address and kind identify the same finding across runs, which
			// is what lets GitHub mark one fixed rather than merely absent. The
			// changed attributes are deliberately excluded: a finding whose diff
			// grows is still the same finding.
			Fingerprint: fmt.Sprintf("%s:%s", f.Kind, f.Address),
			Properties: map[string]any{
				"severity":      string(f.Severity),
				"score":         f.Score,
				"resource_type": f.Type,
				"kind":          f.Kind,
			},
		})
	}

	return findings
}

// printDriftSummary writes the post-table summary and, when requested, the
// property-level diff for each finding.
func printDriftSummary(report driftReport) {
	if driftShowDiff {
		for _, f := range report.Findings {
			if len(f.Changes) == 0 {
				continue
			}
			rt.UI.Println()
			rt.UI.Println(rt.UI.Apply(ui.StyleBold, f.Address) + " " +
				rt.UI.Apply(ui.SeverityStyle(string(f.Severity)), "["+string(f.Severity)+"]"))

			lines := make([]ui.DiffLine, len(f.Changes))
			for i, c := range f.Changes {
				lines[i] = ui.DiffLine{
					Path:      c.Path,
					Expected:  c.Expected,
					Actual:    c.Actual,
					Sensitive: c.Sensitive,
				}
			}
			rt.UI.Raw(rt.UI.RenderDiff(lines))
		}
	}

	if len(report.Findings) == 0 {
		return
	}

	parts := []string{}
	for _, band := range []struct {
		name  string
		style ui.Style
	}{
		{"critical", ui.StyleCritical},
		{"high", ui.StyleHigh},
		{"medium", ui.StyleMedium},
		{"low", ui.StyleLow},
		{"info", ui.StyleNegligib},
	} {
		if n := report.CountsBySeverit[band.name]; n > 0 {
			parts = append(parts, rt.UI.Count(band.name, n, band.style))
		}
	}

	rt.UI.Println()
	rt.UI.Printf("%d finding(s) across %d managed resources: %s\n",
		len(report.Findings), report.StateResources, strings.Join(parts, ", "))

	// Suppression is never silent. If rules hid something, the count is stated
	// along with how to see what was hidden.
	if report.Suppressed > 0 {
		rt.UI.Printf("%s\n", rt.UI.Apply(ui.StyleMuted, fmt.Sprintf(
			"%d suppressed by ignore rules; re-run with --no-ignore to see them.",
			report.Suppressed)))
	}
}

func init() {
	rootCmd.AddCommand(driftCmd)
	driftCmd.AddCommand(driftScanCmd)

	f := driftScanCmd.Flags()
	f.StringVar(&driftStatePath, "state", "", "path to the Terraform or OpenTofu state file (required)")
	f.StringVar(&driftLivePath, "live", "", "path to the live infrastructure snapshot JSON (required)")
	f.StringVar(&driftMinSeverit, "min-severity", "info", "only report findings at or above this severity")
	f.StringVar(&driftFailOn, "fail-on", "none", "exit 3 when a finding at or above this severity is present: none, any, low, medium, high, critical")
	f.BoolVar(&driftShowDiff, "show-diff", false, "print the property-level diff for each finding")
	f.BoolVar(&driftIncludeUnm, "include-unmanaged", true, "report live resources that Terraform does not track")
	f.StringVar(&driftIgnorePath, "ignore-file", "",
		"path to the ignore file (default: nearest .infractl-ignore.yaml, searching upwards)")
	f.BoolVar(&driftNoIgnore, "no-ignore", false,
		"ignore the ignore file and report every finding, for auditing what suppression hides")
	f.BoolVar(&driftFix, "fix", false,
		"print the commands and import blocks that would resolve each finding; runs nothing")
	f.StringVar(&driftEmitImport, "emit-import", "",
		"write Terraform import blocks for unmanaged resources to this file")

	_ = driftScanCmd.MarkFlagRequired("state")
	_ = driftScanCmd.MarkFlagRequired("live")
	_ = driftScanCmd.MarkFlagFilename("state", "tfstate", "json")
	_ = driftScanCmd.MarkFlagFilename("live", "json")

	// Severity values are a closed set, so completing them saves the user a
	// trip to the help text.
	_ = driftScanCmd.RegisterFlagCompletionFunc("min-severity",
		cobra.FixedCompletions([]string{"critical", "high", "medium", "low", "info"}, cobra.ShellCompDirectiveNoFileComp))
	_ = driftScanCmd.RegisterFlagCompletionFunc("fail-on",
		cobra.FixedCompletions([]string{"none", "any", "critical", "high", "medium", "low"}, cobra.ShellCompDirectiveNoFileComp))
}

// loadIgnoreRules resolves and loads the suppression rules for this scan.
//
// With --no-ignore no rules are loaded at all, which is the flag to reach for
// when auditing what suppression is hiding.
func loadIgnoreRules() (*ignore.Ruleset, error) {
	if driftNoIgnore {
		return &ignore.Ruleset{}, nil
	}

	path := driftIgnorePath
	if path == "" {
		// Walk up from the working directory so a scan run in a subdirectory
		// still picks up rules committed at the repository root.
		cwd, err := os.Getwd()
		if err == nil {
			path = ignore.FindDefault(cwd)
		}
	}

	rules, err := ignore.Load(path)
	if err != nil {
		return nil, failf(ExitUsage, "%w", err)
	}
	return rules, nil
}

// applyIgnoreRules splits findings into those to report and those suppressed.
func applyIgnoreRules(findings []driftFinding, rules *ignore.Ruleset) ([]driftFinding, []suppression) {
	if rules.Len() == 0 {
		return findings, nil
	}

	kept := make([]driftFinding, 0, len(findings))
	var suppressed []suppression

	for _, finding := range findings {
		paths := make([]string, 0, len(finding.Changes))
		for _, change := range finding.Changes {
			paths = append(paths, change.Path)
		}

		if rule, matched := rules.Match(finding.Address, paths); matched {
			suppressed = append(suppressed, suppression{finding: finding, rule: rule})
			continue
		}
		kept = append(kept, finding)
	}

	return kept, suppressed
}

// countByRule tallies suppressions per rule, keyed by the rule's description so
// the JSON output names which rule hid what.
func countByRule(suppressed []suppression) map[string]int {
	if len(suppressed) == 0 {
		return nil
	}
	counts := make(map[string]int, len(suppressed))
	for _, s := range suppressed {
		counts[s.rule.Describe()]++
	}
	return counts
}

// computeCoverage measures how much of the observed estate Terraform manages.
func computeCoverage(managed []terraform.StateResource, snapshot *liveSnapshot, countedUnmanaged bool) coverageReport {
	inState := make(map[string]struct{}, len(managed))
	for _, resource := range managed {
		inState[resource.Address] = struct{}{}
	}

	unmanaged := 0
	for address := range snapshot.Resources {
		if _, tracked := inState[address]; !tracked {
			unmanaged++
		}
	}

	report := coverageReport{
		Managed:   len(managed),
		Unmanaged: unmanaged,
		Total:     len(managed) + unmanaged,
		// A snapshot built from a refresh-only plan contains only managed
		// resources by construction, so a coverage figure from it would always
		// read 100% and mean nothing.
		Partial: !countedUnmanaged || unmanaged == 0,
	}
	if report.Total > 0 {
		report.Percent = float64(report.Managed) / float64(report.Total) * 100
	}
	return report
}

// collectDrift runs the comparison and returns the raw inputs and findings.
//
// It exists so that `notify` reports on exactly what `drift scan` would find.
// Two code paths producing two answers from the same files is the kind of
// divergence nobody notices until an alert disagrees with a scan.
//
// Ignore rules are applied here, because a suppressed finding should not
// become a notification either.
func collectDrift() (*terraform.State, *liveSnapshot, []driftFinding, error) {
	state, err := terraform.ParseStateFile(driftStatePath)
	if err != nil {
		return nil, nil, nil, failf(ExitError, "%w", err)
	}

	snapshot, err := readLiveSnapshot(driftLivePath)
	if err != nil {
		return nil, nil, nil, failf(ExitError, "%w", err)
	}

	findings := compareForDrift(state.ManagedResources(), snapshot, driftIncludeUnm)

	rules, err := loadIgnoreRules()
	if err != nil {
		return nil, nil, nil, err
	}
	findings, _ = applyIgnoreRules(findings, rules)

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Score != findings[j].Score {
			return findings[i].Score > findings[j].Score
		}
		return findings[i].Address < findings[j].Address
	})

	return state, snapshot, findings, nil
}
