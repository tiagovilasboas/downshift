// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/catalog"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

func TestRanTiers(t *testing.T) {
	withAllowlist(t)
	cat := catalog.Load()
	small := cat.ModelFor(rankHarness, core.TierSmall).ID
	mid := cat.ModelFor(rankHarness, core.TierMid).ID
	front := cat.ModelFor(rankHarness, core.TierFrontier).ID

	decision := func(outcome, from, to string) Event {
		return Event{CorrelationID: "d", Harness: rankHarness, SessionID: "s", Outcome: outcome, FromModel: from, ToModel: to}
	}
	usage := func(id, model string) Event {
		return Event{CorrelationID: id, Harness: rankHarness, SessionID: "s", Outcome: OutcomeUsage, LinkedDecision: "d", ToModel: model}
	}
	refuted := false
	resolved := Event{CorrelationID: "r", Harness: rankHarness, SessionID: "s", Outcome: OutcomeResolved, LinkedDecision: "d", RewriteHonored: &refuted}

	cases := []struct {
		name   string
		events []Event
		noCat  bool
		want   RanTier
	}{
		{"gateway id resolves through GatewayTier", []Event{decision(OutcomeRewriteEmitted, mid, allowlistSmallID), usage("u", allowlistSmallID)}, false, RanSmall},
		{"two usage records, highest wins", []Event{decision(OutcomeAllow, front, front), usage("u1", small), usage("u2", front)}, false, RanFrontier},
		{"applied rewrite without usage takes final model", []Event{decision(OutcomeRewriteEmitted, front, small)}, false, RanSmall},
		{"refuted rewrite takes requested model", []Event{decision(OutcomeRewriteEmitted, front, small), resolved}, false, RanFrontier},
		{"allow without usage takes requested model", []Event{decision(OutcomeAllow, mid, mid)}, false, RanMid},
		{"hold without usage takes requested model", []Event{decision("hold", front, front)}, false, RanFrontier},
		{"model outside the catalog is unknown", []Event{decision(OutcomeAllow, "mystery", "mystery")}, false, RanUnknown},
		{"nil catalog is unknown", []Event{decision(OutcomeAllow, mid, mid)}, true, RanUnknown},
		{"blocked decision is not a task", []Event{decision(OutcomeBlocked, mid, small)}, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.noCat {
				DirectionCatalog = nil
			}
			got, ok := RanTiers(tc.events)[SpawnRef{rankHarness, "d"}]
			if tc.want == "" {
				if ok {
					t.Fatalf("blocked decision must be left out, got %q", got)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHigherRan(t *testing.T) {
	cases := []struct{ a, b, want RanTier }{
		{RanSmall, RanFrontier, RanFrontier},
		{RanMid, RanSmall, RanMid},
		{RanUnknown, RanSmall, RanSmall},
		{RanFrontier, RanUnknown, RanFrontier},
		{RanUnknown, RanUnknown, RanUnknown},
	}
	for _, tc := range cases {
		if got := higherRan(tc.a, tc.b); got != tc.want {
			t.Errorf("higherRan(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
		}
	}
}
