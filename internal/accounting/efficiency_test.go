// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// effSpawn is one completed spawn with usage: a decision that asked for
// requested and became ran (outcome rewrite or allow), plus its usage and end.
func effSpawn(harness, session, id, requested, ran, outcome string, completed bool) []telemetry.Event {
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	in, out := int64(100), int64(50)
	evs := []telemetry.Event{{CorrelationID: id, Timestamp: ts, Harness: harness, SessionID: session,
		Outcome: outcome, FromModel: requested, ToModel: ran}}
	if completed {
		evs = append(evs,
			telemetry.Event{CorrelationID: id + "u", Timestamp: ts, Harness: harness, SessionID: session,
				Outcome: telemetry.OutcomeUsage, LinkedDecision: id, FromModel: requested, ToModel: ran,
				InputTokens: &in, OutputTokens: &out},
			telemetry.Event{CorrelationID: id + "e", Timestamp: ts, Harness: harness, SessionID: session,
				Outcome: telemetry.OutcomeChildEnded, LinkedDecision: id, ChildStatus: telemetry.ChildCompleted})
	}
	return evs
}

func effRow(t *testing.T, e Efficiency, harness string) HarnessEfficiency {
	t.Helper()
	for _, h := range e.Harnesses {
		if h.Harness == harness {
			return h
		}
	}
	t.Fatalf("no row for %s in %+v", harness, e)
	return HarnessEfficiency{}
}

func TestBuildEfficiency_Tiers(t *testing.T) {
	rw := telemetry.OutcomeRewriteEmitted
	al := telemetry.OutcomeAllow
	cases := []struct {
		name                                string
		spawns                              [][]telemetry.Event
		known, nonFrontier, unknown         int
		observed                            bool
		pct                                 float64
		small, mid, frontier, unknownSpawns int
		completed                           int
	}{
		{"downshifted tasks preserve premium", [][]telemetry.Event{
			effSpawn("claude-code", "", "a", "claude-opus-5-5", "claude-haiku-4-5", rw, true),
			effSpawn("claude-code", "", "b", "claude-opus-5-5", "claude-sonnet-5-5", rw, true),
			effSpawn("claude-code", "", "c", "claude-opus-5-5", "claude-opus-5-5", al, true),
			effSpawn("claude-code", "", "d", "claude-opus-5-5", "claude-opus-5-5", al, true),
		}, 4, 2, 0, true, 50, 1, 1, 2, 0, 4},
		{"applied model, not the request, sets the tier", [][]telemetry.Event{
			effSpawn("claude-code", "", "a", "claude-haiku-4-5", "claude-opus-5-5", rw, true),
		}, 1, 0, 0, true, 0, 0, 0, 1, 0, 1},
		{"unknown tier is excluded from the ratio", [][]telemetry.Event{
			effSpawn("claude-code", "", "a", "mystery", "mystery", al, true),
			effSpawn("claude-code", "", "b", "claude-opus-5-5", "claude-haiku-4-5", rw, true),
		}, 1, 1, 1, true, 100, 1, 0, 0, 1, 2},
		{"no known tier reads unobserved, never 0", [][]telemetry.Event{
			effSpawn("claude-code", "", "a", "mystery", "mystery", al, true),
		}, 0, 0, 1, false, 0, 0, 0, 0, 1, 1},
		{"not completed is not in the ratio but is in the spawns", [][]telemetry.Event{
			effSpawn("claude-code", "", "a", "claude-opus-5-5", "claude-haiku-4-5", rw, false),
		}, 0, 0, 0, false, 0, 1, 0, 0, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var events []telemetry.Event
			for _, s := range tc.spawns {
				events = append(events, s...)
			}
			got := effRow(t, BuildEfficiency(events, Scope{Kind: ScopeAll}), "claude-code")
			if got.TierKnown != tc.known || got.NonFrontier != tc.nonFrontier || got.TierUnknown != tc.unknown ||
				got.PremiumObserved != tc.observed || got.PremiumPreservedPct != tc.pct || got.Completed != tc.completed {
				t.Errorf("premium = %+v", got)
			}
			s := got.SpawnsByTier
			if s.Small.Count != tc.small || s.Mid.Count != tc.mid || s.Frontier.Count != tc.frontier || s.Unknown.Count != tc.unknownSpawns {
				t.Errorf("spawns by tier = %+v", s)
			}
			if got.Spawns != len(tc.spawns) {
				t.Errorf("spawns = %d", got.Spawns)
			}
		})
	}
}

