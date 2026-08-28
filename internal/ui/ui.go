// Package ui provides the terminal presentation layer for infractl.
//
// Everything a user sees goes through this package, which gives the CLI a
// single place to enforce three rules:
//
//  1. Colour and animation are enabled only when the destination is an
//     interactive terminal that has not opted out. Redirect to a file or pipe
//     into another program and the output degrades to clean plain text.
//  2. Human output goes to stdout, diagnostics go to stderr. A caller can pipe
//     stdout into jq without stripping progress chatter first.
//  3. No emoji, anywhere. Status is carried by words, colour, and box-drawing
//     characters, all of which have ASCII fallbacks for terminals that cannot
//     render them.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// ColorMode controls when ANSI styling is emitted.
type ColorMode int

const (
	// ColorAuto enables styling only on an interactive terminal that has not
	// set NO_COLOR. This is the default and the right choice for almost all uses.
	ColorAuto ColorMode = iota
	// ColorAlways forces styling on even when the destination is not a terminal.
	ColorAlways
	// ColorNever disables styling unconditionally.
	ColorNever
)

// ParseColorMode maps a user-supplied string onto a ColorMode.
func ParseColorMode(s string) (ColorMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "auto", "":
		return ColorAuto, nil
	case "always", "force", "true", "yes":
		return ColorAlways, nil
	case "never", "none", "false", "no":
		return ColorNever, nil
	default:
		return ColorAuto, fmt.Errorf("invalid color mode %q (want auto, always, or never)", s)
	}
}

// defaultWidth is used when the terminal width cannot be determined, which is
// the case whenever output is redirected.
const defaultWidth = 100

// minWidth is the narrowest layout the table renderer will target. Below this
// the output is unreadable regardless, so clamping avoids pathological wrapping.
const minWidth = 40

// Renderer carries the resolved presentation settings for one command run.
// It is safe for concurrent use; the spinner serialises its own writes.
type Renderer struct {
	out       io.Writer
	err       io.Writer
	color     bool
	unicode   bool
	width     int
	quiet     bool
	mu        sync.Mutex
	activeSpn *Spinner
}

// Options configures a Renderer.
type Options struct {
	// Out receives human-readable and machine-readable results. Defaults to os.Stdout.
	Out io.Writer
	// Err receives progress, warnings, and errors. Defaults to os.Stderr.
	Err io.Writer
	// Color decides when ANSI styling is emitted.
	Color ColorMode
	// Quiet suppresses progress and non-essential chatter on Err.
	Quiet bool
	// Width overrides terminal width detection. Zero means detect.
	Width int
	// ASCII forces the ASCII symbol set even on a capable terminal.
	ASCII bool
}

// New builds a Renderer from the given options, resolving colour support,
// symbol set, and width against the real environment.
func New(opts Options) *Renderer {
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	errW := opts.Err
	if errW == nil {
		errW = os.Stderr
	}

	r := &Renderer{
		out:     out,
		err:     errW,
		quiet:   opts.Quiet,
		color:   resolveColor(opts.Color, out),
		unicode: !opts.ASCII && supportsUnicode(),
		width:   opts.Width,
	}
	if r.width <= 0 {
		r.width = detectWidth(out)
	}
	if r.width < minWidth {
		r.width = minWidth
	}
	if r.color {
		// Windows consoles need virtual terminal processing switched on before
		// they interpret ANSI escapes rather than printing them literally.
		enableVirtualTerminal(out)
		enableVirtualTerminal(errW)
	}
	return r
}

// resolveColor applies the NO_COLOR and FORCE_COLOR conventions.
//
// NO_COLOR (https://no-color.org) wins over auto-detection but not over an
// explicit --color=always, which is what a user reaches for when piping into a
// pager that understands escapes.
func resolveColor(mode ColorMode, out io.Writer) bool {
	switch mode {
	case ColorNever:
		return false
	case ColorAlways:
		return true
	}

	if v, ok := os.LookupEnv("NO_COLOR"); ok && v != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if v, ok := os.LookupEnv("FORCE_COLOR"); ok && v != "" && v != "0" {
		return true
	}
	return isTerminal(out)
}

// supportsUnicode reports whether box-drawing characters are safe to emit.
// The common failure case is a legacy Windows console under a non-UTF codepage,
// where box characters render as mojibake.
func supportsUnicode() bool {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(key); v != "" {
			return strings.Contains(strings.ToUpper(v), "UTF")
		}
	}
	// Windows Terminal and modern VS Code terminals set WT_SESSION or TERM_PROGRAM
	// and handle UTF-8 correctly; a bare cmd.exe sets neither.
	if os.Getenv("WT_SESSION") != "" || os.Getenv("TERM_PROGRAM") != "" {
		return true
	}
	return !isWindows()
}

// isTerminal reports whether w is an interactive terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// detectWidth returns the usable column count for w.
func detectWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return defaultWidth
	}
	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil || width <= 0 {
		return defaultWidth
	}
	return width
}

// Out returns the result stream.
func (r *Renderer) Out() io.Writer { return r.out }

// Err returns the diagnostic stream.
func (r *Renderer) Err() io.Writer { return r.err }

// Color reports whether ANSI styling is active.
func (r *Renderer) Color() bool { return r.color }

// Unicode reports whether the extended symbol set is active.
func (r *Renderer) Unicode() bool { return r.unicode }

// Width returns the target line width.
func (r *Renderer) Width() int { return r.width }

// Quiet reports whether non-essential output is suppressed.
func (r *Renderer) Quiet() bool { return r.quiet }

// IsInteractive reports whether the renderer is attached to a terminal on both
// streams, which is the precondition for prompting the user for input.
func (r *Renderer) IsInteractive() bool {
	return isTerminal(r.out) && isTerminal(r.err) && isTerminal(os.Stdin)
}

// Printf writes a formatted result line to stdout.
func (r *Renderer) Printf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.out, format, args...)
}

// Println writes a result line to stdout.
func (r *Renderer) Println(args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintln(r.out, args...)
}

// Raw writes a pre-rendered block to stdout verbatim.
func (r *Renderer) Raw(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprint(r.out, s)
}

// Diagnosticf writes a formatted line to stderr, honouring quiet mode.
func (r *Renderer) Diagnosticf(format string, args ...any) {
	if r.quiet {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.err, format, args...)
}
