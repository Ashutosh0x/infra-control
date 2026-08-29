//go:build !preview

package cli

// registerPreviewGroups is a no-op in a default build.
//
// The control-plane group exists only when the commands that belong to it are
// compiled in. An empty section in `--help` advertises absence, which is worse
// than not mentioning it.
func registerPreviewGroups() {}
