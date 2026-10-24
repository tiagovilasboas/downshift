// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decisionintelligence

import (
	"reflect"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func TestSignalsFromDecision(t *testing.T) {
	cases := []struct {
		name string
		d    core.Decision
		want Signals
	}{
		{
			name: "no risk and available quota",
			d:    core.Decision{Tier: core.TierSmall, Confident: true, Session: core.Session{QuotaStatus: "available"}},
			want: Signals{BaseTier: core.TierSmall, ClassifierConfidence: 1, Risk: RiskLow, Budget: BudgetAvailable},
		},
		{
			name: "high risk category",
			d:    core.Decision{Tier: core.TierMid, Risk: []string{"concurrency", "secret"}},
			want: Signals{BaseTier: core.TierMid, Risk: RiskHigh},
		},
		{
			name: "concurrency alone is moderate",
			d:    core.Decision{Tier: core.TierMid, Risk: []string{"concurrency"}},
			want: Signals{BaseTier: core.TierMid, Risk: RiskModerate},
		},
		{
			name: "risk floor without a category is moderate",
			d:    core.Decision{Tier: core.TierMid, Guards: core.Guards{RiskFloor: true}},
			want: Signals{BaseTier: core.TierMid, Risk: RiskModerate},
		},
		{
			name: "opaque task has unknown risk",
			d:    core.Decision{Tier: core.TierMid, TaskNotVisible: true},
			want: Signals{BaseTier: core.TierMid, Risk: RiskUnknown},
		},
		{
			name: "exhausted quota",
			d:    core.Decision{Tier: core.TierFrontier, Session: core.Session{QuotaStatus: "exhausted"}},
			want: Signals{BaseTier: core.TierFrontier, Risk: RiskLow, Budget: BudgetExhausted},
		},
		{
			name: "stale quota is not a budget state",
			d:    core.Decision{Tier: core.TierMid, Session: core.Session{QuotaStatus: "stale", CreditHeld: true}},
			want: Signals{BaseTier: core.TierMid, Risk: RiskLow},
		},
		{
			name: "reviewed miss at this tier",
			d:    core.Decision{Tier: core.TierSmall, Shadow: core.Shadow{AdaptWould: core.TierMid, AdaptWouldSet: true}},
			want: Signals{BaseTier: core.TierSmall, Risk: RiskLow, PreviousReviewedOutcome: OutcomeFailed},
		},
		{
			name: "reviewed hit at a cheaper tier is not an outcome here",
			d:    core.Decision{Tier: core.TierMid, Shadow: core.Shadow{AdaptWould: core.TierSmall, AdaptWouldSet: true}},
			want: Signals{BaseTier: core.TierMid, Risk: RiskLow},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SignalsFromDecision(tc.d); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("SignalsFromDecision = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestSignalsFromDecision_FeedsEvaluateWithoutApplying(t *testing.T) {
	d := core.Decision{Tier: core.TierSmall, Risk: []string{"payment"}, Session: core.Session{QuotaStatus: "exhausted"}}
	got := Evaluate(SignalsFromDecision(d))
	want := []string{"high_risk", "budget_constrained"}
	if !reflect.DeepEqual(got.Reasons, want) || got.Apply || !got.RequiresReview {
		t.Fatalf("recommendation = %+v", got)
	}
}
