package render

import (
	"strings"
	"unicode/utf8"
)

// Color is one of the 16 standard terminal colors. The person's theme picks
// the exact shade, so the output reads on a light or a dark background.
type Color int

// The colors a screen uses.
const (
	Plain Color = iota
	Green
	Red
	Yellow
	Gray
)

// Marks that start a result line.
const (
	MarkDone   = "✓"
	MarkFailed = "✗"
)

var colorCodes = map[Color]string{
	Green:  "32",
	Red:    "31",
	Yellow: "33",
	Gray:   "90",
}

// Style paints text. A Style that is off returns the text unchanged.
type Style struct {
	on bool
}

// NewStyle returns a Style that paints only when on is true.
func NewStyle(on bool) Style {
	return Style{on: on}
}

// Paint wraps text in the color's escape codes.
func (s Style) Paint(color Color, text string) string {
	code, known := colorCodes[color]
	if !s.on || !known || text == "" {
		return text
	}

	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

// Cell is one value in a column.
type Cell struct {
	Text  string
	Color Color
}

// Align lays rows out in columns two spaces apart and returns one line per row. It
// pads by the width of the plain text before it paints, so color codes never break
// the alignment. The last column is not padded.
func (s Style) Align(rows [][]Cell) []string {
	widths := columnWidths(rows)
	lines := make([]string, len(rows))

	for i, row := range rows {
		var line strings.Builder

		for column, cell := range row {
			line.WriteString(s.Paint(cell.Color, cell.Text))

			if column < len(row)-1 {
				padding := widths[column] - utf8.RuneCountInString(cell.Text) + columnGap
				line.WriteString(strings.Repeat(" ", padding))
			}
		}

		lines[i] = line.String()
	}

	return lines
}

const columnGap = 2

func columnWidths(rows [][]Cell) []int {
	var widths []int

	for _, row := range rows {
		for column, cell := range row {
			if column == len(widths) {
				widths = append(widths, 0)
			}
			widths[column] = max(widths[column], utf8.RuneCountInString(cell.Text))
		}
	}

	return widths
}
