package cli

import (
	"github.com/spf13/cobra"
)

var (
	auditAction string
	auditActor  string
	auditSince  string
	auditUntil  string
	auditFormat string
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Manage audit events",
	Long:  `List, show details, and export audit events.`,
}

var auditListCmd = &cobra.Command{
	Use:   "list",
	Short: "List audit events",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl audit")
	},
}

var auditShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show audit event details",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl audit")
	},
}

var auditExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export audit trail",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl audit")
	},
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.AddCommand(auditListCmd)
	auditCmd.AddCommand(auditShowCmd)
	auditCmd.AddCommand(auditExportCmd)

	auditListCmd.Flags().StringVar(&auditAction, "action", "", "filter by action")
	auditListCmd.Flags().StringVar(&auditActor, "actor", "", "filter by actor")
	auditListCmd.Flags().StringVar(&auditSince, "since", "", "filter by time since")
	auditListCmd.Flags().StringVar(&auditUntil, "until", "", "filter by time until")

	auditExportCmd.Flags().StringVar(&auditFormat, "format", "csv", "export format (csv, json)")
}
