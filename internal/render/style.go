package render

import (
	"strings"
	"unicode/utf8"
)

// Color is a named color. Gray and Yellow are standard terminal colors, so the
// person's theme picks the shade. Green and Red are the brand colors, which a
// terminal with more colors shows exactly.
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

// Depth is how many colors a terminal shows.
type Depth int

// The depths, from fewest colors to most.
const (
	Basic Depth = iota
	Ansi256
	TrueColor
)

// codes is the escape parameter for a color at each depth.
type codes struct {
	basic, ansi256, trueColor string
}

// The brand colors are #3DDC84 and #E5484D from the website's dark theme. Truecolor shows them exactly;
// 256-color uses the nearest palette entries, 78 (#5fd787) and 167 (#d75f5f).
var colorCodes = map[Color]codes{
	Green:  {"32", "38;5;78", "38;2;61;220;132"},
	Red:    {"31", "38;5;167", "38;2;229;72;77"},
	Yellow: {"33", "33", "33"},
	Gray:   {"90", "90", "90"},
}

// Style paints text. A Style that is off returns the text unchanged.
type Style struct {
	on    bool
	depth Depth
}

// NewStyle returns a Style that paints at the given depth only when on is true.
func NewStyle(on bool, depth Depth) Style {
	return Style{on: on, depth: depth}
}

// Paint wraps text in the color's escape codes.
func (s Style) Paint(color Color, text string) string {
	code, known := colorCodes[color]
	if !s.on || !known || text == "" {
		return text
	}

	parameter := code.basic
	switch s.depth {
	case Ansi256:
		parameter = code.ansi256
	case TrueColor:
		parameter = code.trueColor
	}

	return "\x1b[" + parameter + "m" + text + "\x1b[0m"
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
