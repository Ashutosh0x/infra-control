package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ashutosh0x/infra-control/internal/risk"
	"github.com/ashutosh0x/infra-control/internal/terraform"
	"github.com/ashutosh0x/infra-control/internal/ui"
	"github.com/ashutosh0x/infra-control/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	riskStatePath   string
	riskMinLevel    string
	riskFailOn      string
	riskShowFactors bool
	riskTopN        int
)

var riskCmd = &cobra.Command{
	Use:     "risk",
	Short:   "Score infrastructure risk across four dimensions",
	GroupID: "analyse",
	Long: `Score resources for security, reliability, cost, and compliance risk, and
combine those into one weighted number.

The four dimensions are scored independently and then weighted, so a resource
that is cheap and reliable but publicly readable still surfaces. The default
weights favour security and reliability:

  security     0.35    public exposure, missing encryption, unmanaged resources
  reliability  0.30    single-AZ deployment, absent backups
  compliance   0.20    missing ownership and cost-attribution tags
  cost         0.15    instance sizes that suggest overprovisioning`,
}

var riskAssessCmd = &cobra.Command{
	Use:   "assess",
	Short: "Score every resource in a state file",
	Long: `Read a Terraform or OpenTofu state file, score each managed resource, and
report the results worst-first.

Scoring is based on what state records. A resource whose risk depends on
something outside state, such as an attached IAM policy defined elsewhere, is
scored only on the attributes present.`,
	Example: `  # Score everything
  infractl risk assess --state terraform.tfstate

  # Show only the resources at high risk or above
  infractl risk assess --state terraform.tfstate --min-level high

  # Show the ten worst, with the reasons
  infractl risk assess --state terraform.tfstate --top 10 --show-factors

  # Fail CI on any critical finding
  infractl risk assess --state terraform.tfstate --fail-on critical`,
	Args: cobra.NoArgs,
	RunE: runRiskAssess,
}

// riskResult pairs a resource with its score.
type riskResult struct {
	Address     string             `json:"address"`
	Type        string             `json:"resource_type"`
	Level       types.RiskLevel    `json:"level"`
	Overall     float64            `json:"overall"`
	Security    float64            `json:"security"`
	Reliability float64            `json:"reliability"`
	Cost        float64            `json:"cost"`
	Compliance  float64            `json:"compliance"`
	Factors     []types.RiskFactor `json:"factors,omitempty"`
}

// riskReport is the full assessment payload.
type riskReport struct {
	StateFile string         `json:"state_file"`
	Assessed  int            `json:"assessed"`
	Reported  int            `json:"reported"`
	ByLevel   map[string]int `json:"by_level"`
	Results   []riskResult   `json:"results"`
}

