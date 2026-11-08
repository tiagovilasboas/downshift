// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/catalog"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

const rankHarness = "claude-code"

// rankEvents builds a decision and a priced usage record for one spawn. The
// costs come from the catalog prices, as the hook writes them: baseline on the
// requested model, actual on the model that ran.
func rankEvents(t *testing.T, cat core.Resolver, requested, ran, outcome string) (usage, decision Event) {
	t.Helper()
	req, ok := cat.LookupByID(rankHarness, requested)
	if !ok {
		t.Fatalf("requested %s not in catalog", requested)
	}
	got, ok := cat.LookupByID(rankHarness, ran)
	if !ok {
		t.Fatalf("ran %s not in catalog", ran)
	}
	_, _ = req, got // the tier comparison reads the catalog, not a price
	in, out := int64(1000), int64(500)
	decision = Event{
		CorrelationID: "d", Harness: rankHarness, SessionID: "s", Outcome: outcome,
		FromModel: requested, ToModel: ran,
	}
	usage = Event{
		CorrelationID: "u", Harness: rankHarness, SessionID: "s", Outcome: OutcomeUsage,
		LinkedDecision: "d", FromModel: requested, ToModel: ran,
		BaselineSource: BaselineRequested,
		InputTokens:    &in, OutputTokens: &out,
	}
	return usage, decision
}

// tokenDirection ranks a usage record's ran model against its requested model,
// as RoutingTokens does for the fallback baseline. A rewrite the harness did
// not honor ran on the requested model.
func tokenDirection(usage, decision Event, found, _ bool, refuted bool) string {
	requested, ran := tokenModels(usage, decision, found, refuted)
	return directionVs(usage.Harness, requested, ran)
}

// TestRoutingTokens_SameModelEffortChangeIsNotBelow: a spawn whose model did not
// change (the panel's "effort changed") is not below the baseline, even
// when the requested model is the native alias and the ran id is the dated one.
func TestRoutingTokens_SameModelEffortChangeIsNotBelow(t *testing.T) {
	cat := catalog.Load()
	prev := DirectionCatalog
	DirectionCatalog = catalog.TierViewOf(cat)
	defer func() { DirectionCatalog = prev }()
	for _, pair := range [][2]string{
		{"claude-haiku-4-5", "claude-haiku-4-5"},
		{"haiku", "claude-haiku-4-5-20251001"},
		{"claude-haiku-5-5", "claude-haiku-5-5"},
	} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			usage, decision := rankEvents(t, cat, pair[0], pair[1], OutcomeRewriteEmitted)
			if got := tokenDirection(usage, decision, true, false, false); got != TokenUnchanged {
				t.Fatalf("direction = %q, want %q", got, TokenUnchanged)
			}
			v := RoutingTokens([]Event{decision, usage})
			if got := v.ByHarness[rankHarness]; got.FallbackBelowBillable != 0 || got.BelowBillable != 0 {
				t.Fatalf("row = %+v, want nothing below", got)
			}
		})
	}
}

// TestTokenDirection_Placement: a run on another tier is cheaper or pricier, and a same-tier rewrite stays unchanged.
func TestTokenDirection_Placement(t *testing.T) {
	cat := catalog.Load()
	cases := []struct {
		name, requested, ran, want string
	}{
		{"downshift to a smaller tier", "claude-opus-4-8", "claude-haiku-4-5", TokenCheaper},
		{"upshift to a larger tier", "claude-haiku-4-5", "claude-sonnet-4-6", TokenPricier},
		{"same tier, different price", "claude-sonnet-4-6", "claude-sonnet-5-5", TokenUnchanged},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			usage, decision := rankEvents(t, cat, tc.requested, tc.ran, OutcomeRewriteEmitted)
			got := tokenDirection(usage, decision, true, false, false)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
