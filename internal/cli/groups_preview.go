//go:build preview

package cli

import "github.com/spf13/cobra"

// registerPreviewGroups adds the group the unimplemented commands belong to.
func registerPreviewGroups() {
	rootCmd.AddGroup(&cobra.Group{
		ID:    "platform",
		Title: "Control plane (not implemented; these return an error):",
	})
}
