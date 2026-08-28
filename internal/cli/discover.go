package cli

import (
	"github.com/spf13/cobra"
)

var (
	discoverProvider string
	discoverType     string
	discoverRegion   string
	discoverAccount  string
	discoverTags     string
	discoverManaged  string
	discoverWatch    bool
	discoverSync     bool
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover infrastructure resources across cloud providers",
	Long: `Discover and inventory infrastructure resources from AWS, GCP, Azure,
and Kubernetes. Supports filtering by provider, resource type, region,
account/project/subscription, tags, and management status.

Cloud-specific identifiers:
  AWS:        --aws-account 123456789012  (12-digit Account ID)
  GCP:        --gcp-project my-project    (Project ID or Number)
  Azure:      --az-subscription xxxxxxxx  (Subscription ID / GUID)
  Kubernetes: --kube-context my-cluster   (Context from kubeconfig)

Examples:
  # Discover all resources across all configured providers
  infractl discover

  # Discover AWS resources in a specific account and region
  infractl discover --provider aws --aws-account 123456789012 --aws-region us-east-1

  # Discover GCP resources in a specific project
  infractl discover --provider gcp --gcp-project my-production-project

  # Discover Azure resources in a specific subscription
  infractl discover --provider azure --az-subscription "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"

  # Discover only unmanaged resources (not in Terraform state)
  infractl discover --managed false

  # Discover resources matching specific tags
  infractl discover --tags "environment=production,team=platform"

  # Discover specific resource types
  infractl discover --type aws_s3_bucket,aws_rds_instance

  # Watch for new resources in real-time
  infractl discover --watch

  # Discover and sync to infra-control inventory
  infractl discover --sync

  # Output as JSON for scripting
  infractl discover --provider aws -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl discover")
	},
}

var discoverSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show discovery summary across all providers",
	Long:  `Display a summary of discovered resources grouped by provider, type, region, and management status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl discover")
	},
}

var discoverDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show differences between last two discovery runs",
	Long:  `Compare the results of the current discovery with the previous run and show added, removed, and modified resources.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl discover")
	},
}

var discoverExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export discovered resources to various formats",
	Long: `Export the full resource inventory to CSV, JSON, or Terraform import blocks.

Examples:
  infractl discover export --format csv > inventory.csv
  infractl discover export --format tf-import > imports.tf`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return requiresBackend("infractl discover")
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)
	discoverCmd.AddCommand(discoverSummaryCmd)
	discoverCmd.AddCommand(discoverDiffCmd)
	discoverCmd.AddCommand(discoverExportCmd)

	discoverCmd.Flags().StringVar(&discoverProvider, "provider", "", "filter by provider: aws, gcp, azure, kubernetes (comma-separated)")
	discoverCmd.Flags().StringVar(&discoverType, "type", "", "filter by resource type (comma-separated, e.g. aws_s3_bucket,aws_rds_instance)")
	discoverCmd.Flags().StringVar(&discoverRegion, "region", "", "filter by region (comma-separated)")
	discoverCmd.Flags().StringVar(&discoverAccount, "account", "", "filter by account/project/subscription ID")
	discoverCmd.Flags().StringVar(&discoverTags, "tags", "", "filter by tags (key=value pairs, comma-separated)")
	discoverCmd.Flags().StringVar(&discoverManaged, "managed", "", "filter by management status: true, false, terraform, manual")
	discoverCmd.Flags().BoolVarP(&discoverWatch, "watch", "w", false, "watch for resource changes in real-time (streaming mode)")
	discoverCmd.Flags().BoolVar(&discoverSync, "sync", false, "sync discovered resources to infra-control inventory")
}
