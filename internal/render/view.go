// Package render writes the compact axi form for a pipe, and holds the color and
// column helpers the terminal screens use.
package render

import (
	"fmt"
	"io"
	"strings"
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

// View is a command's result in the compact form. A detail has fields and may have
// lists. A top-level list has exactly one list and sets ShowCount, so the compact
// form leads with `count: N`; its fields, such as a paging cursor, follow the list.
type View struct {
	Fields    []Field
	Lists     []List
	ShowCount bool
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
