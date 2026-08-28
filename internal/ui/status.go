package ui

import (
	"fmt"
	"strings"
)

// Status messages. These all write to stderr so that a command's actual result
// on stdout stays clean for piping. Each is prefixed with a symbol and a colour
// that agree with one another, so the message survives a mono terminal.

// Success reports a completed action.
func (r *Renderer) Success(format string, args ...any) {
	r.status(StyleSuccess, r.Symbols().Success, format, args...)
}

// Failure reports a failed action. It ignores quiet mode, because suppressing
// a failure notice would leave a user with a silent non-zero exit.
func (r *Renderer) Failure(format string, args ...any) {
	sym := r.Symbols().Failure
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.err, "%s %s\n", r.Apply(StyleError, sym), fmt.Sprintf(format, args...))
}

// Warn reports a condition that did not stop the command.
func (r *Renderer) Warn(format string, args ...any) {
	r.status(StyleWarning, r.Symbols().Warning, format, args...)
}

// Info reports neutral progress detail.
func (r *Renderer) Info(format string, args ...any) {
	r.status(StyleInfo, r.Symbols().Info, format, args...)
}

// Detail reports secondary context, indented under the preceding message.
func (r *Renderer) Detail(format string, args ...any) {
	if r.quiet {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.err, "  %s\n", r.Apply(StyleMuted, fmt.Sprintf(format, args...)))
}

// status is the shared implementation for the symbol-prefixed messages.
func (r *Renderer) status(style Style, symbol, format string, args ...any) {
	if r.quiet {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.err, "%s %s\n", r.Apply(style, symbol), fmt.Sprintf(format, args...))
}

// Heading writes a section heading to stdout, underlined to the width of the
// text rather than the terminal, which keeps it readable in a narrow window.
func (r *Renderer) Heading(text string) {
	sym := r.Symbols()
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.out, "\n%s\n%s\n", r.Apply(StyleHeading, text), r.Apply(StyleMuted, Repeat(sym.Horizontal, displayWidth(text))))
}

// Panel draws a titled box around a block of text. It is reserved for content
// that must not be missed, such as a destructive-change confirmation summary.
func (r *Renderer) Panel(title string, lines []string, style Style) string {
	sym := r.Symbols()

	// Size the panel to its content, capped at the terminal width.
	inner := displayWidth(title) + 2
	for _, line := range lines {
		if w := displayWidth(line); w > inner {
			inner = w
		}
	}
	if limit := r.width - 4; inner > limit {
		inner = limit
	}

	var b strings.Builder
	top := sym.TopLeft + " " + title + " " + Repeat(sym.Horizontal, inner-displayWidth(title)-2) + sym.TopRight
	b.WriteString(r.Apply(style, top) + "\n")

	for _, line := range lines {
		for _, wrapped := range Wrap(line, inner) {
			b.WriteString(r.Apply(style, sym.Vertical) + " " + Pad(wrapped, inner) + " " + r.Apply(style, sym.Vertical) + "\n")
		}
	}

	b.WriteString(r.Apply(style, sym.BottomLeft+Repeat(sym.Horizontal, inner+2)+sym.BottomRight) + "\n")
	return b.String()
}

// KeyValue renders an aligned key-value block, the layout used by every "show"
// subcommand. Keys are dimmed so the eye lands on the values.
func (r *Renderer) KeyValue(pairs [][2]string) string {
	width := 0
	for _, kv := range pairs {
		if w := displayWidth(kv[0]); w > width {
			width = w
		}
	}

	var b strings.Builder
	for _, kv := range pairs {
		key := r.Apply(StyleMuted, Pad(kv[0]+":", width+1))
		b.WriteString("  " + key + "  " + kv[1] + "\n")
	}
	return b.String()
}

// Bar renders a horizontal meter for a 0..max value, used by the risk and
// compliance score displays. The bar is drawn with block characters when the
// terminal supports them and equals signs otherwise.
func (r *Renderer) Bar(value, scale float64, width int, style Style) string {
	if scale <= 0 || width <= 0 {
		return ""
	}
	ratio := value / scale
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	filled := int(ratio * float64(width))
	fillRune, emptyRune := "=", "."
	if r.unicode {
		fillRune, emptyRune = "█", "░"
	}

	return r.Apply(style, strings.Repeat(fillRune, filled)) +
		r.Apply(StyleMuted, strings.Repeat(emptyRune, width-filled))
}

// Count formats a labelled count, dimming the label when the count is zero so
// that a summary line draws attention only to the non-empty buckets.
func (r *Renderer) Count(label string, n int, style Style) string {
	if n == 0 {
		return r.Apply(StyleMuted, fmt.Sprintf("%d %s", n, label))
	}
	return r.Apply(style, fmt.Sprintf("%d", n)) + " " + label
}
