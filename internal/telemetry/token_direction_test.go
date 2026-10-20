// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
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
	u := TokenUsage{InputTokens: 1000, OutputTokens: 500}
	base, actual := CostUSD(req, u), CostUSD(got, u)
	in, out := int64(1000), int64(500)
	decision = Event{
		CorrelationID: "d", Harness: rankHarness, SessionID: "s", Outcome: outcome,
		FromModel: requested, ToModel: ran,
	}
	usage = Event{
		CorrelationID: "u", Harness: rankHarness, SessionID: "s", Outcome: OutcomeUsage,
		LinkedDecision: "d", FromModel: requested, ToModel: ran,
		ActualCostUSD: &actual, BaselineCostUSD: &base, BaselineSource: BaselineRequested,
		InputTokens: &in, OutputTokens: &out,
	}
	return usage, decision
}

// TestTokenDirectionByRank_Gates: the gates that do not depend on a model pair
// place a record exactly as the dollar method does.
func TestTokenDirectionByRank_Gates(t *testing.T) {
	cat := catalog.Load()
	usage, decision := rankEvents(t, cat, "claude-opus-4-8", "claude-haiku-4-5", OutcomeRewriteEmitted)
	cases := []struct {
		name                     string
		decision                 Event
		found, unrouted, refuted bool
		want                     string
	}{
		{"control group", decision, true, true, false, TokenUnchanged},
		{"no linked decision", decision, false, false, false, TokenUnattributed},
		{"held decision", withCorrections(decision), true, false, false, TokenUnchanged},
		{"allow, no rewrite", withOutcome(decision, OutcomeAllow), true, false, false, TokenUnchanged},
		{"rewrite not honored", decision, true, false, true, TokenUnchanged},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tokenDirectionByRank(usage, tc.decision, tc.found, tc.unrouted, tc.refuted, cat)
			want := tokenDirection(usage, tc.decision, tc.found, tc.unrouted, tc.refuted)
			if got != tc.want || want != tc.want {
				t.Fatalf("rank %q, dollars %q, want %q", got, want, tc.want)
			}
		})
	}
}

// TestTokenDirectionByRank_NoRealCost: a record without priced costs is not
// placed, as in the dollar method, so old lines keep their bucket.
func TestTokenDirectionByRank_NoRealCost(t *testing.T) {
	cat := catalog.Load()
	usage, decision := rankEvents(t, cat, "claude-opus-4-8", "claude-haiku-4-5", OutcomeRewriteEmitted)
	usage.ActualCostUSD = nil
	if got := tokenDirectionByRank(usage, decision, true, false, false, cat); got != TokenUnattributed {
		t.Fatalf("got %q, want %q", got, TokenUnattributed)
	}
}

func withCorrections(e Event) Event {
	e.Corrections = []string{"held"}
	return e
}

func withOutcome(e Event, outcome string) Event {
	e.Outcome = outcome
	return e
}

// TestTokenDirectionByRank_Placement: an applied rewrite moves tokens to the
// cheaper or pricier tier, and a same-tier rewrite stays unchanged.
func TestTokenDirectionByRank_Placement(t *testing.T) {
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
			got := tokenDirectionByRank(usage, decision, true, false, false, cat)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTokenDirectionByRank_DivergenceFromDollars lists every claude-code pair
// where the rank direction and the dollar direction differ. Those are the
// cases part 4a has to show: same-tier price gaps and price inversions across
// tiers. The list is logged, not asserted, so the catalog can move.
func TestTokenDirectionByRank_DivergenceFromDollars(t *testing.T) {
	cat := catalog.Load()
	var ids []string
	for _, e := range cat.Entries() {
		if e.Harness == rankHarness {
			ids = append(ids, e.ID)
		}
	}
	var diverged []string
	for _, requested := range ids {
		for _, ran := range ids {
			if requested == ran {
				continue
			}
			usage, decision := rankEvents(t, cat, requested, ran, OutcomeRewriteEmitted)
			rank := tokenDirectionByRank(usage, decision, true, false, false, cat)
			dollars := tokenDirection(usage, decision, true, false, false)
			if rank == dollars {
				continue
			}
			diverged = append(diverged, requested+" -> "+ran+": rank "+rank+", dollars "+dollars)
		}
	}
	t.Logf("%d same-tier or price-inverted pairs differ from the dollar method:\n%v", len(diverged), diverged)
}