func runRiskAssess(cmd *cobra.Command, _ []string) error {
	if err := requireFile(riskStatePath, "state file (--state)"); err != nil {
		return err
	}

	minLevel, err := parseRiskLevel(riskMinLevel, "--min-level")
	if err != nil {
		return err
	}

	var failLevel types.RiskLevel
	failSet := false
	if trimmed := strings.ToLower(strings.TrimSpace(riskFailOn)); trimmed != "" && trimmed != "none" {
		failLevel, err = parseRiskLevel(riskFailOn, "--fail-on")
		if err != nil {
			return err
		}
		failSet = true
	}

	spinner := rt.UI.Spin("Reading Terraform state")
	state, err := terraform.ParseStateFile(riskStatePath)
	if err != nil {
		spinner.Fail("Could not read state")
		return failf(ExitError, "%w", err)
	}
	managed := state.ManagedResources()
	spinner.Stop()

	// The risk engine logs at info level; a CLI run wants that on stderr only in
	// verbose mode, so the logger is wired accordingly.
	logger := zap.NewNop()
	if verbose {
		logger, _ = zap.NewDevelopment()
	}
	engine := risk.NewEngine(risk.DefaultWeights(), logger)

	progress := rt.UI.NewProgress("Scoring", len(managed))
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	var results []riskResult
	for _, stateResource := range managed {
		resource := toTypedResource(stateResource)

		score, err := engine.Assess(ctx, resource)
		if err != nil {
			progress.Done()
			return failf(ExitError, "assess %s: %w", stateResource.Address, err)
		}
		progress.Increment(1)

		results = append(results, riskResult{
			Address:     stateResource.Address,
			Type:        stateResource.Type,
			Level:       score.Level,
			Overall:     score.Overall,
			Security:    score.Security,
			Reliability: score.Reliability,
			Cost:        score.Cost,
			Compliance:  score.Compliance,
			Factors:     score.Factors,
		})
	}
	progress.Done()

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Overall != results[j].Overall {
			return results[i].Overall > results[j].Overall
		}
		return results[i].Address < results[j].Address
	})

	report := riskReport{
		StateFile: riskStatePath,
		Assessed:  len(results),
		ByLevel:   map[string]int{},
	}
	for _, r := range results {
		report.ByLevel[string(r.Level)]++
	}

	// Filter after counting, so the summary describes the whole state file while
	// the table shows only what was asked for.
	filtered := make([]riskResult, 0, len(results))
	for _, r := range results {
		if riskRank(r.Level) >= riskRank(minLevel) {
			filtered = append(filtered, r)
		}
	}
	if riskTopN > 0 && len(filtered) > riskTopN {
		filtered = filtered[:riskTopN]
	}
	report.Results = filtered
	report.Reported = len(filtered)

	if err := rt.write(riskView(report)); err != nil {
		return err
	}

	if !rt.Format.IsMachine() {
		printRiskSummary(report)
	}

	if failSet {
		for _, r := range results {
			if riskRank(r.Level) >= riskRank(failLevel) {
				rt.UI.Failure("Found %s risk at or above the --fail-on threshold of %s.", r.Level, failLevel)
				return findingsExit(1, true)
			}
		}
	}
	return nil
}

// toTypedResource converts a Terraform state resource into the shape the risk
// engine scores, deriving the metadata flags the engine reads from attributes.
func toTypedResource(sr terraform.StateResource) *types.Resource {
	resource := &types.Resource{
		ID:            sr.Address,
		Name:          sr.Name,
		Type:          sr.Type,
		Provider:      types.CloudProvider(sr.Provider),
		Configuration: reliabilityConfig(sr),
		Tags:          extractTags(sr.Attributes),
		Metadata: types.ResourceMetadata{
			// Everything read from a state file is by definition IaC managed.
			ManagedBy: "terraform",
			IsPublic:  inferPublic(sr.Attributes),
			// A resource type with no encryption-at-rest concept is reported as
			// encrypted, so that the security check does not raise a finding
			// nobody can act on. See applicability.go.
			IsEncrypted: !supportsEncryption(sr.Type) || inferEncrypted(sr.Attributes),
		},
	}
	return resource
}

// extractTags pulls the tag or label map out of resource attributes. Providers
// disagree on the key, so all the common spellings are checked.
func extractTags(attrs map[string]any) map[string]string {
	tags := map[string]string{}
	for _, key := range []string{"tags", "labels", "resource_tags"} {
		raw, ok := attrs[key].(map[string]any)
		if !ok {
			continue
		}
		for k, v := range raw {
			if s, ok := v.(string); ok {
				tags[strings.ToLower(k)] = s
			}
		}
	}
	return tags
}

// inferPublic reports whether attributes indicate the resource is reachable
// from the public internet.
func inferPublic(attrs map[string]any) bool {
	for key, value := range attrs {
		lower := strings.ToLower(key)

		if strings.Contains(lower, "publicly_accessible") ||
			strings.Contains(lower, "public_access") ||
			lower == "public" {
			if b, ok := value.(bool); ok && b {
				return true
			}
		}

		// A canned ACL naming public read or write is public regardless of the
		// other settings.
		if lower == "acl" {
			if s, ok := value.(string); ok && strings.HasPrefix(s, "public-") {
				return true
			}
		}

		// A security group open to the world.
		if lower == "cidr_blocks" || lower == "source_ranges" {
			if list, ok := value.([]any); ok {
				for _, entry := range list {
					if s, ok := entry.(string); ok && s == "0.0.0.0/0" {
						return true
					}
				}
			}
		}
	}
	return false
}

