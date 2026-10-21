// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
)

// TestPanelTotal_CountsEachChildOnce: the session total is the sum of the spawn
// rows. A resumed child (two usage records) counts once, and control, held,
// kept and unlinked children are counted but add nothing to the saved tokens.
func TestPanelTotal_CountsEachChildOnce(t *testing.T) {
	prev := DirectionCatalog
	DirectionCatalog = catalog.Load()
	defer func() { DirectionCatalog = prev }()

	session := strings.Repeat("c", 64)
	events := []Event{
		// saved: an applied downshift, resumed (two usage records, one child)
		decisionEv("saved", OutcomeRewriteEmitted, "general-purpose"),
		usageEv("saved", 100, 300, 9000, 600, 0.01, 0.04),
		usageEv("saved", 50, 50, 1000, 0, 0.002, 0.008),
		// kept: the model was allowed through, no rewrite
		decisionEv("kept", OutcomeAllow, "Explore"),
		usageEv("kept", 10, 90, 900, 0, 0.02, 0.02),
		// held: a guardrail corrected the decision
		decisionEv("held", OutcomeAllow, "custom", "R1"),
		usageEv("held", 10, 90, 900, 0, 0.02, 0.02),
		// control: routing disabled, decided but never applied
		{CorrelationID: "control", Harness: "claude-code", Outcome: OutcomeBaseline, Verdict: "DOWNSHIFT",
			FromModel: "claude-opus-5-5", ToModel: "claude-opus-5-5", ReasonCodes: []string{ReasonControl}},
		usageEv("control", 200, 200, 4000, 0, 0.05, 0.05),
		// pricier: an applied upshift
		{CorrelationID: "upshift", Harness: "claude-code", Outcome: OutcomeRewriteEmitted, Verdict: "UPSHIFT",
			FromModel: "claude-haiku-4-5", ToModel: "claude-opus-5-5"},
		usageEv("upshift", 40, 60, 0, 0, 0.03, 0.01),
		// unlinked usage: no decision in the session
		usageEv("", 1, 1, 0, 0, 0.001, 0.001),
	}
	for i := range events {
		events[i].SessionID = session
	}

	v := BuildPanel(events, session)
	if len(v.Spawns) != 5 {
		t.Fatalf("spawns = %d, want 5 (one row per decision)", len(v.Spawns))
	}
	if got := v.Tokens; got.Children != 6 || got.SavedChildren != 1 || got.WorkSaved != 500 || got.CacheSaved != 10600 || got.WorkPricier != 100 {
		t.Fatalf("total = %+v, want 6 children (5 decisions and the unlinked usage), 1 saved (500 work, 10600 cache), 100 pricier work", got)
	}

	var sum TokenSaving
	for _, sp := range v.Spawns {
		sum.add(sp.Tokens)
	}
	if sum.WorkSaved != v.Tokens.WorkSaved || sum.CacheSaved != v.Tokens.CacheSaved ||
		sum.WorkPricier != v.Tokens.WorkPricier || sum.SavedChildren != v.Tokens.SavedChildren {
		t.Fatalf("spawn rows sum to %+v, panel total is %+v", sum, v.Tokens)
	}
}
