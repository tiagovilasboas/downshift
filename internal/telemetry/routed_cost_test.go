// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func decisionRow(id, outcome, verdict, from, to string) telemetry.Event {
	return telemetry.Event{CorrelationID: id, Harness: "claude-code", SessionID: "s1", Outcome: outcome, Verdict: verdict, FromModel: from, ToModel: to}
}

// A usage record is priced against the model its decision asked for. Only an
// applied rewrite makes that a counterfactual: when the hook left the spawn
// alone, a child that ran on another model than the one the transcript showed
// (a /model switch, opusplan) is not the router's saving or its overspend.

// Honor is asked of a rewrite only. Counting every unchanged spawn as
// "unobserved" made the report say 58 of 61 were unobserved when the three
// rewrites that existed were all confirmed.
func TestHonorCountsAppliedRewritesOnly(t *testing.T) {
	events := []telemetry.Event{
		decisionRow("keep-1", telemetry.OutcomeAllow, "OK", "claude-sonnet-4-6", "claude-sonnet-4-6"),
		decisionRow("keep-2", telemetry.OutcomeAllow, "DOWNSHIFT", "claude-opus-4-7", "claude-opus-4-7"),
		decisionRow("seen", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", "claude-opus-4-7", "claude-sonnet-4-6"),
		decisionRow("blind", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", "claude-opus-4-7", "claude-sonnet-4-6"),
		{CorrelationID: "r", Harness: "claude-code", SessionID: "s1", Outcome: telemetry.OutcomeResolved, LinkedDecision: "seen", RewriteHonored: boolPtr(true)},
	}
	stats := telemetry.Aggregate(events)
	if stats.HonorObserved != 1 || stats.HonorUnobserved != 1 {
		t.Fatalf("honor observed/unobserved = %d/%d, want 1/1 (the two unchanged spawns are neither)", stats.HonorObserved, stats.HonorUnobserved)
	}
}
