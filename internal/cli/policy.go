package cli

import (
	"github.com/spf13/cobra"
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Policy-as-code management, evaluation, and generation",
	Long: `Manage infrastructure policies using OPA/Rego. Supports built-in
compliance policies, custom rules, Terraform plan validation, and
AI-powered natural language to Rego policy generation.

Enforcement Levels:
  advisory         Log violations, do not block
  soft-mandatory    Block unless override is provided
  hard-mandatory    Block unconditionally

Examples:
  infractl policy list
  infractl policy evaluate --provider aws --aws-account 123456789012
  infractl policy test ./policies/
  infractl policy generate "All RDS instances must have encryption enabled"`,
}

var policyListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all registered policies",
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl policy")
	},
}

var policyEvaluateCmd = &cobra.Command{
	Use:     "evaluate",
	Aliases: []string{"eval", "check"},
	Short:   "Evaluate policies against current infrastructure state",
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl policy")
	},
}

var policyTestCmd = &cobra.Command{
	Use:   "test <path>",
	Short: "Test policy files with fixtures",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl policy")
	},
}

var policyAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Add a custom policy from a Rego file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl policy")
	},
}

var policyShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show policy details including Rego source code",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl policy")
	},
}

var policyDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a custom policy",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl policy")
	},
}

var policyEnableCmd = &cobra.Command{
	Use:   "enable <id>",
	Short: "Enable a policy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl policy")
	},
}

var policyDisableCmd = &cobra.Command{
	Use:   "disable <id>",
	Short: "Disable a policy without deleting it",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl policy")
	},
}

var policyGenerateCmd = &cobra.Command{
	Use:   "generate <description>",
	Short: "Generate a Rego policy from natural language using AI",
	Long: `Use AI to convert a natural language policy description into
valid OPA Rego code. The generated policy includes proper package
declaration, rule definitions, and violation messages.

Examples:
  infractl policy generate "All S3 buckets must have encryption enabled"
  infractl policy generate "No security groups should allow ingress from 0.0.0.0/0"
  infractl policy generate "All RDS instances must have multi-AZ enabled in production"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl policy")
	},
}

var policyViolationsCmd = &cobra.Command{
	Use:   "violations",
	Short: "List current policy violations across infrastructure",
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl policy")
	},
}

func init() {
	rootCmd.AddCommand(policyCmd)
	policyCmd.AddCommand(policyListCmd)
	policyCmd.AddCommand(policyEvaluateCmd)
	policyCmd.AddCommand(policyTestCmd)
	policyCmd.AddCommand(policyAddCmd)
	policyCmd.AddCommand(policyShowCmd)
	policyCmd.AddCommand(policyDeleteCmd)
	policyCmd.AddCommand(policyEnableCmd)
	policyCmd.AddCommand(policyDisableCmd)
	policyCmd.AddCommand(policyGenerateCmd)
	policyCmd.AddCommand(policyViolationsCmd)

	policyListCmd.Flags().String("type", "", "filter by type: security, cost, reliability, compliance, custom")
	policyListCmd.Flags().String("severity", "", "filter by severity: error, warning, info")
	policyListCmd.Flags().String("provider", "", "filter by provider scope")
	policyListCmd.Flags().String("enforcement", "", "filter by enforcement: advisory, soft-mandatory, hard-mandatory")
	policyEvaluateCmd.Flags().String("provider", "", "evaluate against specific provider")
	policyEvaluateCmd.Flags().String("type", "", "evaluate specific policy types")
	policyEvaluateCmd.Flags().String("plan", "", "evaluate against a Terraform plan JSON file")
	policyViolationsCmd.Flags().String("severity", "", "filter by violation severity")
	policyViolationsCmd.Flags().String("provider", "", "filter by provider")
	policyGenerateCmd.Flags().String("save", "", "save generated policy to file")
}
