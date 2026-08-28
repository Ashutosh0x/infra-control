package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ashutosh0x/infra-control/internal/terraform"
	"github.com/ashutosh0x/infra-control/internal/ui"
	"github.com/ashutosh0x/infra-control/pkg/types"
	"github.com/spf13/cobra"
)

var (
	driftStatePath  string
	driftLivePath   string
	driftMinSeverit string
	driftFailOn     string
	driftShowDiff   bool
	driftIncludeUnm bool
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
}

func runDriftScan(_ *cobra.Command, _ []string) error {
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
	spinner.Stop()

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
	}
	if !snapshot.CapturedAt.IsZero() {
		report.SnapshotAge = time.Since(snapshot.CapturedAt).Round(time.Minute).String()
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
		Data:  report,
		Table: table,
		Names: names,
		Empty: fmt.Sprintf("No drift found. %d managed resources match the live snapshot.", report.StateResources),
	}
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
