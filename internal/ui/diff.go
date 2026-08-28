package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// DiffLine is one property-level change to render.
type DiffLine struct {
	// Path is the dotted attribute path, for example "server_side_encryption.enabled".
	Path string
	// Expected is the value recorded in Terraform state. Nil means the attribute
	// exists only in live infrastructure.
	Expected any
	// Actual is the value observed live. Nil means the attribute is absent
	// from live infrastructure.
	Actual any
	// Sensitive marks a value that must be masked rather than printed. Secrets
	// routinely appear in resource attributes and printing them to a terminal
	// puts them into shell history and CI logs.
	Sensitive bool
}

// Kind classifies the change for display purposes.
func (d DiffLine) Kind() string {
	switch {
	case d.Expected == nil && d.Actual != nil:
		return "added"
	case d.Expected != nil && d.Actual == nil:
		return "removed"
	default:
		return "changed"
	}
}

// redacted is what stands in for a sensitive value. It names the reason so a
// reader does not mistake it for the literal stored value.
const redacted = "(sensitive value hidden)"

// RenderDiff formats property changes in a unified, aligned layout.
//
// Lines are grouped by kind and sorted by path so that two runs over the same
// resource produce byte-identical output, which lets the result be diffed or
// checksummed in CI.
func (r *Renderer) RenderDiff(lines []DiffLine) string {
	if len(lines) == 0 {
		return ""
	}

	sorted := make([]DiffLine, len(lines))
	copy(sorted, lines)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Kind() != sorted[j].Kind() {
			return kindOrder(sorted[i].Kind()) < kindOrder(sorted[j].Kind())
		}
		return sorted[i].Path < sorted[j].Path
	})

	sym := r.Symbols()
	var b strings.Builder

	for _, line := range sorted {
		switch line.Kind() {
		case "added":
			b.WriteString(r.Apply(StyleAdded, fmt.Sprintf("  %s %s = %s", sym.Added, line.Path, formatValue(line.Actual, line.Sensitive))))
			b.WriteString("\n")

		case "removed":
			b.WriteString(r.Apply(StyleRemoved, fmt.Sprintf("  %s %s = %s", sym.Removed, line.Path, formatValue(line.Expected, line.Sensitive))))
			b.WriteString("\n")

		default:
			// A changed value gets two lines so both sides are readable even when
			// the values are long. The arrow form collapses to one line only when
			// both values are short.
			expected := formatValue(line.Expected, line.Sensitive)
			actual := formatValue(line.Actual, line.Sensitive)

			head := fmt.Sprintf("  %s %s", sym.Changed, line.Path)
			if displayWidth(head)+displayWidth(expected)+displayWidth(actual)+8 <= r.width {
				b.WriteString(r.Apply(StyleChanged, head))
				b.WriteString(" " + r.Apply(StyleRemoved, expected))
				b.WriteString(" " + r.Apply(StyleMuted, sym.Arrow))
				b.WriteString(" " + r.Apply(StyleAdded, actual) + "\n")
				continue
			}

			b.WriteString(r.Apply(StyleChanged, head) + "\n")
			b.WriteString("      " + r.Apply(StyleMuted, "state: ") + r.Apply(StyleRemoved, expected) + "\n")
			b.WriteString("      " + r.Apply(StyleMuted, "live:  ") + r.Apply(StyleAdded, actual) + "\n")
		}
	}

	return b.String()
}

// kindOrder gives removed/added/changed a stable display order.
func kindOrder(kind string) int {
	switch kind {
	case "removed":
		return 0
	case "added":
		return 1
	default:
		return 2
	}
}

// maxInlineValue caps how much of a long value is printed inline. Attributes
// such as IAM policy documents run to kilobytes and would bury the diff.
const maxInlineValue = 120

// formatValue renders an attribute value for display, masking sensitive values
// and compacting structured ones onto a single line.
func formatValue(v any, sensitive bool) string {
	if sensitive {
		return redacted
	}
	if v == nil {
		return "null"
	}

	switch t := v.(type) {
	case string:
		if !validUTF8(t) {
			return fmt.Sprintf("(%d bytes of binary data)", len(t))
		}
		return Truncate(fmt.Sprintf("%q", t), maxInlineValue, `…"`)
	case bool:
		return fmt.Sprintf("%t", t)
	case float64:
		// JSON numbers decode as float64; print integral values without a
		// trailing ".0" so a port number reads as 443, not 443.0.
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case int, int32, int64:
		return fmt.Sprintf("%d", t)
	}

	encoded, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return Truncate(string(encoded), maxInlineValue, "…")
}

// DiffStat summarises a set of changes as a one-line counter, the form used in
// scan summaries where the full diff would be too much.
func (r *Renderer) DiffStat(added, removed, changed int) string {
	sym := r.Symbols()
	parts := make([]string, 0, 3)
	if added > 0 {
		parts = append(parts, r.Apply(StyleAdded, fmt.Sprintf("%s%d", sym.Added, added)))
	}
	if removed > 0 {
		parts = append(parts, r.Apply(StyleRemoved, fmt.Sprintf("%s%d", sym.Removed, removed)))
	}
	if changed > 0 {
		parts = append(parts, r.Apply(StyleChanged, fmt.Sprintf("%s%d", sym.Changed, changed)))
	}
	if len(parts) == 0 {
		return r.Apply(StyleMuted, "no changes")
	}
	return strings.Join(parts, " ")
}
