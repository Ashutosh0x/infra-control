//go:build !windows

package ui

import "io"

// isWindows reports whether the build targets Windows.
func isWindows() bool { return false }

// enableVirtualTerminal is a no-op outside Windows, where terminals interpret
// ANSI escape sequences without any mode change.
func enableVirtualTerminal(io.Writer) {}
