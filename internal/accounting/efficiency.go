// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"sort"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Efficiency is, per harness, how much work finished without premium
// capacity: "more work done, less premium intelligence consumed". Built only
// from logged routing, usage and end records. Counts and shares: no tokens
// (the Compactor prints those once), no money, never summed across harnesses.
type Efficiency struct {
	Scope     Scope               `json:"scope"`
	Harnesses []HarnessEfficiency `json:"harnesses"`
}

// TierCount is a number of spawns on one ran-on tier and its share (percent)
// of all routed spawns of the harness.
type TierCount struct {
	Count int     `json:"count"`
	Pct   float64 `json:"pct"`
}

// SpawnsByTier splits all routed spawns of a harness by the tier they ran on.
type SpawnsByTier struct {
	Small    TierCount `json:"small"`
	Mid      TierCount `json:"mid"`
	Frontier TierCount `json:"frontier"`
	Unknown  TierCount `json:"unknown"`
}

// HarnessEfficiency is one harness over the window.
type HarnessEfficiency struct {
	Harness string `json:"harness"`
	// Spawns is the routed spawns, split by the tier each ran on.
	Spawns       int          `json:"spawns"`
	SpawnsByTier SpawnsByTier `json:"spawns_by_tier"`
	// Ended, Completed, Reworked and ReworkRatio are the routed table's own
	// figures (telemetry.GroupSummary): Completed is completed tasks with
	// usage observed, the population of the premium figure below.
	Ended       int     `json:"ended"`
	Completed   int     `json:"completed_tasks"`
	Reworked    int     `json:"reworked"`
	ReworkRatio float64 `json:"rework_ratio"`
	// Of the Completed tasks: those whose ran-on tier is known, the ones that
	// ran on a non-frontier tier, and the ones whose tier is unknown (left out
	// of the ratio). PremiumPreservedPct is NonFrontier / TierKnown and is
	// meaningful only when PremiumObserved; false reads "unobserved", never 0.
	TierKnown           int     `json:"completed_tier_known"`
	NonFrontier         int     `json:"completed_non_frontier"`
	TierUnknown         int     `json:"completed_tier_unknown"`
	PremiumObserved     bool    `json:"premium_observed"`
	PremiumPreservedPct float64 `json:"premium_preserved_pct"`
}

// BuildEfficiency folds the routed spawns of scope per harness, in name order.
// Pure: the caller reads the event log. The window and the spawn join are the
// report's own (scopeTelemetry, telemetry.BuildReport / BuildTasks), so the
// completed, ended and rework figures are the routed table's.
func BuildEfficiency(events []telemetry.Event, scope Scope) Efficiency {
	scoped := scopeTelemetry(events, scope)
	out := Efficiency{Scope: scope, Harnesses: []HarnessEfficiency{}}
	byHarness := map[string][]telemetry.Event{}
	for _, ev := range scoped {
		if ev.Harness != "" {
			byHarness[ev.Harness] = append(byHarness[ev.Harness], ev)
		}
	}
	tiers := telemetry.RanTiers(scoped)
	for h, evs := range byHarness {
		if row, ok := efficiencyOf(h, evs, tiers); ok {
			out.Harnesses = append(out.Harnesses, row)
		}
	}
	sort.Slice(out.Harnesses, func(i, j int) bool { return out.Harnesses[i].Harness < out.Harnesses[j].Harness })
	return out
}

// efficiencyOf is one harness; false when it has no routed spawn.
func efficiencyOf(harness string, evs []telemetry.Event, tiers map[telemetry.SpawnRef]telemetry.RanTier) (HarnessEfficiency, bool) {
	rep := telemetry.BuildReport(evs, 0)
	g := rep.Routed
	row := HarnessEfficiency{Harness: harness, Spawns: g.Spawns, Ended: g.Ended, Completed: g.Completed,
		Reworked: g.Reworked, ReworkRatio: g.ReworkRate}
	if g.Spawns == 0 {
		return row, false
	}
	for _, t := range telemetry.BuildTasks(evs) {
		if t.Group != telemetry.GroupRouted {
			continue
		}
		tier := tiers[telemetry.SpawnRef{Harness: harness, Key: t.Key}]
		if tier == "" {
			tier = telemetry.RanUnknown
		}
		row.SpawnsByTier.add(tier)
		if !t.Completed() || !t.Observed {
			continue
		}
		switch tier {
		case telemetry.RanUnknown:
			row.TierUnknown++
		case telemetry.RanFrontier:
			row.TierKnown++
		default:
			row.TierKnown++
			row.NonFrontier++
		}
	}
	row.SpawnsByTier.setPct(row.Spawns)
	if row.TierKnown > 0 {
		row.PremiumObserved = true
		row.PremiumPreservedPct = pct(row.NonFrontier, row.TierKnown)
	}
	return row, true
}

func (s *SpawnsByTier) add(t telemetry.RanTier) {
	switch t {
	case telemetry.RanSmall:
		s.Small.Count++
	case telemetry.RanMid:
		s.Mid.Count++
	case telemetry.RanFrontier:
		s.Frontier.Count++
	default:
		s.Unknown.Count++
	}
}

func (s *SpawnsByTier) setPct(total int) {
	s.Small.Pct = pct(s.Small.Count, total)
	s.Mid.Pct = pct(s.Mid.Count, total)
	s.Frontier.Pct = pct(s.Frontier.Count, total)
	s.Unknown.Pct = pct(s.Unknown.Count, total)
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) * 100 / float64(d)
}
