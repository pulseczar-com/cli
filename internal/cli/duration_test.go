package cli

import "testing"

func TestParseSeconds(t *testing.T) {
	cases := []struct {
		text    string
		seconds int
		valid   bool
	}{
		{"90s", 90, true},
		{"5m", 300, true},
		{"2h", 7200, true},
		{"1d", 86400, true},
		{"0s", 0, true},
		{"", 0, false},
		{"m", 0, false},
		{"5", 0, false},
		{"5x", 0, false},
		{"-5m", 0, false},
		{"1.5h", 0, false},
		{"five minutes", 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			seconds, err := parseSeconds(tc.text)

			if (err == nil) != tc.valid || seconds != tc.seconds {
				t.Errorf("parseSeconds(%q) = %d, %v", tc.text, seconds, err)
			}
		})
	}
}

func TestFormatSeconds(t *testing.T) {
	cases := map[int]string{60: "1m", 300: "5m", 3600: "1h", 5400: "90m", 86400: "1d", 90: "90s", 59: "59s"}

	for seconds, want := range cases {
		if got := formatSeconds(seconds); got != want {
			t.Errorf("formatSeconds(%d) = %q, want %q", seconds, got, want)
		}
	}
}
