package ui

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Format names an output encoding. Human-facing formats render through the
// Renderer; machine formats are byte-exact and never carry ANSI styling, so a
// piped `--output json` is always valid JSON.
type Format string

const (
	// FormatTable is the default aligned human-readable grid.
	FormatTable Format = "table"
	// FormatWide is the table with additional columns that are normally elided.
	FormatWide Format = "wide"
	// FormatJSON is indented JSON.
	FormatJSON Format = "json"
	// FormatYAML is YAML.
	FormatYAML Format = "yaml"
	// FormatCSV is comma-separated values with a header row.
	FormatCSV Format = "csv"
	// FormatTSV is tab-separated values, which pastes cleanly into spreadsheets.
	FormatTSV Format = "tsv"
	// FormatGoTemplate renders through a user-supplied text/template.
	FormatGoTemplate Format = "go-template"
	// FormatName prints only identifiers, one per line, for shell pipelines.
	FormatName Format = "name"
)

// ParseFormat resolves a --output value, returning the format and any argument
// carried after an equals sign, as in `go-template={{.ID}}`.
func ParseFormat(s string) (Format, string, error) {
	name, arg, hasArg := strings.Cut(strings.TrimSpace(s), "=")
	name = strings.ToLower(name)

	switch Format(name) {
	case FormatTable, FormatWide, FormatJSON, FormatYAML, FormatCSV, FormatTSV, FormatName:
		if hasArg {
			return "", "", fmt.Errorf("output format %q takes no argument", name)
		}
		return Format(name), "", nil

	case FormatGoTemplate:
		if !hasArg || arg == "" {
			return "", "", fmt.Errorf("output format go-template requires a template, as in go-template='{{.ID}}'")
		}
		return FormatGoTemplate, arg, nil

	case "":
		return FormatTable, "", nil

	// Common aliases, accepted so that muscle memory from kubectl and docker works.
	case "yml":
		return FormatYAML, "", nil
	case "text", "plain":
		return FormatTable, "", nil

	default:
		return "", "", fmt.Errorf("invalid output format %q (want table, wide, json, yaml, csv, tsv, name, or go-template=TMPL)", name)
	}
}

// IsMachine reports whether the format is intended for another program. Callers
// use this to suppress progress output and decorative headings, which would
// corrupt a machine-readable stream.
func (f Format) IsMachine() bool {
	switch f {
	case FormatJSON, FormatYAML, FormatCSV, FormatTSV, FormatName, FormatGoTemplate:
		return true
	default:
		return false
	}
}

// View is what a command hands to the output layer: one payload that can be
// encoded for machines, plus a table projection for humans. Keeping both on one
// value means the two representations cannot describe different result sets.
type View struct {
	// Data is the structured payload for json, yaml, and go-template output.
	Data any
	// Table is the human projection. It may be nil for commands with no
	// meaningful tabular form.
	Table *Table
	// Names supplies the identifiers for `--output name`.
	Names []string
	// Empty is the message shown when there are no results. It should name what
	// was searched so an empty result is not mistaken for a broken command.
	Empty string
}

// Write encodes the view in the requested format and writes it to stdout.
func (r *Renderer) Write(view View, format Format, arg string) error {
	switch format {
	case FormatJSON:
		return r.writeJSON(view.Data)

	case FormatYAML:
		return r.writeYAML(view.Data)

	case FormatCSV:
		return r.writeSeparated(view.Table, ',')

	case FormatTSV:
		return r.writeSeparated(view.Table, '\t')

	case FormatName:
		for _, name := range view.Names {
			r.Println(name)
		}
		return nil

	case FormatGoTemplate:
		return r.writeTemplate(view.Data, arg)

	default: // FormatTable, FormatWide
		if view.Table == nil || view.Table.Len() == 0 {
			if view.Empty != "" {
				r.Diagnosticf("%s\n", r.Apply(StyleMuted, view.Empty))
			}
			return nil
		}
		r.Raw(r.Render(view.Table))
		return nil
	}
}

// writeJSON emits indented JSON. Encoder is used rather than Marshal so that
// HTML escaping can be disabled: resource ARNs and policy documents contain
// characters that Marshal would mangle into < sequences.
func (r *Renderer) writeJSON(data any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	enc := json.NewEncoder(r.out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("encode json output: %w", err)
	}
	return nil
}

// writeYAML emits YAML at a two-space indent.
func (r *Renderer) writeYAML(data any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	enc := yaml.NewEncoder(r.out)
	enc.SetIndent(2)
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("encode yaml output: %w", err)
	}
	return enc.Close()
}

// writeSeparated emits the table projection as delimited text. Cell text is
// written unstyled, since a CSV consumer has no use for escape sequences.
func (r *Renderer) writeSeparated(table *Table, delim rune) error {
	if table == nil {
		return fmt.Errorf("this command does not support delimited output")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	w := csv.NewWriter(r.out)
	w.Comma = delim

	header := make([]string, len(table.columns))
	for i, col := range table.columns {
		header[i] = col.Title
	}
	if err := w.Write(header); err != nil {
		return fmt.Errorf("write header row: %w", err)
	}

	for _, row := range table.rows {
		record := make([]string, len(row))
		for i, cell := range row {
			record[i] = cell.Text
		}
		if err := w.Write(record); err != nil {
			return fmt.Errorf("write data row: %w", err)
		}
	}

	w.Flush()
	return w.Error()
}

// writeTemplate renders the payload through a user-supplied Go template.
func (r *Renderer) writeTemplate(data any, tmpl string) error {
	t, err := template.New("output").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("parse go-template: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := t.Execute(r.out, data); err != nil {
		return fmt.Errorf("execute go-template: %w", err)
	}
	_, _ = fmt.Fprintln(r.out)
	return nil
}
