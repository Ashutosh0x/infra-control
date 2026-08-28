//go:build windows

package ui

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// isWindows reports whether the build targets Windows.
func isWindows() bool { return true }

// enableVirtualTerminal switches a Windows console handle into virtual terminal
// mode so ANSI escape sequences are interpreted rather than printed literally.
//
// Windows 10 1511 and later support this; on older builds the SetConsoleMode
// call fails and we leave the mode untouched, which is why the error is
// deliberately ignored. Callers decide separately whether to emit colour.
func enableVirtualTerminal(w io.Writer) {
	f, ok := w.(*os.File)
	if !ok {
		return
	}
	handle := windows.Handle(f.Fd())

	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return
	}
	_ = windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