// inferEncrypted reports whether attributes indicate encryption at rest.
//
// Absence of any encryption attribute is treated as unencrypted. That is the
// conservative reading: a resource type with no encryption setting either does
// not support it or defaults to off, and both are worth surfacing.
func inferEncrypted(attrs map[string]any) bool {
	for key, value := range attrs {
		lower := strings.ToLower(key)
		if !strings.Contains(lower, "encrypt") {
			continue
		}
		switch v := value.(type) {
		case bool:
			if v {
				return true
			}
		case string:
			if v != "" && v != "false" && v != "NONE" {
				return true
			}
		case map[string]any, []any:
			// A populated encryption configuration block counts as enabled.
			return true
		}
	}

	// A KMS key reference implies encryption even when no boolean is recorded.
	for key, value := range attrs {
		if strings.Contains(strings.ToLower(key), "kms_key") {
			if s, ok := value.(string); ok && s != "" {
				return true
			}
		}
	}
	return false
}

// riskRank orders risk levels for threshold comparisons.
func riskRank(level types.RiskLevel) int {
	switch level {
	case types.RiskLevelCritical:
		return 5
	case types.RiskLevelHigh:
		return 4
	case types.RiskLevelMedium:
		return 3
	case types.RiskLevelLow:
		return 2
	case types.RiskLevelNegligible:
		return 1
	default:
		return 0
	}
}

// parseRiskLevel resolves a risk level name.
func parseRiskLevel(value, flag string) (types.RiskLevel, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "negligible", "all":
		return types.RiskLevelNegligible, nil
	case "low":
		return types.RiskLevelLow, nil
	case "medium":
		return types.RiskLevelMedium, nil
	case "high":
		return types.RiskLevelHigh, nil
	case "critical":
		return types.RiskLevelCritical, nil
	default:
		return "", failf(ExitUsage,
			"invalid %s %q (want critical, high, medium, low, or negligible)", flag, value)
	}
}

// riskView builds the table and machine payload.
func riskView(report riskReport) ui.View {
	table := ui.NewTable(
		ui.Column{Title: "LEVEL", MinWidth: 10},
		ui.Column{Title: "SCORE", Align: ui.AlignRight, MinWidth: 5},
		ui.Column{Title: "ADDRESS", MinWidth: 24, Truncatable: true},
		ui.Column{Title: "SEC", Align: ui.AlignRight, MinWidth: 3},
		ui.Column{Title: "REL", Align: ui.AlignRight, MinWidth: 3},
		ui.Column{Title: "COST", Align: ui.AlignRight, MinWidth: 4},
		ui.Column{Title: "COMP", Align: ui.AlignRight, MinWidth: 4},
	)

	names := make([]string, 0, len(report.Results))
	for _, r := range report.Results {
		table.Row(
			ui.Styled(strings.ToUpper(string(r.Level)), ui.SeverityStyle(string(r.Level))),
			ui.Text(fmt.Sprintf("%.0f", r.Overall)),
			ui.Text(r.Address),
			ui.Text(fmt.Sprintf("%.0f", r.Security)),
			ui.Text(fmt.Sprintf("%.0f", r.Reliability)),
			ui.Text(fmt.Sprintf("%.0f", r.Cost)),
			ui.Text(fmt.Sprintf("%.0f", r.Compliance)),
		)
		names = append(names, r.Address)
	}

	return ui.View{
		Data:  report,
		Table: table,
		Names: names,
		Empty: fmt.Sprintf("No resources at or above that risk level. %d assessed.", report.Assessed),
	}
}

