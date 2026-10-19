// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"math"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func decisionRow(id, outcome, verdict, from, to string) telemetry.Event {
	return telemetry.Event{CorrelationID: id, Harness: "claude-code", SessionID: "s1", Outcome: outcome, Verdict: verdict, FromModel: from, ToModel: to}
}

func usageRow(link, ran string, actual, baseline float64) telemetry.Event {
	return telemetry.Event{
		CorrelationID: "u-" + link, Harness: "claude-code", SessionID: "s1", Outcome: telemetry.OutcomeUsage,
		LinkedDecision: link, FromModel: "x", ToModel: ran, ActualCostUSD: &actual, BaselineCostUSD: &baseline,
		BaselineSource: telemetry.BaselineRequested,
	}
}

// A usage record is priced against the model its decision asked for. Only an
// applied rewrite makes that a counterfactual: when the hook left the spawn
// alone, a child that ran on another model than the one the transcript showed
// (a /model switch, opusplan) is not the router's saving or its overspend.
func TestRealCostIsCreditedOnlyToAppliedRewrites(t *testing.T) {
	cases := []struct {
		name      string
		decision  telemetry.Event
		usage     telemetry.Event
		wantDelta float64
	}{
		{"applied downshift keeps its saving", decisionRow("d", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", "opus", "sonnet"), usageRow("d", "sonnet", 0.5, 1.0), 0.5},
		{"applied upshift keeps its overspend", decisionRow("d", telemetry.OutcomeRewriteEmitted, "UPSHIFT", "sonnet", "opus"), usageRow("d", "opus", 2.0, 1.0), -1.0},
		{"unchanged spawn that ran dearer is not an overspend", decisionRow("d", telemetry.OutcomeAllow, "OK", "sonnet", "sonnet"), usageRow("d", "opus", 4.76, 2.38), 0},
		{"unchanged spawn that ran cheaper is not a saving", decisionRow("d", telemetry.OutcomeAllow, "DOWNSHIFT", "opus", "opus"), usageRow("d", "sonnet", 0.41, 0.82), 0},
		{"held downshift is not a saving", decisionRow("d", telemetry.OutcomeAllow, "DOWNSHIFT", "opus", "opus"), usageRow("d", "sonnet", 1, 2), 0},
		{"usage with no known decision is left as recorded", decisionRow("other", telemetry.OutcomeAllow, "OK", "sonnet", "sonnet"), usageRow("d", "opus", 2, 1), -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events := []telemetry.Event{tc.decision, tc.usage}
			stats := telemetry.Aggregate(events)
			if math.Abs(stats.RealCostDeltaUSD-tc.wantDelta) > 1e-9 {
				t.Errorf("Aggregate delta = %v, want %v", stats.RealCostDeltaUSD, tc.wantDelta)
			}
			if tc.wantDelta >= 0 && stats.RealOverspendUSD != 0 {
				t.Errorf("overspend = %v, want none", stats.RealOverspendUSD)
			}
			if stats.RealCostEvents != 1 {
				t.Errorf("the child still counts as a priced event: got %d", stats.RealCostEvents)
			}
			sum := telemetry.SessionSummary(events, "s1")
			if math.Abs(sum.DeltaUSD-tc.wantDelta) > 1e-9 {
				t.Errorf("SessionSummary delta = %v, want %v", sum.DeltaUSD, tc.wantDelta)
			}
		})
	}
}

// Honor is asked of a rewrite only. Counting every unchanged spawn as
// "unobserved" made the report say 58 of 61 were unobserved when the three
// rewrites that existed were all confirmed.
func TestHonorCountsAppliedRewritesOnly(t *testing.T) {
	events := []telemetry.Event{
		decisionRow("keep-1", telemetry.OutcomeAllow, "OK", "sonnet", "sonnet"),
		decisionRow("keep-2", telemetry.OutcomeAllow, "DOWNSHIFT", "opus", "opus"),
		decisionRow("seen", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", "opus", "sonnet"),
		decisionRow("blind", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", "opus", "sonnet"),
		{CorrelationID: "r", Harness: "claude-code", SessionID: "s1", Outcome: telemetry.OutcomeResolved, LinkedDecision: "seen", RewriteHonored: boolPtr(true)},
	}
	stats := telemetry.Aggregate(events)
	if stats.HonorObserved != 1 || stats.HonorUnobserved != 1 {
		t.Fatalf("honor observed/unobserved = %d/%d, want 1/1 (the two unchanged spawns are neither)", stats.HonorObserved, stats.HonorUnobserved)
	}
}
