// Package render turns a command's result into the two human forms: an aligned
// table for a terminal, and the compact axi form for a pipe.
package render

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Field is one `key: value` line.
type Field struct {
	Key   string
	Value string
}

// List is rows under named columns.
type List struct {
	Name    string
	Columns []string
	Rows    [][]string
}

// View is a command's result. A detail has fields and may have lists. A top-level
// list has exactly one list and sets ShowCount, so the compact form leads with
// `count: N`; its fields, such as a paging cursor, follow the list.
type View struct {
	Fields    []Field
	Lists     []List
	ShowCount bool
}

// Table writes the view for a person at a terminal.
func (v View) Table(w io.Writer) {
	if !v.ShowCount {
		writeFieldsAligned(w, v.Fields)
	}

	for _, list := range v.Lists {
		if len(list.Rows) == 0 {
			if v.ShowCount {
				fmt.Fprintf(w, "No %s.\n", list.Name)
			}
			continue
		}

		if !v.ShowCount {
			fmt.Fprintf(w, "\n%s\n", list.Name)
		}
		writeTable(w, list)
	}

	if v.ShowCount && len(v.Fields) > 0 {
		fmt.Fprintln(w)
		writeFieldsAligned(w, v.Fields)
	}
}

// Compact writes the view in the axi form: plain `key: value` lines, and each list
// as `name[N]{columns}:` followed by one indented row per item.
func (v View) Compact(w io.Writer) {
	if v.ShowCount {
		fmt.Fprintf(w, "count: %d\n", len(v.Lists[0].Rows))
	}

	if !v.ShowCount {
		writeFieldsCompact(w, v.Fields)
	}

	for _, list := range v.Lists {
		fmt.Fprintf(w, "%s[%d]{%s}:\n", list.Name, len(list.Rows), strings.Join(list.Columns, ","))
		for _, row := range list.Rows {
			fmt.Fprintf(w, "  %s\n", joinQuoted(row))
		}
	}

	if v.ShowCount {
		writeFieldsCompact(w, v.Fields)
	}
}

func writeFieldsCompact(w io.Writer, fields []Field) {
	for _, field := range fields {
		fmt.Fprintf(w, "%s: %s\n", field.Key, quote(field.Value))
	}
}

func writeFieldsAligned(w io.Writer, fields []Field) {
	width := 0
	for _, field := range fields {
		width = max(width, len(field.Key))
	}

	for _, field := range fields {
		fmt.Fprintf(w, "%-*s  %s\n", width, field.Key, field.Value)
	}
}

func writeTable(w io.Writer, list List) {
	tab := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	headers := make([]string, len(list.Columns))
	for i, column := range list.Columns {
		headers[i] = strings.ToUpper(strings.ReplaceAll(column, "_", " "))
	}
	fmt.Fprintln(tab, strings.Join(headers, "\t"))

	for _, row := range list.Rows {
		fmt.Fprintln(tab, strings.Join(row, "\t"))
	}

	tab.Flush()
}

func joinQuoted(row []string) string {
	cells := make([]string, len(row))
	for i, cell := range row {
		cells[i] = quote(cell)
	}

	return strings.Join(cells, ",")
}

// quote wraps a value that would break the compact form: an empty one, one with a
// comma, a quote or a newline, or one with a space at either end.
func quote(value string) string {
	needsQuotes := value == "" ||
		strings.ContainsAny(value, ",\"\n") ||
		value != strings.TrimSpace(value)
	if !needsQuotes {
		return value
	}

	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(value)

	return `"` + escaped + `"`
}
