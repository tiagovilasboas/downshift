// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

package decisionintelligence

import (
	"reflect"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

func TestEvaluate_NoSignalPreservesTierAndIsAdvisory(t *testing.T) {
	got := Evaluate(Signals{BaseTier: core.TierSmall, ClassifierConfidence: 0.4})
	if got.Tier != core.TierSmall || got.Apply || got.RequiresReview || got.BudgetConstrained {
		t.Fatalf("unexpected recommendation: %+v", got)
	}
	if !reflect.DeepEqual(got.Reasons, []string{"no_advisory_signal"}) {
		t.Fatalf("reasons = %v", got.Reasons)
	}
}

func TestEvaluate_NeverDowngradesBaseTier(t *testing.T) {
	got := Evaluate(Signals{BaseTier: core.TierFrontier, Budget: BudgetExhausted})
	if got.Tier != core.TierFrontier || !got.BudgetConstrained {
		t.Fatalf("recommendation = %+v", got)
	}
}

func TestEvaluate_HighRiskRaisesTierAndRequiresReview(t *testing.T) {
	got := Evaluate(Signals{BaseTier: core.TierSmall, Risk: RiskHigh, ClassifierConfidence: 0.2})
	if got.Tier != core.TierFrontier || !got.RequiresReview || got.Confidence != 0.8 {
		t.Fatalf("recommendation = %+v", got)
	}
}

func TestEvaluate_ReviewedFailureEscalatesOneTier(t *testing.T) {
	got := Evaluate(Signals{BaseTier: core.TierSmall, PreviousReviewedOutcome: OutcomeFailed})
	if got.Tier != core.TierMid || got.RequiresReview {
		t.Fatalf("recommendation = %+v", got)
	}
}

func TestEvaluate_SensitivityDoesNotSelectAProvider(t *testing.T) {
	got := Evaluate(Signals{BaseTier: core.TierMid, Sensitivity: SensitivityRestricted})
	if got.Tier != core.TierMid || !got.RequiresReview || got.Apply {
		t.Fatalf("recommendation = %+v", got)
	}
}

func TestJevAdapterIsDisabledByDefault(t *testing.T) {
	if (JevAdapter{}).Available() {
		t.Fatal("zero-value JevAdapter must be disabled")
	}
}
