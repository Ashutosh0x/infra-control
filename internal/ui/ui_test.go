package ui

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// newTestRenderer builds a renderer writing to buffers, with colour forced to a
// known state so assertions do not depend on the test environment's terminal.
func newTestRenderer(color bool) (*Renderer, *bytes.Buffer, *bytes.Buffer) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	mode := ColorNever
	if color {
		mode = ColorAlways
	}
	r := New(Options{Out: out, Err: errOut, Color: mode, Width: 100})
	return r, out, errOut
}

func TestParseFormat(t *testing.T) {
	cases := []struct {
		input   string
		format  Format
		arg     string
		wantErr bool
	}{
		{"table", FormatTable, "", false},
		{"json", FormatJSON, "", false},
		{"yml", FormatYAML, "", false},
		{"", FormatTable, "", false},
		{"go-template={{.ID}}", FormatGoTemplate, "{{.ID}}", false},
		{"go-template", "", "", true},
		{"json=oops", "", "", true},
		{"nonsense", "", "", true},
	}

	for _, tc := range cases {
		format, arg, err := ParseFormat(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseFormat(%q) should have failed", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseFormat(%q): %v", tc.input, err)
			continue
		}
		if format != tc.format || arg != tc.arg {
			t.Errorf("ParseFormat(%q) = %q/%q, want %q/%q", tc.input, format, arg, tc.format, tc.arg)
		}
	}
}

func TestMachineFormatsAreRecognised(t *testing.T) {
	// Progress output is suppressed based on this, so a format misclassified as
	// human would corrupt a piped payload.
	for _, f := range []Format{FormatJSON, FormatYAML, FormatCSV, FormatTSV, FormatName, FormatGoTemplate} {
		if !f.IsMachine() {
			t.Errorf("%q should be a machine format", f)
		}
	}
	for _, f := range []Format{FormatTable, FormatWide} {
		if f.IsMachine() {
			t.Errorf("%q should be a human format", f)
		}
	}
}

func TestJSONOutputCarriesNoEscapeSequences(t *testing.T) {
	// Even with colour forced on, a machine payload must stay parseable.
	r, out, _ := newTestRenderer(true)

	payload := map[string]any{"address": "aws_s3_bucket.assets", "severity": "critical"}
	if err := r.Write(View{Data: payload}, FormatJSON, ""); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if strings.Contains(out.String(), "\x1b") {
		t.Fatalf("JSON output contains ANSI escapes: %q", out.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("JSON output is not parseable: %v\n%s", err, out.String())
	}
	if decoded["severity"] != "critical" {
		t.Errorf("round-trip lost data: %+v", decoded)
	}
}

func TestJSONOutputDoesNotEscapeHTML(t *testing.T) {
	// Resource ARNs and IAM policy documents contain characters the default
	// encoder would mangle into < sequences.
	r, out, _ := newTestRenderer(false)

	payload := map[string]string{"condition": `a < b && c > d`}
	if err := r.Write(View{Data: payload}, FormatJSON, ""); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// With escaping disabled the angle brackets and ampersand stay literal
	// instead of being rewritten as the six-character < style escapes the
	// default encoder emits. The needles are built from bytes so that the test
	// source itself contains no escape sequence to misread.
	for _, escaped := range []string{
		string([]byte{'\\', 'u', '0', '0', '3', 'c'}), // <
		string([]byte{'\\', 'u', '0', '0', '3', 'e'}), // >
		string([]byte{'\\', 'u', '0', '0', '2', '6'}), // &
	} {
		if strings.Contains(out.String(), escaped) {
			t.Errorf("HTML escaping should be disabled, found %q in: %s", escaped, out.String())
		}
	}
	if !strings.Contains(out.String(), `a < b && c > d`) {
		t.Errorf("literal characters should survive encoding, got: %s", out.String())
	}
}

func TestCSVOutputMatchesTableColumns(t *testing.T) {
	r, out, _ := newTestRenderer(false)

	table := NewTable(
		Column{Title: "ADDRESS"},
		Column{Title: "SEVERITY"},
	)
	table.StringRow("aws_s3_bucket.assets", "critical")
	table.StringRow("aws_vpc.main", "low")

	if err := r.Write(View{Table: table}, FormatCSV, ""); err != nil {
		t.Fatalf("Write: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d CSV lines, want 3 (header plus two rows):\n%s", len(lines), out.String())
	}
	if !strings.HasPrefix(lines[0], "ADDRESS,SEVERITY") {
		t.Errorf("header row = %q", lines[0])
	}
}

func TestTableAlignsAroundStyledCells(t *testing.T) {
	// Column widths are computed from display width. Measuring a styled cell
	// with len() would count the invisible escape bytes and misalign the row.
	r, _, _ := newTestRenderer(true)

	table := NewTable(
		Column{Title: "SEVERITY"},
		Column{Title: "ADDRESS"},
	)
	table.Row(Styled("critical", StyleCritical), Text("aws_s3_bucket.assets"))
	table.Row(Styled("low", StyleLow), Text("aws_vpc.main"))

	rendered := r.Render(table)
	for _, line := range strings.Split(strings.TrimSpace(rendered), "\n") {
		stripped := stripANSI(line)
		// Every row must place ADDRESS at the same column.
		if idx := strings.Index(stripped, "aws_"); idx >= 0 && idx != 10 {
			t.Errorf("address column starts at %d, want 10 in line %q", idx, stripped)
		}
	}
}

func TestTruncateRespectsDisplayWidth(t *testing.T) {
	cases := []struct {
		input string
		width int
		want  string
	}{
		{"short", 10, "short"},
		{"aws_s3_bucket.assets", 10, "aws_s3_bu…"},
		{"exact", 5, "exact"},
	}
	for _, tc := range cases {
		got := Truncate(tc.input, tc.width, "…")
		if got != tc.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
		}
		if displayWidth(got) > tc.width {
			t.Errorf("Truncate(%q, %d) produced width %d", tc.input, tc.width, displayWidth(got))
		}
	}
}

