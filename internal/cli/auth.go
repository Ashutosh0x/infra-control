//go:build preview

// This command has no working implementation. It is excluded from default
// builds so that `infractl --help` lists only what the tool can actually do.
//
// A CLI whose help text advertises commands that return "not configured"
// invites the reader to judge the whole tool a scaffold, which is unfair to
// the commands that work. Build with `-tags preview` to see it.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with infra-control server and cloud providers",
	Long: `Authenticate with the infra-control API server and optionally
configure cloud provider credentials.

Authentication Methods:
  - API token (--token or INFRACTL_TOKEN env var)
  - Browser-based SSO (opens browser for OAuth2 flow)
  - Service account (--service-account)
  - Cloud provider credentials (aws/gcp/azure subcommands)

Examples:
  # Interactive login to infra-control server
  infractl login --server https://infra.example.com

  # Login with API token
  infractl login --server https://infra.example.com --token <token>

  # Verify current authentication
  infractl login status

  # Login to AWS (configures AWS credentials for infractl)
  infractl login aws --profile production

  # Login to GCP (triggers browser-based OAuth)
  infractl login gcp --project my-project

  # Login to Azure (triggers az login flow)
  infractl login azure --subscription <id>`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Authentication", "docs/ROADMAP.md#authentication")
	},
}

var loginStatusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"whoami"},
	Short:   "Show current authentication status across all providers",
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Authentication", "docs/ROADMAP.md#authentication")
	},
}

var loginAWSCmd = &cobra.Command{
	Use:   "aws",
	Short: "Configure AWS credentials for infractl",
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Authentication", "docs/ROADMAP.md#authentication")
	},
}

var loginGCPCmd = &cobra.Command{
	Use:   "gcp",
	Short: "Configure GCP credentials for infractl",
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Authentication", "docs/ROADMAP.md#authentication")
	},
}

var loginAzureCmd = &cobra.Command{
	Use:   "azure",
	Short: "Configure Azure credentials for infractl",
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Authentication", "docs/ROADMAP.md#authentication")
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove locally stored credentials",
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Authentication", "docs/ROADMAP.md#authentication")
	},
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for infractl.

To load completions:

  Bash:
    $ source <(infractl completion bash)
    # To load for each session, add to ~/.bashrc:
    $ echo 'source <(infractl completion bash)' >> ~/.bashrc

  Zsh:
    $ source <(infractl completion zsh)
    # To load for each session:
    $ infractl completion zsh > "${fpath[1]}/_infractl"

  Fish:
    $ infractl completion fish | source
    # To load for each session:
    $ infractl completion fish > ~/.config/fish/completions/infractl.fish

  PowerShell:
    PS> infractl completion powershell | Out-String | Invoke-Expression
    # To load for each session, add to $PROFILE`,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			return rootCmd.GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		default:
			return fmt.Errorf("unsupported shell: %s", args[0])
		}
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage infractl configuration",
	Long: `View and modify infractl configuration settings.

Examples:
  # Show current config
  infractl config view

  # Set a config value
  infractl config set server.address https://infra.example.com

  # Get a config value
  infractl config get server.address

  # Initialize a new config file
  infractl config init`,
}

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Display the current configuration",
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Authentication", "docs/ROADMAP.md#authentication")
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Authentication", "docs/ROADMAP.md#authentication")
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Authentication", "docs/ROADMAP.md#authentication")
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new configuration file interactively",
	RunE: func(_ *cobra.Command, _ []string) error {
		return notImplemented("Authentication", "docs/ROADMAP.md#authentication")
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(configCmd)

	loginCmd.AddCommand(loginStatusCmd)
	loginCmd.AddCommand(loginAWSCmd)
	loginCmd.AddCommand(loginGCPCmd)
	loginCmd.AddCommand(loginAzureCmd)

	configCmd.AddCommand(configViewCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configInitCmd)

	loginCmd.Flags().String("server", "", "infra-control server address")
	loginCmd.Flags().String("token", "", "API authentication token")
	loginCmd.Flags().Bool("sso", false, "use browser-based SSO authentication")
	loginAWSCmd.Flags().String("profile", "", "AWS profile name")
	loginAWSCmd.Flags().String("role-arn", "", "AWS IAM role ARN to assume")
	loginGCPCmd.Flags().String("project", "", "GCP project ID")
	loginGCPCmd.Flags().Bool("adc", false, "use Application Default Credentials")
	loginAzureCmd.Flags().String("subscription", "", "Azure subscription ID")
	loginAzureCmd.Flags().Bool("service-principal", false, "authenticate as service principal")
}
