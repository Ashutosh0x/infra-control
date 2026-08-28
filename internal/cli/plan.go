package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ashutosh0x/infra-control/internal/terraform"
	"github.com/ashutosh0x/infra-control/internal/ui"
	"github.com/spf13/cobra"
)

var (
	planFailOn      string
	planShowDetails bool
	planMaxDeletes  int
)

var planCmd = &cobra.Command{
	Use:     "plan",
	Short:   "Analyse Terraform plans before they are applied",
	GroupID: "analyse",
	Long: `Analyse a Terraform or OpenTofu plan and report what it would do to live
infrastructure, with attention to the changes that are hard to undo.

Plans are read from the JSON produced by:

  terraform plan -out=tfplan
  terraform show -json tfplan > plan.json

The binary plan file cannot be read directly; it has no public format.`,
}

// planReport is the result of analysing a plan.
type planReport struct {
	Path         string           `json:"path"`
	TotalChanges int              `json:"total_changes"`
	Creates      int              `json:"creates"`
	Updates      int              `json:"updates"`
	Deletes      int              `json:"deletes"`
	Replaces     int              `json:"replaces"`
	Destructive  int              `json:"destructive"`
	BlastRadius  int              `json:"blast_radius"`
	Warnings     []string         `json:"warnings,omitempty"`
	Changes      []planChangeItem `json:"changes"`
}

// planChangeItem is one resource change in the report.
type planChangeItem struct {
	Address     string `json:"address"`
	Type        string `json:"resource_type"`
	Action      string `json:"action"`
	Provider    string `json:"provider"`
	Destructive bool   `json:"destructive"`
}

var planAnalyseCmd = &cobra.Command{
	Use:     "analyse <plan.json>",
	Aliases: []string{"analyze"},
	Short:   "Report what a plan would change, and what it would destroy",
	Long: `Report the change set in a plan, separating the changes that add or modify
infrastructure from the ones that destroy it.

A replacement is counted as destructive. Terraform encodes a replacement as a
delete followed by a create, and the delete is real: the live resource and
anything depending on it goes away, however briefly.

Use --fail-on in CI to reject a plan automatically.`,
	Example: `  # Summarise a plan
  infractl plan analyse plan.json

  # List every change, not just the summary
  infractl plan analyse plan.json --details

  # Fail the build if the plan destroys or replaces anything
  infractl plan analyse plan.json --fail-on destructive

  # Fail if it would delete more than three resources
  infractl plan analyse plan.json --max-deletes 3`,
	Args: cobra.ExactArgs(1),
	RunE: runPlanAnalyse,
}

func runPlanAnalyse(cmd *cobra.Command, args []string) error {
	path := args[0]
	if err := requireFile(path, "plan file"); err != nil {
		return err
	}

	if err := validatePlanFailOn(planFailOn); err != nil {
		return err
	}

	spinner := rt.UI.Spin("Reading plan")
	plan, err := terraform.ParsePlanFile(path)
	if err != nil {
		spinner.Fail("Could not read plan")
		return failf(ExitError, "%w", err)
	}
	spinner.Stop()

	report := planReport{
		Path:         path,
		TotalChanges: plan.TotalChanges,
		Creates:      plan.Creates,
		Updates:      plan.Updates,
		Deletes:      plan.Deletes,
		Replaces:     plan.Replaces,
		Destructive:  plan.Deletes + plan.Replaces,
		// Blast radius is the count of resources whose live state this plan
		// touches. Every change is included, since an update to a security group
		// can be as consequential as a delete.
		BlastRadius: plan.TotalChanges,
	}

	for _, change := range plan.Changes {
		report.Changes = append(report.Changes, planChangeItem{
			Address:     change.Address,
			Type:        change.Type,
			Action:      string(change.Action),
			Provider:    change.Provider,
			Destructive: change.Action.IsDestructive(),
		})
	}
	report.Warnings = planWarnings(plan)

	if err := rt.write(planView(report)); err != nil {
		return err
	}

	if !rt.Format.IsMachine() {
		printPlanSummary(report)
	}

	return planExitStatus(report)
}

// planWarnings flags patterns worth a second look before applying.
func planWarnings(plan *terraform.PlanSummary) []string {
	var warnings []string

	// A plan that only destroys is nearly always a mistake, usually a wrong
	// workspace or a state file that has lost track of its resources.
	if plan.Deletes > 0 && plan.Creates == 0 && plan.Updates == 0 && plan.Replaces == 0 {
		warnings = append(warnings, fmt.Sprintf(
			"This plan only deletes. It removes %d resources and creates none, which usually means "+
				"the wrong workspace is selected or state has lost track of its resources.", plan.Deletes))
	}

	// Replacing a stateful resource destroys its data.
	statefulTypes := []string{"db_instance", "rds_cluster", "sql_database", "elasticache",
		"dynamodb_table", "s3_bucket", "storage_bucket", "persistent_volume", "efs_file_system"}
	for _, change := range plan.Destructive() {
		for _, stateful := range statefulTypes {
			if strings.Contains(change.Type, stateful) {
				warnings = append(warnings, fmt.Sprintf(
					"%s (%s) is a stateful resource being %sd; its data does not survive.",
					change.Address, change.Type, change.Action))
				break
			}
		}
	}

	sort.Strings(warnings)
	return warnings
}

