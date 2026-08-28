package cli

import (
	"github.com/spf13/cobra"
)

var complianceCmd = &cobra.Command{
	Use:     "compliance",
	Aliases: []string{"comply"},
	Short:   "Compliance framework assessment and reporting",
	Long: `Evaluate infrastructure against compliance frameworks including
SOC 2 Type II, ISO 27001, PCI DSS, HIPAA, NIST 800-53, and CIS Benchmarks.

Supported Frameworks:
  soc2        SOC 2 Type II controls
  iso27001    ISO 27001:2022 Annex A controls
  pci         PCI DSS v4.0 requirements
  hipaa       HIPAA Security Rule
  nist        NIST 800-53 Rev 5 controls
  cis-aws     CIS Amazon Web Services Foundations Benchmark
  cis-gcp     CIS Google Cloud Platform Foundations Benchmark
  cis-azure   CIS Microsoft Azure Foundations Benchmark
  cis-k8s     CIS Kubernetes Benchmark

Examples:
  # Run all compliance checks
  infractl compliance run --all

  # Evaluate against a specific framework
  infractl compliance run --framework cis-aws

  # Run CIS benchmarks for a specific AWS account
  infractl compliance run --framework cis-aws --aws-account 123456789012

  # Generate a compliance report in PDF
  infractl compliance report --framework soc2 --format pdf

  # Show compliance status summary
  infractl compliance status

  # List all available controls for a framework
  infractl compliance controls --framework pci

  # Export evidence for auditors
  infractl compliance evidence --framework soc2 --since 2026-01-01`,
}

var complianceRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute compliance checks against infrastructure",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl compliance")
	},
}

var complianceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show compliance posture summary across all frameworks",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl compliance")
	},
}

var complianceReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate compliance report (HTML, PDF, JSON, CSV)",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl compliance")
	},
}

var complianceControlsCmd = &cobra.Command{
	Use:   "controls",
	Short: "List all controls for a compliance framework",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl compliance")
	},
}

var complianceEvidenceCmd = &cobra.Command{
	Use:   "evidence",
	Short: "Export compliance evidence for auditors",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl compliance")
	},
}

func init() {
	rootCmd.AddCommand(complianceCmd)
	complianceCmd.AddCommand(complianceRunCmd)
	complianceCmd.AddCommand(complianceStatusCmd)
	complianceCmd.AddCommand(complianceReportCmd)
	complianceCmd.AddCommand(complianceControlsCmd)
	complianceCmd.AddCommand(complianceEvidenceCmd)

	complianceRunCmd.Flags().String("framework", "", "compliance framework: soc2, iso27001, pci, hipaa, nist, cis-aws, cis-gcp, cis-azure, cis-k8s")
	complianceRunCmd.Flags().Bool("all", false, "run all configured compliance frameworks")
	complianceReportCmd.Flags().String("framework", "", "compliance framework to report on")
	complianceReportCmd.Flags().String("format", "html", "report format: html, pdf, json, csv, markdown")
	complianceReportCmd.Flags().String("output-file", "", "output file path (default: stdout)")
	complianceControlsCmd.Flags().String("framework", "", "compliance framework")
	complianceEvidenceCmd.Flags().String("framework", "", "compliance framework")
	complianceEvidenceCmd.Flags().String("since", "", "start date for evidence collection (YYYY-MM-DD)")
	complianceEvidenceCmd.Flags().String("until", "", "end date for evidence collection (YYYY-MM-DD)")
}
