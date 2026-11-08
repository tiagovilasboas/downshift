// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"sort"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// Period granularities of `downshift report --daily` and `--history`.
const (
	PerDay  = "day"
	PerWeek = "week" // ISO weeks, labelled by their Monday
)

// PeriodHarness is one harness inside one period, in tokens only. The two
// figures are never added: compaction removes tokens from the context, routing
// only runs billable tokens below the session model.
type PeriodHarness struct {
	Harness string `json:"harness"`
	// CompactionTokens is the Compactor's saved bytes / execwrap.BytesPerToken.
	CompactionTokens int64 `json:"compaction_tokens"`
	// RoutingBelowTokens is the billable tokens of spawns that ran below the
	// session model; unobserved spawns are not counted.
	RoutingBelowTokens int64 `json:"routing_below_tokens"`
}

// PeriodRow is one day or week. Period is "2006-01-02" (the day, or the Monday
// of the week) in the caller's time zone.
type PeriodRow struct {
	Period    string          `json:"period"`
	Harnesses []PeriodHarness `json:"harnesses"`
}

// PeriodOptions selects the buckets.
type PeriodOptions struct {
	Per string // PerDay or PerWeek
	// Days limits the window to the last Days calendar days, today included;
	// 0 means the whole retained history.
	Days int
	// Fill emits a row for every day of the window, even without activity.
	Fill bool
	Now  time.Time
	Loc  *time.Location
}

func (o PeriodOptions) loc() *time.Location {
	if o.Loc == nil {
		return time.UTC
	}
	return o.Loc
}

// periodKey is the bucket label of t.
func (o PeriodOptions) periodKey(t time.Time) string {
	t = t.In(o.loc())
	y, m, d := t.Date()
	if o.Per == PerWeek {
		d -= (int(t.Weekday()) + 6) % 7 // back to Monday
	}
	return time.Date(y, m, d, 12, 0, 0, 0, o.loc()).Format("2006-01-02")
}

// windowStart is the first instant of the window, or the zero time for all.
func (o PeriodOptions) windowStart() time.Time {
	if o.Days <= 0 {
		return time.Time{}
	}
	y, m, d := o.Now.In(o.loc()).Date()
	return time.Date(y, m, d-(o.Days-1), 0, 0, 0, 0, o.loc())
}

func parseStamp(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, true
	}
	t, err := time.Parse(time.RFC3339, s)
	return t, err == nil
}

// BuildPeriods groups the routing tokens and the Compactor tokens by period
// and harness. Pure; reads only counts. A timestamp that does not parse is
// outside every window.
func BuildPeriods(events []telemetry.Event, execEvents []execwrap.Event, o PeriodOptions) []PeriodRow {
	start := o.windowStart()
	in := func(t time.Time) bool { return start.IsZero() || !t.Before(start) }

	byPeriod := map[string][]telemetry.Event{}
	var decisions []telemetry.Event
	for _, ev := range events {
		if telemetry.IsDecisionRow(ev) {
			decisions = append(decisions, ev)
		}
		if t, ok := parseStamp(ev.Timestamp); ok && in(t) && ev.Outcome == telemetry.OutcomeUsage {
			k := o.periodKey(t)
			byPeriod[k] = append(byPeriod[k], ev)
		}
	}
	cells := map[string]map[string]*PeriodHarness{}
	cell := func(p, h string) *PeriodHarness {
		if cells[p] == nil {
			cells[p] = map[string]*PeriodHarness{}
		}
		if cells[p][h] == nil {
			cells[p][h] = &PeriodHarness{Harness: h}
		}
		return cells[p][h]
	}
	for p, evs := range byPeriod {
		// Decisions of any day link the usage rows of this period.
		all := append(append([]telemetry.Event{}, evs...), decisions...)
		for h, r := range telemetry.RoutingTokens(all).ByHarness {
			if r.Spawns > 0 {
				cell(p, h).RoutingBelowTokens = r.BelowBillable
			}
		}
	}
	addCompaction(execEvents, o, in, cell)
	if o.Fill && o.Days > 0 {
		y, m, d := o.Now.In(o.loc()).Date()
		for i := 0; i < o.Days; i++ {
			k := o.periodKey(time.Date(y, m, d-i, 12, 0, 0, 0, o.loc()))
			if cells[k] == nil {
				cells[k] = map[string]*PeriodHarness{}
			}
		}
	}
	return sortedRows(cells)
}

// addCompaction folds the Compactor's saved bytes into the cells, converting
// to tokens once per cell so rounding does not accumulate.
func addCompaction(execEvents []execwrap.Event, o PeriodOptions, in func(time.Time) bool, cell func(p, h string) *PeriodHarness) {
	saved := map[[2]string]int64{}
	for _, ev := range execEvents {
		t, ok := parseStamp(ev.TS)
		if !ok || !in(t) || ev.RawBytes <= ev.EmittedBytes {
			continue
		}
		h := ev.Harness
		if h == "" {
			h = UnknownHarness
		}
		saved[[2]string{o.periodKey(t), h}] += ev.RawBytes - ev.EmittedBytes
	}
	for k, b := range saved {
		cell(k[0], k[1]).CompactionTokens = b / execwrap.BytesPerToken
	}
}

func sortedRows(cells map[string]map[string]*PeriodHarness) []PeriodRow {
	out := make([]PeriodRow, 0, len(cells))
	for p, hs := range cells {
		row := PeriodRow{Period: p, Harnesses: []PeriodHarness{}}
		for _, c := range hs {
			row.Harnesses = append(row.Harnesses, *c)
		}
		sort.Slice(row.Harnesses, func(i, j int) bool { return row.Harnesses[i].Harness < row.Harnesses[j].Harness })
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Period < out[j].Period })
	return out
}
