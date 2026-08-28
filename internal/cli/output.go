package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// FormatTable formats data into a table string
func FormatTable(headers []string, rows [][]string) string {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)

	// Write headers
	fmt.Fprintln(w, strings.Join(headers, "\t"))

	// Write separator
	var sep []string
	for _, h := range headers {
		sep = append(sep, strings.Repeat("-", len(h)))
	}
	fmt.Fprintln(w, strings.Join(sep, "\t"))

	// Write rows
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}

	w.Flush()
	return buf.String()
}

// FormatJSON formats data into a JSON string
func FormatJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return string(b)
}

// FormatYAML formats data into a YAML string
func FormatYAML(v any) string {
	b, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Sprintf("Error formatting YAML: %v", err)
	}
	return string(b)
}

// PrintOutput prints output in the specified format
func PrintOutput(format string, v any, tableHeaders []string, tableRows [][]string) {
	switch format {
	case "json":
		fmt.Println(FormatJSON(v))
	case "yaml", "yml":
		fmt.Println(FormatYAML(v))
	case "table":
		fallthrough
	default:
		fmt.Print(FormatTable(tableHeaders, tableRows))
	}
}
