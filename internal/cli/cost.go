//go:build preview

// This command has no working implementation. It is excluded from default
// builds so that `infractl --help` lists only what the tool can actually do.
//
// A CLI whose help text advertises commands that return "not configured"
// invites the reader to judge the whole tool a scaffold, which is unfair to
// the commands that work. Build with `-tags preview` to see it.

package cli

import (
	"github.com/spf13/cobra"
)

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Infrastructure cost analysis, optimization, and forecasting",
	Long: `Analyze infrastructure costs, detect anomalies, forecast spending,
and generate optimization recommendations.

Examples:
  # Show current cost breakdown by provider
  infractl cost breakdown

  # Show cost by AWS account
  infractl cost breakdown --aws-account 123456789012

  # Show cost by GCP project
  infractl cost breakdown --gcp-project my-project

  # Show cost breakdown by resource type
  infractl cost breakdown --group-by type

  # Estimate cost impact of a Terraform plan
  infractl cost estimate --plan plan.json

  # Detect cost anomalies
  infractl cost anomalies

  # Show cost optimization recommendations
  infractl cost optimize

  # Show cost forecast for next 30 days
  infractl cost forecast --period 30d

  # Show cost trends
  infractl cost trends --period 90d`,
}

var costBreakdownCmd = &cobra.Command{
	Use:   "breakdown",
	Short: "Show cost breakdown by provider, account, type, or tag",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl cost")
	},
}

var costEstimateCmd = &cobra.Command{
	Use:   "estimate",
	Short: "Estimate cost impact of a Terraform plan",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl cost")
	},
}

var costAnomaliesCmd = &cobra.Command{
	Use:   "anomalies",
	Short: "Detect cost anomalies and unexpected spending",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl cost")
	},
}

var costOptimizeCmd = &cobra.Command{
	Use:     "optimize",
	Aliases: []string{"recommendations", "savings"},
	Short:   "Show cost optimization recommendations",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl cost")
	},
}

var costForecastCmd = &cobra.Command{
	Use:   "forecast",
	Short: "Forecast infrastructure spending",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl cost")
	},
}

var costTrendsCmd = &cobra.Command{
	Use:   "trends",
	Short: "Show cost trends over time",
	RunE: func(_ *cobra.Command, _ []string) error {
		return requiresBackend("infractl cost")
	},
}

func init() {
	rootCmd.AddCommand(costCmd)
	costCmd.AddCommand(costBreakdownCmd)
	costCmd.AddCommand(costEstimateCmd)
	costCmd.AddCommand(costAnomaliesCmd)
	costCmd.AddCommand(costOptimizeCmd)
	costCmd.AddCommand(costForecastCmd)
	costCmd.AddCommand(costTrendsCmd)

	costBreakdownCmd.Flags().String("group-by", "provider", "group by: provider, account, type, region, tag")
	costBreakdownCmd.Flags().String("period", "30d", "time period: 7d, 30d, 90d, 1y")
	costEstimateCmd.Flags().String("plan", "", "path to Terraform plan JSON file")
	costForecastCmd.Flags().String("period", "30d", "forecast period: 7d, 30d, 90d, 6m, 1y")
	costTrendsCmd.Flags().String("period", "90d", "time period: 7d, 30d, 90d, 1y")
}