func TestDisplayWidthIgnoresANSI(t *testing.T) {
	styled := "\x1b[1;31mcritical\x1b[0m"
	if got := displayWidth(styled); got != 8 {
		t.Errorf("displayWidth of styled 'critical' = %d, want 8", got)
	}
}

func TestApplyIsInertWithoutColor(t *testing.T) {
	r, _, _ := newTestRenderer(false)
	if got := r.Apply(StyleCritical, "critical"); got != "critical" {
		t.Errorf("with colour off Apply must return the text unchanged, got %q", got)
	}
}

func TestNoColorEnvironmentVariableDisablesColor(t *testing.T) {
	// https://no-color.org: any non-empty value disables colour.
	t.Setenv("NO_COLOR", "1")

	out := &bytes.Buffer{}
	if resolveColor(ColorAuto, out) {
		t.Error("NO_COLOR must disable colour in auto mode")
	}
	// An explicit --color=always still wins, which is what a user piping into a
	// pager that understands escapes reaches for.
	if !resolveColor(ColorAlways, out) {
		t.Error("--color=always must override NO_COLOR")
	}
}

func TestDiffMasksSensitiveValues(t *testing.T) {
	r, _, _ := newTestRenderer(false)

	rendered := r.RenderDiff([]DiffLine{
		{Path: "password", Expected: "hunter2", Actual: "swordfish", Sensitive: true},
	})

	if strings.Contains(rendered, "hunter2") || strings.Contains(rendered, "swordfish") {
		t.Fatalf("sensitive values leaked into diff output: %q", rendered)
	}
	if !strings.Contains(rendered, "sensitive value hidden") {
		t.Errorf("diff should say the value was withheld, got %q", rendered)
	}
}

func TestDiffOutputIsDeterministic(t *testing.T) {
	r, _, _ := newTestRenderer(false)

	lines := []DiffLine{
		{Path: "z_last", Expected: 1, Actual: 2},
		{Path: "a_first", Expected: 1, Actual: 2},
		{Path: "m_mid", Expected: nil, Actual: 3},
	}

	first := r.RenderDiff(lines)
	for i := 0; i < 10; i++ {
		if got := r.RenderDiff(lines); got != first {
			t.Fatalf("diff rendering varied between runs:\n%q\n%q", first, got)
		}
	}
}

func TestFormatValueRendersIntegralFloatsAsIntegers(t *testing.T) {
	// JSON numbers decode as float64; a port must read as 443, not 443.0.
	if got := formatValue(float64(443), false); got != "443" {
		t.Errorf("formatValue(443.0) = %q, want 443", got)
	}
	if got := formatValue(1.5, false); got != "1.5" {
		t.Errorf("formatValue(1.5) = %q, want 1.5", got)
	}
}

func TestEmptyTableRendersNothing(t *testing.T) {
	// The caller prints the "no results" message, because only the caller knows
	// what was being searched for.
	r, _, _ := newTestRenderer(false)
	table := NewTable(Column{Title: "A"})
	if got := r.Render(table); got != "" {
		t.Errorf("empty table should render as empty string, got %q", got)
	}
}

func TestNarrowTerminalShrinksTruncatableColumnsOnly(t *testing.T) {
	r, _, _ := newTestRenderer(false)
	r.width = 40

	table := NewTable(
		Column{Title: "ID", MinWidth: 6},
		Column{Title: "DESCRIPTION", Truncatable: true},
	)
	table.StringRow("abc123", strings.Repeat("long ", 30))

	for _, line := range strings.Split(strings.TrimSpace(r.Render(table)), "\n") {
		if displayWidth(line) > 40 {
			t.Errorf("line exceeds terminal width %d: %q (%d cols)", 40, line, displayWidth(line))
		}
	}
}

// stripANSI removes escape sequences so assertions can inspect layout.
func stripANSI(s string) string {
	const (
		stateText = iota
		stateEscape
		stateCSI
	)

	var b strings.Builder
	state := stateText
	for _, r := range s {
		switch state {
		case stateEscape:
			if r == '[' {
				state = stateCSI
				continue
			}
			state = stateText
		case stateCSI:
			if r >= '@' && r <= '~' {
				state = stateText
			}
			continue
		}
		if r == '\x1b' {
			state = stateEscape
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
