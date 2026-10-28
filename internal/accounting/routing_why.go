// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"sort"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// topReasonLimit bounds the reasons listed per harness.
const topReasonLimit = 5

// WhyCount is one key and how many decisions carry it.
type WhyCount struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// WhyHarness is the routing note of one harness over the window: how many
// decisions, how each verdict decided, and the reasons behind them. Built from
// the routing notes the events hold. Decisions with no note are counted
// apart, never guessed.
type WhyHarness struct {
	Harness     string     `json:"harness"`
	Decisions   int        `json:"decisions"`
	Recorded    int        `json:"recorded"`
	NotRecorded int        `json:"not_recorded"`
	ByVerdict   []WhyCount `json:"by_verdict"`
	TopReasons  []WhyCount `json:"top_reasons"`
}

// RoutingWhy answers "why did spawns change, or not" for one window. No
// money, no prompt: enums, tiers and declared ids only.
type RoutingWhy struct {
	Scope     Scope        `json:"scope"`
	Decisions int          `json:"decisions"`
	Harnesses []WhyHarness `json:"harnesses"`
}

// BuildRoutingWhy folds the decision rows of scope into a RoutingWhy. Pure:
// the caller reads the event log.
func BuildRoutingWhy(events []telemetry.Event, scope Scope) RoutingWhy {
	out := RoutingWhy{Scope: scope, Harnesses: []WhyHarness{}}
	type acc struct {
		h        WhyHarness
		verdicts map[string]int
		reasons  map[string]int
	}
	byHarness := map[string]*acc{}
	for _, ev := range scopeTelemetry(events, scope) {
		if !telemetry.IsDecisionRow(ev) {
			continue
		}
		hname := ev.Harness
		a := byHarness[hname]
		if a == nil {
			a = &acc{h: WhyHarness{Harness: hname}, verdicts: map[string]int{}, reasons: map[string]int{}}
			byHarness[hname] = a
		}
		a.h.Decisions++
		out.Decisions++
		note, ok := noteOf(ev)
		if !ok {
			a.h.NotRecorded++
			continue
		}
		a.h.Recorded++
		a.verdicts[note.Verdict]++
		a.reasons[reasonKey(note)]++
	}
	for _, a := range byHarness {
		a.h.ByVerdict = sortedCounts(a.verdicts, 0)
		a.h.TopReasons = sortedCounts(a.reasons, topReasonLimit)
		out.Harnesses = append(out.Harnesses, a.h)
	}
	sort.Slice(out.Harnesses, func(i, j int) bool { return out.Harnesses[i].Harness < out.Harnesses[j].Harness })
	return out
}

// noteOf returns the cleaned routing note of a decision, and false when the
// event has none (or only free text, which Clean drops).
func noteOf(ev telemetry.Event) (telemetry.RoutingNote, bool) {
	if ev.RoutingNote == nil {
		return telemetry.RoutingNote{}, false
	}
	n := ev.RoutingNote.Clean()
	return n, n.Verdict != ""
}

// reasonKey names what decided: the declared rule or profile, else the
// refinement reason, else the verdict alone. "rule:fast-lane", "guard_floor:risk_floor", "kept".
func reasonKey(n telemetry.RoutingNote) string {
	switch {
	case n.RuleID != "":
		return n.Verdict + ":" + n.RuleID
	case n.Profile != "":
		return n.Verdict + ":" + n.Profile
	case n.Reason != "":
		return n.Verdict + ":" + n.Reason
	}
	return n.Verdict
}

// sortedCounts lists counts by count descending, then key; limit <= 0 keeps all.
func sortedCounts(m map[string]int, limit int) []WhyCount {
	out := make([]WhyCount, 0, len(m))
	for k, c := range m {
		out = append(out, WhyCount{Key: k, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