func TestBuildEfficiency_ReusesRoutedTableFigures(t *testing.T) {
	var events []telemetry.Event
	events = append(events, effSpawn("codex", "", "a", "claude-opus-5-5", "claude-haiku-4-5", telemetry.OutcomeRewriteEmitted, true)...)
	events = append(events, effSpawn("codex", "", "b", "claude-opus-5-5", "claude-opus-5-5", telemetry.OutcomeAllow, false)...)
	want := telemetry.BuildReport(events, 0).Routed
	got := effRow(t, BuildEfficiency(events, Scope{Kind: ScopeAll}), "codex")
	if got.Ended != want.Ended || got.Completed != want.Completed || got.Reworked != want.Reworked || got.ReworkRatio != want.ReworkRate {
		t.Errorf("got %+v, routed %+v", got, want)
	}
}

func TestBuildEfficiency_HarnessesInNameOrderNeverSummed(t *testing.T) {
	var events []telemetry.Event
	events = append(events, effSpawn("kiro", "", "k", "claude-opus-5-5", "claude-haiku-4-5", telemetry.OutcomeRewriteEmitted, true)...)
	events = append(events, effSpawn("codex", "", "c", "claude-opus-5-5", "claude-opus-5-5", telemetry.OutcomeAllow, true)...)
	events = append(events, effSpawn("claude-code", "", "a", "claude-opus-5-5", "claude-sonnet-5-5", telemetry.OutcomeRewriteEmitted, true)...)
	events = append(events, telemetry.Event{Outcome: telemetry.OutcomeAllow, Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}) // no harness
	got := BuildEfficiency(events, Scope{Kind: ScopeAll})
	if len(got.Harnesses) != 3 || got.Harnesses[0].Harness != "claude-code" || got.Harnesses[1].Harness != "codex" || got.Harnesses[2].Harness != "kiro" {
		t.Fatalf("harnesses = %+v", got.Harnesses)
	}
	if p := got.Harnesses[1].PremiumPreservedPct; p != 0 {
		t.Errorf("codex pct = %v, want 0 (own row only)", p)
	}
}

func TestBuildEfficiency_SessionScope(t *testing.T) {
	var events []telemetry.Event
	events = append(events, effSpawn("claude-code", "s1", "a", "claude-opus-5-5", "claude-haiku-4-5", telemetry.OutcomeRewriteEmitted, true)...)
	events = append(events, effSpawn("claude-code", "s2", "b", "claude-opus-5-5", "claude-opus-5-5", telemetry.OutcomeAllow, true)...)
	got := effRow(t, BuildEfficiency(events, Scope{Kind: ScopeSession, SessionHash: "s1"}), "claude-code")
	if got.Spawns != 1 || got.SpawnsByTier.Small.Count != 1 || got.PremiumPreservedPct != 100 {
		t.Errorf("session s1 = %+v", got)
	}
	if n := len(BuildEfficiency(events, Scope{Kind: ScopeSession, SessionHash: "none"}).Harnesses); n != 0 {
		t.Errorf("unknown session has %d rows", n)
	}
}

func TestBuildEfficiency_DaysWindow(t *testing.T) {
	old := effSpawn("claude-code", "", "old", "claude-opus-5-5", "claude-haiku-4-5", telemetry.OutcomeRewriteEmitted, true)
	stamp := time.Now().Add(-90 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
	for i := range old {
		old[i].Timestamp = stamp
	}
	if n := len(BuildEfficiency(old, Scope{Kind: ScopeDays, Days: 30}).Harnesses); n != 0 {
		t.Errorf("rows outside the window = %d", n)
	}
	if n := len(BuildEfficiency(old, Scope{Kind: ScopeAll}).Harnesses); n != 1 {
		t.Errorf("all-time rows = %d", n)
	}
}

func TestBuildEfficiency_EmptyIsNotNil(t *testing.T) {
	if got := BuildEfficiency(nil, Scope{Kind: ScopeAll}); got.Harnesses == nil || len(got.Harnesses) != 0 {
		t.Errorf("got %+v", got)
	}
}
