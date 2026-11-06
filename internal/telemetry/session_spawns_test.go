// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "testing"

func TestSummarizeSpawns(t *testing.T) {
	events := []Event{
		{Harness: "h", SessionID: "a", Outcome: "allow", Tier: "MID", AgentType: "custom", AgentName: "coder", ReasonCodes: []string{ReasonAboveAgentDefault}},
		{Harness: "h", SessionID: "a", Outcome: "rewrite_emitted", Tier: "SMALL", AgentType: "Explore"},
		{Harness: "h", SessionID: "a", Outcome: "baseline", Tier: "SMALL"},
		{Harness: "h", SessionID: "a", Outcome: OutcomeUsage, Tier: "SMALL"},
		{Harness: "h", SessionID: "b", Outcome: "allow", Tier: "FRONTIER"},
	}
	got := SummarizeSpawns(events, "a")
	if got.Spawns != 3 || got.ByTier["mid"] != 1 || got.ByTier["small"] != 2 || got.ByAgentType["unspecified"] != 1 || got.ByAgentType["coder"] != 1 ||
		got.ByAgentType["Explore"] != 1 || got.AboveAgentDefault != 1 {
		t.Fatalf("summary = %+v", got)
	}
	if empty := SummarizeSpawns(events, ""); empty.Spawns != 0 {
		t.Fatalf("empty session counted %d spawns", empty.Spawns)
	}
}
