package cli

import (
	"github.com/spf13/cobra"
)

// contextCmd manages cloud provider contexts (like kubectl config use-context).
var contextCmd = &cobra.Command{
	Use:     "context",
	Aliases: []string{"ctx"},
	Short:   "Manage cloud provider contexts and active targets",
	Long: `Manage the active cloud provider context including AWS accounts,
GCP projects, Azure subscriptions, and Kubernetes clusters.

Contexts define which cloud accounts and regions infractl targets.
Switch between accounts without re-authenticating.

Examples:
  # Show current active context
  infractl context current

  # List all configured contexts
  infractl context list

  # Switch to a specific context
  infractl context use production-aws

  # Create a new context
  infractl context create staging-gcp \
    --gcp-project my-staging-project \
    --gcp-region us-central1

  # Delete a context
  infractl context delete old-context`,
}

var contextCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the current active context",
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Cloud context management", "docs/ROADMAP.md#contexts")
	},
}

var contextListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all configured contexts",
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Cloud context management", "docs/ROADMAP.md#contexts")
	},
}

var contextUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch to a named context",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Cloud context management", "docs/ROADMAP.md#contexts")
	},
}

var contextCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new named context",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Cloud context management", "docs/ROADMAP.md#contexts")
	},
}

var contextDeleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"rm"},
	Short:   "Delete a named context",
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Cloud context management", "docs/ROADMAP.md#contexts")
	},
}

func init() {
	rootCmd.AddCommand(contextCmd)
	contextCmd.AddCommand(contextCurrentCmd)
	contextCmd.AddCommand(contextListCmd)
	contextCmd.AddCommand(contextUseCmd)
	contextCmd.AddCommand(contextCreateCmd)
	contextCmd.AddCommand(contextDeleteCmd)
}
