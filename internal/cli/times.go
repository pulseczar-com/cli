package cli

import (
	"fmt"
	"time"
)

const (
	justNow     = 5 * time.Second
	relativeMax = 30 * 24 * time.Hour
)

// relativeTime says how far a time is from now: "2 minutes ago", "in 1 hour". A time
// more than 30 days away shows as a date.
func relativeTime(now, when time.Time) string {
	distance := now.Sub(when)
	future := distance < 0
	if future {
		distance = -distance
	}

	if distance < justNow {
		return "just now"
	}
	if distance > relativeMax {
		return absoluteTime(now, when)
	}

	span := countUnits(distance)
	if future {
		return "in " + span
	}

	return span + " ago"
}

func countUnits(distance time.Duration) string {
	switch {
	case distance < time.Minute:
		return plural(int(distance/time.Second), "second")
	case distance < time.Hour:
		return plural(int(distance/time.Minute), "minute")
	case distance < 24*time.Hour:
		return plural(int(distance/time.Hour), "hour")
	default:
		return plural(int(distance/(24*time.Hour)), "day")
	}
}

func plural(count int, unit string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", unit)
	}

	return fmt.Sprintf("%d %ss", count, unit)
}

// absoluteTime shows a time in the clock's own zone: the hour for today, the day and
// hour for this year, the date for any other year.
func absoluteTime(now, when time.Time) string {
	when = when.In(now.Location())

	switch {
	case sameDay(now, when):
		return when.Format("15:04")
	case now.Year() == when.Year():
		return when.Format("Jan 2 15:04")
	default:
		return when.Format("Jan 2 2006")
	}
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()

	return ay == by && am == bm && ad == bd
}
