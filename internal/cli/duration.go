package cli

import (
	"fmt"
	"strconv"
)

var durationUnits = map[byte]int{'s': 1, 'm': 60, 'h': 3600, 'd': 86400}

// parseSeconds reads a length such as 90s, 5m, 2h or 1d and returns whole seconds.
func parseSeconds(text string) (int, error) {
	if len(text) < 2 {
		return 0, fmt.Errorf("%q is not a length; use a number and s, m, h or d, such as 5m", text)
	}

	unit, ok := durationUnits[text[len(text)-1]]
	count, err := strconv.Atoi(text[:len(text)-1])
	if !ok || err != nil || count < 0 {
		return 0, fmt.Errorf("%q is not a length; use a number and s, m, h or d, such as 5m", text)
	}

	return count * unit, nil
}

// formatSeconds is the inverse of parseSeconds for whole units: 300 is 5m, 86400 is 1d.
func formatSeconds(seconds int) string {
	for _, unit := range []struct {
		letter string
		size   int
	}{{"d", 86400}, {"h", 3600}, {"m", 60}} {
		if seconds%unit.size == 0 {
			return fmt.Sprintf("%d%s", seconds/unit.size, unit.letter)
		}
	}

	return fmt.Sprintf("%ds", seconds)
}