// printRiskSummary writes the distribution and, when asked, the per-resource
// factors that produced each score.
func printRiskSummary(report riskReport) {
	if riskShowFactors {
		for _, r := range report.Results {
			if len(r.Factors) == 0 {
				continue
			}
			rt.UI.Println()
			rt.UI.Println(rt.UI.Apply(ui.StyleBold, r.Address) + " " +
				rt.UI.Apply(ui.SeverityStyle(string(r.Level)),
					fmt.Sprintf("[%s %.0f]", r.Level, r.Overall)))

			for _, factor := range r.Factors {
				rt.UI.Printf("  %s %-22s %s\n",
					rt.UI.Apply(ui.StyleMuted, rt.UI.Symbols().Bullet),
					factor.Name,
					rt.UI.Apply(ui.StyleMuted, factor.Description))
			}
		}
	}

	if report.Assessed == 0 {
		return
	}

	rt.UI.Println()
	rt.UI.Println(rt.UI.Apply(ui.StyleBold, "Risk distribution"))

	for _, band := range []struct {
		level types.RiskLevel
		style ui.Style
	}{
		{types.RiskLevelCritical, ui.StyleCritical},
		{types.RiskLevelHigh, ui.StyleHigh},
		{types.RiskLevelMedium, ui.StyleMedium},
		{types.RiskLevelLow, ui.StyleLow},
		{types.RiskLevelNegligible, ui.StyleNegligib},
	} {
		count := report.ByLevel[string(band.level)]
		bar := rt.UI.Bar(float64(count), float64(report.Assessed), 24, band.style)
		rt.UI.Printf("  %-12s %s %d\n", band.level, bar, count)
	}
}

func init() {
	rootCmd.AddCommand(riskCmd)
	riskCmd.AddCommand(riskAssessCmd)

	f := riskAssessCmd.Flags()
	f.StringVar(&riskStatePath, "state", "", "path to the Terraform or OpenTofu state file (required)")
	f.StringVar(&riskMinLevel, "min-level", "negligible", "only report resources at or above this risk level")
	f.StringVar(&riskFailOn, "fail-on", "none", "exit 3 when a resource at or above this level is found")
	f.BoolVar(&riskShowFactors, "show-factors", false, "print the factors behind each score")
	f.IntVar(&riskTopN, "top", 0, "show only the N highest-scoring resources (0 shows all)")

	_ = riskAssessCmd.MarkFlagRequired("state")
	_ = riskAssessCmd.MarkFlagFilename("state", "tfstate", "json")

	levels := []string{"critical", "high", "medium", "low", "negligible"}
	_ = riskAssessCmd.RegisterFlagCompletionFunc("min-level",
		cobra.FixedCompletions(levels, cobra.ShellCompDirectiveNoFileComp))
	_ = riskAssessCmd.RegisterFlagCompletionFunc("fail-on",
		cobra.FixedCompletions(append([]string{"none"}, levels...), cobra.ShellCompDirectiveNoFileComp))
}

// reliabilityConfig returns the attribute map the risk engine scores, with the
// keys that drive inapplicable reliability checks removed.
//
// The engine treats the presence of an availability_zone attribute without a
// multi_az attribute as a single-AZ deployment. That reading is correct for a
// database but wrong for a subnet, which exists in exactly one zone by
// definition and cannot be made multi-AZ.
func reliabilityConfig(sr terraform.StateResource) map[string]any {
	if supportsMultiAZ(sr.Type) && supportsBackup(sr.Type) {
		return sr.Attributes
	}

	config := make(map[string]any, len(sr.Attributes))
	for key, value := range sr.Attributes {
		if key == "availability_zone" && !supportsMultiAZ(sr.Type) {
			continue
		}
		if key == "backup_retention_period" && !supportsBackup(sr.Type) {
			continue
		}
		config[key] = value
	}
	return config
}
