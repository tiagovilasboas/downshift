package task

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func field(s string, lo, hi int) (map[int]bool, bool, error) {
	set := map[int]bool{}
	for _, part := range strings.Split(s, ",") {
		rng, stepStr, hasStep := strings.Cut(part, "/")
		step := 1
		if hasStep {
			n, err := strconv.Atoi(stepStr)
			if err != nil || n <= 0 {
				return nil, false, fmt.Errorf("bad step %q", part)
			}
			step = n
		}
		a, b := lo, hi
		if rng != "*" {
			x, y, isRange := strings.Cut(rng, "-")
			var err error
			if a, err = strconv.Atoi(x); err != nil {
				return nil, false, fmt.Errorf("bad value %q", part)
			}
			b = a
			if isRange {
				if b, err = strconv.Atoi(y); err != nil {
					return nil, false, fmt.Errorf("bad value %q", part)
				}
			} else if hasStep {
				b = hi
			}
		}
		if a < lo || b > hi || a > b {
			return nil, false, fmt.Errorf("out of range %q", part)
		}
		for v := a; v <= b; v += step {
			set[v] = true
		}
	}
	return set, s != "*", nil
}

func Next(spec string, after time.Time) (time.Time, error) {
	f := strings.Fields(spec)
	if len(f) != 5 {
		return time.Time{}, fmt.Errorf("want 5 fields, got %d", len(f))
	}
	bounds := [5][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 6}}
	var sets [5]map[int]bool
	var restricted [5]bool
	for i := range f {
		var err error
		if sets[i], restricted[i], err = field(f[i], bounds[i][0], bounds[i][1]); err != nil {
			return time.Time{}, err
		}
	}
	t := after.Truncate(time.Minute).Add(time.Minute)
	limit := t.AddDate(5, 0, 0)
	for t.Before(limit) {
		if !sets[3][int(t.Month())] {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}
		dom, dow := sets[2][t.Day()], sets[4][int(t.Weekday())]
		dayOK := dom && dow
		if restricted[2] && restricted[4] {
			dayOK = dom || dow
		}
		if !dayOK {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !sets[1][t.Hour()] {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}
		if sets[0][t.Minute()] {
			return t, nil
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}, errors.New("no matching time within 5 years")
}
