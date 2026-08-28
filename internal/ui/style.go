package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ANSI SGR sequences. The palette is restricted to the 8 basic colours plus
// bright variants so output stays legible on light and dark terminal themes
// alike; 256-colour and truecolor sequences are avoided because they render
// unpredictably against unknown backgrounds.
const (
	ansiReset     = "\x1b[0m"
	ansiBold      = "\x1b[1m"
	ansiDim       = "\x1b[2m"
	ansiItalic    = "\x1b[3m"
	ansiUnderline = "\x1b[4m"

	ansiRed     = "\x1b[31m"
	ansiGreen   = "\x1b[32m"
	ansiYellow  = "\x1b[33m"
	ansiBlue    = "\x1b[34m"
	ansiMagenta = "\x1b[35m"
	ansiCyan    = "\x1b[36m"
	ansiGray    = "\x1b[90m"

	ansiBrightRed    = "\x1b[91m"
	ansiBrightYellow = "\x1b[93m"
)

// Style is a composable set of SGR attributes.
type Style struct {
	codes []string
}

// Predefined styles covering every semantic role the CLI needs.
var (
	StyleNone      = Style{}
	StyleBold      = Style{codes: []string{ansiBold}}
	StyleDim       = Style{codes: []string{ansiDim}}
	StyleItalic    = Style{codes: []string{ansiItalic}}
	StyleUnderline = Style{codes: []string{ansiUnderline}}

	StyleSuccess = Style{codes: []string{ansiGreen}}
	StyleWarning = Style{codes: []string{ansiYellow}}
	StyleError   = Style{codes: []string{ansiRed}}
	StyleInfo    = Style{codes: []string{ansiCyan}}
	StyleMuted   = Style{codes: []string{ansiGray}}
	StyleHeading = Style{codes: []string{ansiBold, ansiCyan}}
	StyleAccent  = Style{codes: []string{ansiMagenta}}
	StyleLink    = Style{codes: []string{ansiUnderline, ansiBlue}}

	// Severity styles. Critical is bold-bright-red so it separates from the
	// merely high findings at a glance in a long list.
	StyleCritical = Style{codes: []string{ansiBold, ansiBrightRed}}
	StyleHigh     = Style{codes: []string{ansiRed}}
	StyleMedium   = Style{codes: []string{ansiBrightYellow}}
	StyleLow      = Style{codes: []string{ansiBlue}}
	StyleNegligib = Style{codes: []string{ansiGray}}

	// Diff styles.
	StyleAdded   = Style{codes: []string{ansiGreen}}
	StyleRemoved = Style{codes: []string{ansiRed}}
	StyleChanged = Style{codes: []string{ansiYellow}}
)

// With returns a style combining s with the given additional style.
func (s Style) With(other Style) Style {
	combined := make([]string, 0, len(s.codes)+len(other.codes))
	combined = append(combined, s.codes...)
	combined = append(combined, other.codes...)
	return Style{codes: combined}
}

// Apply wraps text in this style's escape sequences when colour is enabled.
// With colour off it returns the text unchanged, so callers never need to
// branch on colour support themselves.
func (r *Renderer) Apply(s Style, text string) string {
	if !r.color || len(s.codes) == 0 || text == "" {
		return text
	}
	return strings.Join(s.codes, "") + text + ansiReset
}

// Sprintf formats and styles in one step.
func (r *Renderer) Sprintf(s Style, format string, args ...any) string {
	return r.Apply(s, fmt.Sprintf(format, args...))
}

// SeverityStyle maps a severity or risk level name onto its style. The lookup
// is case-insensitive and covers the vocabulary used by both the drift engine
// (critical/high/medium/low/info) and the risk engine (which adds negligible).
func SeverityStyle(level string) Style {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "critical":
		return StyleCritical
	case "high":
		return StyleHigh
	case "medium", "moderate":
		return StyleMedium
	case "low":
		return StyleLow
	case "info", "informational", "negligible":
		return StyleNegligib
	default:
		return StyleNone
	}
}

// Symbols is the set of glyphs used for status and structure. Two sets exist so
// that a terminal which cannot render box-drawing characters still produces
// aligned, readable output rather than mojibake.
type Symbols struct {
	Success   string
	Failure   string
	Warning   string
	Info      string
	Pending   string
	Bullet    string
	Arrow     string
	Added     string
	Removed   string
	Changed   string
	Ellipsis  string
	TreeItem  string
	TreeLast  string
	TreePipe  string
	TreeBlank string

	// Box-drawing runes for table and panel borders.
	Horizontal  string
	Vertical    string
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
	TeeDown     string
	TeeUp       string
	TeeRight    string
	TeeLeft     string
	Cross       string
}

var unicodeSymbols = Symbols{
	Success:   "✓", // check mark
	Failure:   "✗", // ballot x
	Warning:   "!",
	Info:      "•", // bullet
	Pending:   "○", // white circle
	Bullet:    "•",
	Arrow:     "→", // rightwards arrow
	Added:     "+",
	Removed:   "-",
	Changed:   "~",
	Ellipsis:  "…",
	TreeItem:  "├─ ",
	TreeLast:  "└─ ",
	TreePipe:  "│  ",
	TreeBlank: "   ",

	Horizontal:  "─",
	Vertical:    "│",
	TopLeft:     "┌",
	TopRight:    "┐",
	BottomLeft:  "└",
	BottomRight: "┘",
	TeeDown:     "┬",
	TeeUp:       "┴",
	TeeRight:    "├",
	TeeLeft:     "┤",
	Cross:       "┼",
}

