package cli

import (
	"github.com/ashutosh0x/infra-control/internal/ui"
	"github.com/ashutosh0x/infra-control/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Short:   "Print version, commit, and build information",
	GroupID: "system",
	Long: `Print the version of infractl along with the commit it was built from,
the build date, and the Go toolchain and platform it was built with.

Include this output when reporting a bug.`,
	Example: `  infractl version
  infractl version -o json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		info := version.Get()

		table := ui.NewTable(
			ui.Column{Title: "FIELD", MinWidth: 10},
			ui.Column{Title: "VALUE", MinWidth: 10, Truncatable: true},
		)
		table.StringRow("version", info.Version)
		table.StringRow("commit", info.Commit)
		table.StringRow("built", info.BuildDate)
		table.StringRow("go", info.GoVersion)
		table.StringRow("platform", info.Platform)

		return rt.write(ui.View{
			Data:  info,
			Table: table,
			Names: []string{info.Version},
		})
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
