package cli

import (
	"testing"
	"time"
)

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		when time.Time
		want string
	}{
		{"now", now, "just now"},
		{"a few seconds is now", now.Add(-4 * time.Second), "just now"},
		{"seconds", now.Add(-30 * time.Second), "30 seconds ago"},
		{"one minute", now.Add(-time.Minute), "1 minute ago"},
		{"minutes", now.Add(-48 * time.Minute), "48 minutes ago"},
		{"one hour in the future", now.Add(time.Hour), "in 1 hour"},
		{"hours", now.Add(-5 * time.Hour), "5 hours ago"},
		{"days", now.Add(-48 * time.Hour), "2 days ago"},
		{"exactly 30 days", now.Add(-30 * 24 * time.Hour), "30 days ago"},
		{"over 30 days, this year", time.Date(2026, 9, 1, 8, 30, 0, 0, time.UTC), "Sep 1 08:30"},
		{"over 30 days, another year", time.Date(2025, 9, 1, 8, 30, 0, 0, time.UTC), "Sep 1 2025"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := relativeTime(now, tc.when); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAbsoluteTimeUsesTheClocksZone(t *testing.T) {
	zone := time.FixedZone("test", 2*60*60)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, zone)
	when := time.Date(2026, 11, 1, 16, 0, 0, 0, time.UTC) // 18:00 in the test zone

	if got := absoluteTime(now, when); got != "18:00" {
		t.Errorf("same day: got %q, want 18:00", got)
	}

	tomorrow := when.Add(24 * time.Hour)
	if got := absoluteTime(now, tomorrow); got != "Nov 2 18:00" {
		t.Errorf("next day: got %q, want Nov 2 18:00", got)
	}
}
