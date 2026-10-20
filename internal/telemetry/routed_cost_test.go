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
	in := int64(100)
	return telemetry.Event{InputTokens: &in,
		CorrelationID: "u-" + link, Harness: "claude-code", SessionID: "s1", Outcome: telemetry.OutcomeUsage,
		LinkedDecision: link, ToModel: ran, ActualCostUSD: &actual, BaselineCostUSD: &baseline,
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
		{"applied downshift keeps its saving", decisionRow("d", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", "claude-opus-4-7", "claude-sonnet-4-6"), usageRow("d", "claude-sonnet-4-6", 0.5, 1.0), 0.5},
		{"applied upshift keeps its overspend", decisionRow("d", telemetry.OutcomeRewriteEmitted, "UPSHIFT", "claude-sonnet-4-6", "claude-opus-4-7"), usageRow("d", "claude-opus-4-7", 2.0, 1.0), -1.0},
		{"unchanged spawn that ran dearer is not an overspend", decisionRow("d", telemetry.OutcomeAllow, "OK", "claude-sonnet-4-6", "claude-sonnet-4-6"), usageRow("d", "claude-opus-4-7", 4.76, 2.38), 0},
		{"unchanged spawn that ran cheaper is not a saving", decisionRow("d", telemetry.OutcomeAllow, "DOWNSHIFT", "claude-opus-4-7", "claude-opus-4-7"), usageRow("d", "claude-sonnet-4-6", 0.41, 0.82), 0},
		{"held downshift is not a saving", decisionRow("d", telemetry.OutcomeAllow, "DOWNSHIFT", "claude-opus-4-7", "claude-opus-4-7"), usageRow("d", "claude-sonnet-4-6", 1, 2), 0},
		{"usage with no known decision is left as recorded", decisionRow("other", telemetry.OutcomeAllow, "OK", "claude-sonnet-4-6", "claude-sonnet-4-6"), usageRow("d", "claude-opus-4-7", 2, 1), -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.usage.FromModel = tc.decision.FromModel // the child was requested as its decision says
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
			// The live session view says the same in tokens: saved only
			// for an applied cheaper rewrite, pricier only for an applied
			// dearer one.
			sum := telemetry.SessionSummary(events, "s1").Tokens
			// A record with no known decision is unattributed: neither.
			wantSaved, wantPricier := tc.wantDelta > 0, tc.wantDelta < 0
			if tc.decision.CorrelationID != tc.usage.LinkedDecision {
				wantSaved, wantPricier = false, false
			}
			if saved, pricier := sum.WorkSaved > 0, sum.WorkPricier > 0; saved != wantSaved || pricier != wantPricier {
				t.Errorf("SessionSummary tokens %+v, want saved %v pricier %v", sum, wantSaved, wantPricier)
			}
		})
	}
}

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
