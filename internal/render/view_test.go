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
	if got := NewStyle(true).Paint(Green, "up"); got != "\x1b[32mup\x1b[0m" {
		t.Errorf("on: got %q", got)
	}
	if got := NewStyle(false).Paint(Green, "up"); got != "up" {
		t.Errorf("off: got %q", got)
	}
	if got := NewStyle(true).Paint(Plain, "up"); got != "up" {
		t.Errorf("plain: got %q", got)
	}
}

func TestAlignPadsByVisibleWidth(t *testing.T) {
	rows := [][]Cell{
		{{Text: "NAME"}, {Text: "STATUS"}},
		{{Text: "Checkout"}, {Text: "Down", Color: Red}},
		{{Text: "Blog"}, {Text: "Up", Color: Green}},
	}

	plain := NewStyle(false).Align(rows)
	want := []string{"NAME      STATUS", "Checkout  Down", "Blog      Up"}
	for i := range want {
		if plain[i] != want[i] {
			t.Errorf("plain line %d = %q, want %q", i, plain[i], want[i])
		}
	}

	painted := NewStyle(true).Align(rows)
	if painted[1] != "Checkout  \x1b[31mDown\x1b[0m" {
		t.Errorf("painted line = %q", painted[1])
	}
}
