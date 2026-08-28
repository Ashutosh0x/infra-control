package cli

import (
	"github.com/spf13/cobra"
)

var securityCmd = &cobra.Command{
	Use:     "security",
	Aliases: []string{"sec"},
	Short:   "Security scanning, posture assessment, and vulnerability detection",
	Long: `Comprehensive security scanning across IAM, network, encryption,
secrets exposure, and cloud misconfigurations.

Scan Categories:
  iam         Overprivileged roles, unused permissions, policy analysis
  network     Open security groups, public endpoints, VPC flow analysis
  encryption  Unencrypted storage, weak KMS keys, missing TLS
  secrets     Exposed credentials, hardcoded API keys, secret rotation
  posture     Overall security posture score and trending

Examples:
  # Full security scan across all providers
  infractl security scan --all

  # Scan IAM for a specific AWS account
  infractl security scan --category iam --aws-account 123456789012

  # Scan network exposure in GCP project
  infractl security scan --category network --gcp-project my-project

  # Show security posture score
  infractl security posture

  # List active security findings
  infractl security findings --severity critical,high

  # Show security trends over time
  infractl security trends --period 30d`,
}

var securityScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Run security scan across infrastructure",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl security")
	},
}

var securityPostureCmd = &cobra.Command{
	Use:   "posture",
	Short: "Show overall security posture score",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl security")
	},
}

var securityFindingsCmd = &cobra.Command{
	Use:   "findings",
	Short: "List security findings and misconfigurations",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl security")
	},
}

var securityTrendsCmd = &cobra.Command{
	Use:   "trends",
	Short: "Show security posture trends over time",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl security")
	},
}

var securityBenchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run CIS benchmarks against infrastructure",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl security")
	},
}

func init() {
	rootCmd.AddCommand(securityCmd)
	securityCmd.AddCommand(securityScanCmd)
	securityCmd.AddCommand(securityPostureCmd)
	securityCmd.AddCommand(securityFindingsCmd)
	securityCmd.AddCommand(securityTrendsCmd)
	securityCmd.AddCommand(securityBenchmarkCmd)

	securityScanCmd.Flags().String("category", "", "scan category: iam, network, encryption, secrets, posture (comma-separated)")
	securityScanCmd.Flags().Bool("all", false, "scan all categories")
	securityFindingsCmd.Flags().String("severity", "", "filter by severity: critical, high, medium, low")
	securityFindingsCmd.Flags().String("category", "", "filter by category")
	securityTrendsCmd.Flags().String("period", "30d", "time period: 7d, 30d, 90d, 1y")
}
