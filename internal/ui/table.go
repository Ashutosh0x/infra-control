package ui

import (
	"strings"
)

// Alignment controls horizontal placement of a column's cells.
type Alignment int

const (
	// AlignLeft is the default and correct choice for names and identifiers.
	AlignLeft Alignment = iota
	// AlignRight suits numbers, so digits line up on the decimal position.
	AlignRight
)

// Column describes one table column.
type Column struct {
	// Title is the header text.
	Title string
	// Align controls cell placement.
	Align Alignment
	// MinWidth prevents the column from being squeezed below this many columns
	// when the table has to fit a narrow terminal.
	MinWidth int
	// MaxWidth caps the column even when space is available. Zero means no cap.
	MaxWidth int
	// Truncatable marks a column as safe to shorten when the table overflows.
	// Identifier columns should leave this false so they stay copy-pasteable.
	Truncatable bool
	// Style is applied to every cell in the column unless the cell carries its own.
	Style Style
}

// Cell is a single table cell. A cell may override the column style, which is
// how a severity column colours each row differently.
type Cell struct {
	Text  string
	Style Style
	// styled marks that Style should be used even if it is the zero Style.
	styled bool
}

// Text builds a plain cell inheriting the column style.
func Text(s string) Cell { return Cell{Text: s} }

// Styled builds a cell with an explicit style.
func Styled(s string, style Style) Cell { return Cell{Text: s, Style: style, styled: true} }

// Table accumulates rows and renders them as an aligned grid.
type Table struct {
	columns []Column
	rows    [][]Cell
	// borders selects the bordered box layout over the default compact layout.
	borders bool
}

// NewTable starts a table with the given columns.
func NewTable(columns ...Column) *Table {
	return &Table{columns: columns}
}

// WithBorders draws full box-drawing borders around and between cells. The
// default compact style, which uses only a header rule, is easier to scan for
// long lists and is what most subcommands should use.
func (t *Table) WithBorders() *Table {
	t.borders = true
	return t
}

// Row appends a row. Rows with fewer cells than columns are padded with blanks;
// extra cells are dropped so a miscounted row can never break alignment.
func (t *Table) Row(cells ...Cell) *Table {
	row := make([]Cell, len(t.columns))
	copy(row, cells)
	t.rows = append(t.rows, row)
	return t
}

// StringRow appends a row from plain strings, which covers most call sites.
func (t *Table) StringRow(values ...string) *Table {
	cells := make([]Cell, len(values))
	for i, v := range values {
		cells[i] = Text(v)
	}
	return t.Row(cells...)
}

// Len returns the number of rows added so far.
func (t *Table) Len() int { return len(t.rows) }

// Render lays the table out for the renderer's terminal width and returns it as
// a string ending in a newline. An empty table renders as an empty string; the
// caller is responsible for printing a "no results" message, because only the
// caller knows what was being searched for.
func (r *Renderer) Render(t *Table) string {
	if len(t.rows) == 0 || len(t.columns) == 0 {
		return ""
	}

	widths := t.computeWidths(r.width)

	var b strings.Builder
	sym := r.Symbols()

	if t.borders {
		b.WriteString(r.borderLine(sym, widths, sym.TopLeft, sym.TeeDown, sym.TopRight))
	}

	// Header.
	header := make([]string, len(t.columns))
	for i, col := range t.columns {
		title := Truncate(col.Title, widths[i], sym.Ellipsis)
		if col.Align == AlignRight {
			title = PadLeft(title, widths[i])
		} else {
			title = Pad(title, widths[i])
		}
		header[i] = r.Apply(StyleBold, title)
	}
	b.WriteString(r.joinRow(sym, header, t.borders))

	// Header rule.
	if t.borders {
		b.WriteString(r.borderLine(sym, widths, sym.TeeRight, sym.Cross, sym.TeeLeft))
	} else {
		rule := make([]string, len(widths))
		for i, w := range widths {
			rule[i] = Repeat(sym.Horizontal, w)
		}
		b.WriteString(r.Apply(StyleMuted, strings.Join(rule, "  "))) //nolint:gocritic // two-space gutter matches joinRow
		b.WriteString("\n")
	}

	// Body.
	for _, row := range t.rows {
		rendered := make([]string, len(t.columns))
		for i, col := range t.columns {
			cell := row[i]
			text := Truncate(cell.Text, widths[i], sym.Ellipsis)
			if col.Align == AlignRight {
				text = PadLeft(text, widths[i])
			} else {
				text = Pad(text, widths[i])
			}

			style := col.Style
			if cell.styled {
				style = cell.Style
			}
			rendered[i] = r.Apply(style, text)
		}
		b.WriteString(r.joinRow(sym, rendered, t.borders))
	}

	if t.borders {
		b.WriteString(r.borderLine(sym, widths, sym.BottomLeft, sym.TeeUp, sym.BottomRight))
	}

	return b.String()
}

// joinRow assembles one rendered row, with or without vertical borders.
func (r *Renderer) joinRow(sym Symbols, cells []string, borders bool) string {
	if !borders {
		// Trailing whitespace is trimmed so that copying a row out of the
		// terminal does not pick up padding.
		return strings.TrimRight(strings.Join(cells, "  "), " ") + "\n"
	}
	bar := r.Apply(StyleMuted, sym.Vertical)
	return bar + " " + strings.Join(cells, " "+bar+" ") + " " + bar + "\n"
}

// borderLine builds a horizontal rule using the given corner and junction runes.
func (r *Renderer) borderLine(sym Symbols, widths []int, left, mid, right string) string {
	segments := make([]string, len(widths))
	for i, w := range widths {
		segments[i] = Repeat(sym.Horizontal, w+2)
	}
	return r.Apply(StyleMuted, left+strings.Join(segments, mid)+right) + "\n"
}

// computeWidths sizes each column to its widest cell, then shrinks truncatable
// columns proportionally if the table would overflow the terminal.
func (t *Table) computeWidths(available int) []int {
	widths := make([]int, len(t.columns))

	// Natural width: the widest cell, header included.
	for i, col := range t.columns {
		widths[i] = displayWidth(col.Title)
		for _, row := range t.rows {
			if w := displayWidth(row[i].Text); w > widths[i] {
				widths[i] = w
			}
		}
		if col.MaxWidth > 0 && widths[i] > col.MaxWidth {
			widths[i] = col.MaxWidth
		}
		if widths[i] < col.MinWidth {
			widths[i] = col.MinWidth
		}
	}

	// Gutters: two spaces between columns, or the border decoration.
	gutter := 2 * (len(t.columns) - 1)
	if t.borders {
		gutter = 3*(len(t.columns)-1) + 4
	}

	total := gutter
	for _, w := range widths {
		total += w
	}
	if total <= available {
		return widths
	}

	// Overflow: reclaim space from truncatable columns, largest first, never
	// taking a column below its MinWidth or below a floor of 8 columns.
	excess := total - available
	for excess > 0 {
		widest, widestIdx := 0, -1
		for i, col := range t.columns {
			if !col.Truncatable {
				continue
			}
			floor := col.MinWidth
			if floor < 8 {
				floor = 8
			}
			if widths[i] > floor && widths[i] > widest {
				widest, widestIdx = widths[i], i
			}
		}
		if widestIdx < 0 {
			// Nothing left to shrink; the caller's terminal is simply too narrow.
			break
		}
		widths[widestIdx]--
		excess--
	}

	return widths
}