var asciiSymbols = Symbols{
	Success:   "OK",
	Failure:   "X",
	Warning:   "!",
	Info:      "*",
	Pending:   "o",
	Bullet:    "*",
	Arrow:     "->",
	Added:     "+",
	Removed:   "-",
	Changed:   "~",
	Ellipsis:  "...",
	TreeItem:  "|- ",
	TreeLast:  "`- ",
	TreePipe:  "|  ",
	TreeBlank: "   ",

	Horizontal:  "-",
	Vertical:    "|",
	TopLeft:     "+",
	TopRight:    "+",
	BottomLeft:  "+",
	BottomRight: "+",
	TeeDown:     "+",
	TeeUp:       "+",
	TeeRight:    "+",
	TeeLeft:     "+",
	Cross:       "+",
}

// Symbols returns the glyph set matching the renderer's terminal capabilities.
func (r *Renderer) Symbols() Symbols {
	if r.unicode {
		return unicodeSymbols
	}
	return asciiSymbols
}

// displayWidth returns the rendered column count of s, ignoring any ANSI escape
// sequences it contains. Table alignment depends on this: measuring a styled
// cell with len() would count the invisible escape bytes and misalign the row.
func displayWidth(s string) int {
	width := 0

	// An ANSI control sequence is ESC, then '[', then zero or more parameter
	// bytes, then one final byte in the range @ to ~. The '[' itself falls
	// inside that terminator range, so the scanner must consume it before it
	// starts looking for the end, or every sequence terminates one byte early
	// and its parameter digits get counted as visible text.
	const (
		stateText = iota
		stateEscape
		stateCSI
	)

	state := stateText
	for _, rn := range s {
		switch state {
		case stateEscape:
			if rn == '[' {
				state = stateCSI
				continue
			}
			// A two-character escape such as ESC c; nothing further to skip.
			state = stateText

		case stateCSI:
			if rn >= '@' && rn <= '~' {
				state = stateText
			}
			continue
		}

		if rn == '\x1b' {
			state = stateEscape
			continue
		}
		width += runeWidth(rn)
	}
	return width
}

// runeWidth returns the column count of a single rune. Combining marks occupy
// no columns; East Asian wide and fullwidth forms occupy two.
func runeWidth(rn rune) int {
	switch {
	case rn == 0:
		return 0
	case rn < 32 || (rn >= 0x7f && rn < 0xa0):
		return 0
	case rn >= 0x0300 && rn <= 0x036f: // combining diacritical marks
		return 0
	case rn >= 0x1100 && rn <= 0x115f, // Hangul Jamo
		rn >= 0x2e80 && rn <= 0xa4cf, // CJK radicals through Yi
		rn >= 0xac00 && rn <= 0xd7a3, // Hangul syllables
		rn >= 0xf900 && rn <= 0xfaff, // CJK compatibility ideographs
		rn >= 0xfe30 && rn <= 0xfe6f, // CJK compatibility forms
		rn >= 0xff00 && rn <= 0xff60, // fullwidth forms
		rn >= 0xffe0 && rn <= 0xffe6:
		return 2
	default:
		return 1
	}
}

// Truncate shortens s to at most width columns, appending an ellipsis when it
// had to cut. Text shorter than the limit is returned untouched.
//
// The input is assumed to be unstyled; truncating a string that already
// contains escape sequences risks severing one mid-sequence.
func Truncate(s string, width int, ellipsis string) string {
	if width <= 0 {
		return ""
	}
	if displayWidth(s) <= width {
		return s
	}

	ellipsisWidth := displayWidth(ellipsis)
	if width <= ellipsisWidth {
		// No room for content alongside the ellipsis; return a hard cut.
		return string([]rune(s)[:width])
	}

	budget := width - ellipsisWidth
	var b strings.Builder
	used := 0
	for _, rn := range s {
		w := runeWidth(rn)
		if used+w > budget {
			break
		}
		b.WriteRune(rn)
		used += w
	}
	return b.String() + ellipsis
}

// Pad right-pads s with spaces to the given display width.
func Pad(s string, width int) string {
	gap := width - displayWidth(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

// PadLeft left-pads s with spaces to the given display width, which is what
// numeric columns need so their digits line up.
func PadLeft(s string, width int) string {
	gap := width - displayWidth(s)
	if gap <= 0 {
		return s
	}
	return strings.Repeat(" ", gap) + s
}

// Repeat builds a run of a possibly multi-byte symbol to the given width.
func Repeat(sym string, width int) string {
	if width <= 0 {
		return ""
	}
	w := displayWidth(sym)
	if w == 0 {
		return ""
	}
	return strings.Repeat(sym, width/w)
}

// Wrap breaks text into lines of at most width columns, splitting on spaces.
// A word longer than the width is placed on its own line rather than being
// broken, which keeps identifiers such as ARNs and resource addresses intact.
func Wrap(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		current := words[0]
		for _, word := range words[1:] {
			if displayWidth(current)+1+displayWidth(word) <= width {
				current += " " + word
				continue
			}
			lines = append(lines, current)
			current = word
		}
		lines = append(lines, current)
	}
	return lines
}

// Indent prefixes every line of s with the given prefix.
func Indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// validUTF8 reports whether s is well-formed UTF-8, used to guard against
// emitting raw bytes from cloud APIs straight into the terminal.
func validUTF8(s string) bool { return utf8.ValidString(s) }
