package render

import (
	"strings"
	"testing"
)

func TestQuote(t *testing.T) {
	cases := map[string]string{
		"plain":        "plain",
		"https://a.io": "https://a.io",
		"":             `""`,
		"a, b":         `"a, b"`,
		`say "hi"`:     `"say \"hi\""`,
		"two\nlines":   `"two\nlines"`,
		" padded":      `" padded"`,
	}

	for value, want := range cases {
		if got := quote(value); got != want {
			t.Errorf("quote(%q) = %s, want %s", value, got, want)
		}
	}
}

func TestACompactListWithNoRowsStillHasItsHeader(t *testing.T) {
	view := View{ShowCount: true, Lists: []List{{Name: "monitors", Columns: []string{"id", "name"}}}}

	var out strings.Builder
	view.Compact(&out)

	if want := "count: 0\nmonitors[0]{id,name}:\n"; out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}

func TestPaintAddsColorOnlyWhenOn(t *testing.T) {
	if got := NewStyle(true, Basic).Paint(Green, "up"); got != "\x1b[32mup\x1b[0m" {
		t.Errorf("on: got %q", got)
	}
	if got := NewStyle(false, Basic).Paint(Green, "up"); got != "up" {
		t.Errorf("off: got %q", got)
	}
	if got := NewStyle(true, Basic).Paint(Plain, "up"); got != "up" {
		t.Errorf("plain: got %q", got)
	}
}

func TestAlignPadsByVisibleWidth(t *testing.T) {
	rows := [][]Cell{
		{{Text: "NAME"}, {Text: "STATUS"}},
		{{Text: "Checkout"}, {Text: "Down", Color: Red}},
		{{Text: "Blog"}, {Text: "Up", Color: Green}},
	}

	plain := NewStyle(false, Basic).Align(rows)
	want := []string{"NAME      STATUS", "Checkout  Down", "Blog      Up"}
	for i := range want {
		if plain[i] != want[i] {
			t.Errorf("plain line %d = %q, want %q", i, plain[i], want[i])
		}
	}

	painted := NewStyle(true, Basic).Align(rows)
	if painted[1] != "Checkout  \x1b[31mDown\x1b[0m" {
		t.Errorf("painted line = %q", painted[1])
	}
}

func TestPaintUsesTheBrandColorsOnATerminalThatShowsThem(t *testing.T) {
	cases := []struct {
		depth Depth
		color Color
		want  string
	}{
		{TrueColor, Green, "\x1b[38;2;61;220;132mup\x1b[0m"},
		{TrueColor, Red, "\x1b[38;2;229;72;77mup\x1b[0m"},
		{Ansi256, Green, "\x1b[38;5;78mup\x1b[0m"},
		{Ansi256, Red, "\x1b[38;5;167mup\x1b[0m"},
		{Basic, Green, "\x1b[32mup\x1b[0m"},
		{TrueColor, Yellow, "\x1b[38;2;240;166;60mup\x1b[0m"},
		{TrueColor, Gray, "\x1b[38;2;111;128;120mup\x1b[0m"},
		{Ansi256, Yellow, "\x1b[38;5;215mup\x1b[0m"},
		{Ansi256, Gray, "\x1b[38;5;243mup\x1b[0m"},
		{Basic, Yellow, "\x1b[33mup\x1b[0m"},
		{Basic, Gray, "\x1b[90mup\x1b[0m"},
	}

	for _, tc := range cases {
		if got := NewStyle(true, tc.depth).Paint(tc.color, "up"); got != tc.want {
			t.Errorf("depth %d color %d: got %q, want %q", tc.depth, tc.color, got, tc.want)
		}
	}
}
