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

func TestATableListWithNoRowsSaysSo(t *testing.T) {
	view := View{ShowCount: true, Lists: []List{{Name: "monitors", Columns: []string{"id"}}}}

	var out strings.Builder
	view.Table(&out)

	if want := "No monitors.\n"; out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}