// validatePlanFailOn rejects an unknown threshold before any work is done.
func validatePlanFailOn(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "none", "any", "destructive", "delete":
		return nil
	default:
		return failf(ExitUsage,
			"invalid --fail-on %q (want none, any, destructive, or delete)", value)
	}
}

// planExitStatus applies the configured CI gates to the report.
func planExitStatus(report planReport) error {
	if planMaxDeletes >= 0 && report.Deletes > planMaxDeletes {
		rt.UI.Failure("Plan deletes %d resources, over the --max-deletes limit of %d.",
			report.Deletes, planMaxDeletes)
		return findingsExit(1, true)
	}

	switch strings.ToLower(strings.TrimSpace(planFailOn)) {
	case "any":
		if report.TotalChanges > 0 {
			rt.UI.Failure("Plan contains %d changes and --fail-on=any is set.", report.TotalChanges)
			return findingsExit(1, true)
		}
	case "destructive":
		if report.Destructive > 0 {
			rt.UI.Failure("Plan destroys or replaces %d resources.", report.Destructive)
			return findingsExit(1, true)
		}
	case "delete":
		if report.Deletes > 0 {
			rt.UI.Failure("Plan deletes %d resources.", report.Deletes)
			return findingsExit(1, true)
		}
	}
	return nil
}

// planView builds the table and machine payload.
func planView(report planReport) ui.View {
	table := ui.NewTable(
		ui.Column{Title: "ACTION", MinWidth: 8},
		ui.Column{Title: "ADDRESS", MinWidth: 24, Truncatable: true},
		ui.Column{Title: "TYPE", MinWidth: 14, Truncatable: true},
		ui.Column{Title: "PROVIDER", MinWidth: 8},
	)

	// Without --details the table lists only the destructive changes, since
	// those are what a reviewer must actually look at.
	names := make([]string, 0, len(report.Changes))
	for _, change := range report.Changes {
		names = append(names, change.Address)
		if !planShowDetails && !change.Destructive {
			continue
		}
		table.Row(
			ui.Styled(strings.ToUpper(change.Action), planActionStyle(change.Action)),
			ui.Text(change.Address),
			ui.Text(change.Type),
			ui.Text(change.Provider),
		)
	}

	empty := "This plan makes no changes."
	if report.TotalChanges > 0 {
		empty = fmt.Sprintf(
			"No destructive changes. %d non-destructive changes; pass --details to list them.",
			report.TotalChanges)
	}

	return ui.View{Data: report, Table: table, Names: names, Empty: empty}
}

// planActionStyle colours an action by how hard it is to undo.
func planActionStyle(action string) ui.Style {
	switch action {
	case "create":
		return ui.StyleAdded
	case "update":
		return ui.StyleChanged
	case "delete":
		return ui.StyleRemoved
	case "replace":
		return ui.StyleCritical
	default:
		return ui.StyleNone
	}
}

// printPlanSummary writes the counters and warnings below the table.
func printPlanSummary(report planReport) {
	rt.UI.Println()
	rt.UI.Printf("Plan: %s, %s, %s, %s\n",
		rt.UI.Count("to add", report.Creates, ui.StyleAdded),
		rt.UI.Count("to change", report.Updates, ui.StyleChanged),
		rt.UI.Count("to destroy", report.Deletes, ui.StyleRemoved),
		rt.UI.Count("to replace", report.Replaces, ui.StyleCritical),
	)

	if len(report.Warnings) == 0 {
		return
	}

	rt.UI.Println()
	rt.UI.Raw(rt.UI.Panel("Review before applying", report.Warnings, ui.StyleWarning))
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.AddCommand(planAnalyseCmd)

	f := planAnalyseCmd.Flags()
	f.StringVar(&planFailOn, "fail-on", "none",
		"exit 3 when the plan matches: none, any, destructive, delete")
	f.BoolVar(&planShowDetails, "details", false,
		"list every change, not only the destructive ones")
	f.IntVar(&planMaxDeletes, "max-deletes", -1,
		"exit 3 if the plan deletes more than this many resources (-1 disables the check)")

	_ = planAnalyseCmd.RegisterFlagCompletionFunc("fail-on",
		cobra.FixedCompletions([]string{"none", "any", "destructive", "delete"}, cobra.ShellCompDirectiveNoFileComp))
}
