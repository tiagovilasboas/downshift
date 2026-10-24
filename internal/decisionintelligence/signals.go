// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decisionintelligence

import (
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/quotamodel"
)

// moderateRiskCategories are risk categories (classify/risk.go) that hold a
// downshift but do not, on their own, call for a frontier model and review.
// Every other category is high risk: secrets, auth, crypto, money, PII,
// vulnerabilities, destructive operations and prompt injection.
var moderateRiskCategories = map[string]bool{"concurrency": true}

// SignalsFromDecision maps the structured fields the deterministic policy
// already computed into advisory signals. It never reads task text, and it
// takes outcomes only from reviewed memory: unreviewed telemetry stays
// OutcomeUnknown.
func SignalsFromDecision(d core.Decision) Signals {
	confidence := 0.0
	if d.Confident {
		confidence = 1
	}
	return Signals{
		BaseTier:                d.Tier,
		ClassifierConfidence:    confidence,
		Risk:                    riskFromDecision(d),
		Budget:                  budgetFromQuota(d.Session.QuotaStatus),
		PreviousReviewedOutcome: outcomeFromAdaptMemory(d),
	}
}

func riskFromDecision(d core.Decision) Risk {
	for _, category := range d.Risk {
		if !moderateRiskCategories[category] {
			return RiskHigh
		}
	}
	if len(d.Risk) > 0 || d.Guards.RiskFloor {
		return RiskModerate
	}
	if d.TaskNotVisible {
		return RiskUnknown
	}
	return RiskLow
}

func budgetFromQuota(status string) Budget {
	switch quotamodel.Status(status) {
	case quotamodel.Available:
		return BudgetAvailable
	case quotamodel.Exhausted:
		return BudgetExhausted
	default:
		// Stale and unknown evidence is not a budget state.
		return BudgetUnknown
	}
}

// outcomeFromAdaptMemory reads adapt memory, which is built only from
// engineer-reviewed feedback. Memory selects a higher tier only after a
// reviewed miss at this tier for the same task shape. A cheaper pick is a
// success at another tier, not at this one, so it stays unknown.
func outcomeFromAdaptMemory(d core.Decision) Outcome {
	if d.Shadow.AdaptWouldSet && d.Shadow.AdaptWould > d.Tier {
		return OutcomeFailed
	}
	return OutcomeUnknown
}
