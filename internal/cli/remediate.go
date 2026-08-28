package cli

import (
	"github.com/spf13/cobra"
)

var remediateCmd = &cobra.Command{
	Use:     "remediate",
	Aliases: []string{"fix", "rem"},
	Short:   "AI-powered infrastructure remediation",
	Long: `Use AI to investigate, plan, and execute infrastructure remediations.
Supports automatic Terraform code generation, pull request creation,
approval workflows, and post-apply verification.

Examples:
  # Investigate a drift event with AI
  infractl remediate investigate drift-12345

  # Generate a remediation plan
  infractl remediate plan drift-12345

  # Apply a remediation (requires approval for high-risk)
  infractl remediate apply rem-67890

  # List pending remediations
  infractl remediate list --status pending

  # Rollback a failed remediation
  infractl remediate rollback rem-67890`,
}

var remediateInvestigateCmd = &cobra.Command{
	Use:   "investigate <drift-event-id>",
	Short: "AI-powered root cause investigation",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl remediate")
	},
}

var remediatePlanCmd = &cobra.Command{
	Use:   "plan <drift-event-id>",
	Short: "Generate an AI-powered remediation plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl remediate")
	},
}

var remediateApplyCmd = &cobra.Command{
	Use:   "apply <remediation-id>",
	Short: "Execute an approved remediation plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl remediate")
	},
}

var remediateListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List remediation plans",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl remediate")
	},
}

var remediateApproveCmd = &cobra.Command{
	Use:   "approve <remediation-id>",
	Short: "Approve a pending remediation plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl remediate")
	},
}

var remediateRejectCmd = &cobra.Command{
	Use:   "reject <remediation-id>",
	Short: "Reject a pending remediation plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl remediate")
	},
}

var remediateRollbackCmd = &cobra.Command{
	Use:   "rollback <remediation-id>",
	Short: "Rollback a completed or failed remediation",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl remediate")
	},
}

var remediateShowCmd = &cobra.Command{
	Use:   "show <remediation-id>",
	Short: "Show remediation details including Terraform code diff",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl remediate")
	},
}

func init() {
	rootCmd.AddCommand(remediateCmd)
	remediateCmd.AddCommand(remediateInvestigateCmd)
	remediateCmd.AddCommand(remediatePlanCmd)
	remediateCmd.AddCommand(remediateApplyCmd)
	remediateCmd.AddCommand(remediateListCmd)
	remediateCmd.AddCommand(remediateApproveCmd)
	remediateCmd.AddCommand(remediateRejectCmd)
	remediateCmd.AddCommand(remediateRollbackCmd)
	remediateCmd.AddCommand(remediateShowCmd)

	remediateListCmd.Flags().String("status", "", "filter by status: proposed, validating, pending, approved, rejected, executing, completed, failed, rolled-back")
	remediateApplyCmd.Flags().Bool("auto-approve", false, "skip approval confirmation (dangerous)")
	remediateApplyCmd.Flags().Bool("dry-run", false, "simulate execution without applying changes")
}
