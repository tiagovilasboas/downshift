// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "testing"

// routingSpawn is one decision plus one usage record: 1000 in, 500 out, 200
// cache write and cacheRead cache read tokens (billable 1700).
func routingSpawn(harness, id, parent, requested, ran string, cacheRead int64) []Event {
	in, out, cached, write := int64(1000), int64(500), cacheRead+200, int64(200)
	return []Event{
		{CorrelationID: id, Harness: harness, Outcome: OutcomeAllow, FromModel: requested, ToModel: ran, ParentModel: parent},
		{CorrelationID: id + "u", Harness: harness, Outcome: OutcomeUsage, LinkedDecision: id, FromModel: requested, ToModel: ran,
			InputTokens: &in, OutputTokens: &out, CachedTokens: &cached, CacheWriteTokens: &write},
	}
}

func TestRoutingTokens(t *testing.T) {
	cases := []struct {
		name             string
		parent, req, ran string
		cacheRead        int64
		want             RoutingRow
	}{
		{"session baseline, below", "claude-opus-4-8", "claude-opus-4-8", "claude-sonnet-4-6", 0,
			RoutingRow{Spawns: 1, BelowSpawns: 1, Billable: 1700, BelowBillable: 1700}},
		{"same tier is not below", "claude-opus-4-8", "claude-opus-4-8", "claude-opus-4-8", 0,
			RoutingRow{Spawns: 1, Billable: 1700}},
		{"session wins over requested", "claude-sonnet-4-6", "claude-opus-4-8", "claude-sonnet-4-6", 0,
			RoutingRow{Spawns: 1, Billable: 1700}},
		{"no session model falls back to requested, kept apart", "", "claude-opus-4-8", "claude-sonnet-4-6", 0,
			RoutingRow{FallbackSpawns: 1, FallbackBillable: 1700, FallbackBelowBillable: 1700}},
		{"unranked session model falls back", "mystery", "claude-opus-4-8", "claude-opus-4-8", 0,
			RoutingRow{FallbackSpawns: 1, FallbackBillable: 1700}},
		{"neither baseline ranks", "", "mystery", "claude-sonnet-4-6", 0,
			RoutingRow{UnobservedSpawns: 1}},
		{"cache read is its own field", "claude-opus-4-8", "claude-opus-4-8", "claude-haiku-4-5", 50000,
			RoutingRow{Spawns: 1, BelowSpawns: 1, Billable: 1700, BelowBillable: 1700, CacheRead: 50000}},
		{"above the session model is not below", "claude-haiku-4-5", "claude-haiku-4-5", "claude-opus-4-8", 0,
			RoutingRow{Spawns: 1, Billable: 1700}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RoutingTokens(routingSpawn("claude-code", "d", tc.parent, tc.req, tc.ran, tc.cacheRead)).ByHarness["claude-code"]
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestRoutingTokens_ResumedChildIsOneSpawn(t *testing.T) {
	ev := routingSpawn("claude-code", "d", "claude-opus-4-8", "claude-opus-4-8", "claude-sonnet-4-6", 0)
	second := ev[1]
	second.CorrelationID = "u2"
	got := RoutingTokens(append(ev, second)).ByHarness["claude-code"]
	want := RoutingRow{Spawns: 1, BelowSpawns: 1, Billable: 3400, BelowBillable: 3400}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRoutingTokens_HarnessesAreNeverSummed(t *testing.T) {
	ev := routingSpawn("claude-code", "a", "claude-opus-4-8", "claude-opus-4-8", "claude-sonnet-4-6", 0)
	ev = append(ev, routingSpawn("codex", "b", "", "mystery", "mystery", 0)...)
	ev = append(ev, routingSpawn("unknown", "c", "claude-opus-4-8", "claude-opus-4-8", "claude-opus-4-8", 0)...)
	v := RoutingTokens(ev)
	if len(v.ByHarness) != 3 {
		t.Fatalf("rows = %+v", v.ByHarness)
	}
	if !v.ByHarness["claude-code"].Observed() || v.ByHarness["codex"].Observed() || v.ByHarness["codex"].UnobservedSpawns != 1 {
		t.Fatalf("got %+v", v.ByHarness)
	}
	if u, ok := v.ByHarness["unknown"]; !ok || u.UnobservedSpawns != 1 {
		t.Fatalf("unknown stays its own row, got %+v", u)
	}
	if got := v.Only(); got != (RoutingRow{}) {
		t.Fatalf("Only() over three harnesses must be zero, got %+v", got)
	}
}

func TestRoutingTokens_ByAgentType(t *testing.T) {
	a := routingSpawn("claude-code", "a", "claude-opus-4-8", "claude-opus-4-8", "claude-sonnet-4-6", 0)
	a[0].AgentType = "coder"
	b := routingSpawn("claude-code", "b", "claude-opus-4-8", "claude-opus-4-8", "claude-opus-4-8", 0)
	v := RoutingTokens(append(append([]Event{}, a...), b...))
	if got := v.ByAgentType["coder"]; got.BelowSpawns != 1 || got.Spawns != 1 {
		t.Fatalf("coder = %+v", got)
	}
	if got := v.ByAgentType[UntypedAgent]; got.BelowSpawns != 0 || got.Spawns != 1 {
		t.Fatalf("untyped = %+v", got)
	}
	if v.Spawn(a[0]).BelowBillable != 1700 {
		t.Fatalf("Spawn() = %+v", v.Spawn(a[0]))
	}
}

func TestRoutingRow_Line(t *testing.T) {
	if (RoutingRow{}).Line() != "" {
		t.Fatal("empty must read empty")
	}
	got := RoutingRow{Spawns: 5, BelowSpawns: 3, Billable: 40000, BelowBillable: 12000, UnobservedSpawns: 1}.Line()
	want := "12k of 40k billable below the session model (3 of 5 spawns) · 1 unobserved"
	if got != want {
		t.Fatalf("got %q", got)
	}
}
