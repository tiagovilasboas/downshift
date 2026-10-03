package task

import (
	"testing"
	"time"
)

func TestNext(t *testing.T) {
	base := time.Date(2026, 3, 14, 10, 7, 30, 0, time.UTC) // Saturday
	cases := map[string]time.Time{
		"* * * * *":      time.Date(2026, 3, 14, 10, 8, 0, 0, time.UTC),
		"*/15 * * * *":   time.Date(2026, 3, 14, 10, 15, 0, 0, time.UTC),
		"0 9 * * *":      time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC),
		"30 8 * * 1-5":   time.Date(2026, 3, 16, 8, 30, 0, 0, time.UTC),
		"0 0 1 * *":      time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		"0 12 29 2 *":    time.Date(2028, 2, 29, 12, 0, 0, 0, time.UTC),
		"5,10 10 * * *":  time.Date(2026, 3, 14, 10, 10, 0, 0, time.UTC),
		"0 0 13 * 5":     time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC), // Friday OR the 13th
		"0 1-10/3 * * *": time.Date(2026, 3, 15, 1, 0, 0, 0, time.UTC),
		"0 0 * 12 *":     time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
	}
	for spec, want := range cases {
		got, err := Next(spec, base)
		if err != nil || !got.Equal(want) {
			t.Errorf("Next(%q)=%v,%v want %v", spec, got, err, want)
		}
	}
	exact := time.Date(2026, 3, 14, 10, 15, 0, 0, time.UTC)
	if got, _ := Next("15 10 * * *", exact); !got.Equal(time.Date(2026, 3, 15, 10, 15, 0, 0, time.UTC)) {
		t.Errorf("must be strictly after: %v", got)
	}
	for _, bad := range []string{"* * * *", "60 * * * *", "* 24 * * *", "*/0 * * * *", "a * * * *", "5-1 * * * *", "* * 0 * *"} {
		if _, err := Next(bad, base); err == nil {
			t.Errorf("Next(%q): expected error", bad)
		}
	}
}
