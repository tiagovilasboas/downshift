// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package decisionintelligence provides advisory, provider-agnostic routing
// signals. It never authorizes an action or replaces deterministic safety,
// permission, or budget gates owned by the routing policy.
package decisionintelligence

import "github.com/tiagovilasboas/downshift/internal/core"

// Risk is a structured assessment supplied by a trusted caller. It is not
// inferred from, or a place to retain, task text.
type Risk int

const (
	RiskUnknown Risk = iota
	RiskLow
	RiskModerate
	RiskHigh
	RiskCritical
)

// Sensitivity is the handling classification of task data. It is advisory:
// provider allowlists and permission checks remain hard gates elsewhere.
type Sensitivity int

const (
	SensitivityUnknown Sensitivity = iota
	SensitivityPublic
	SensitivityInternal
	SensitivityConfidential
	SensitivityRestricted
)

// Budget describes the trusted budget state. This package never approves an
// over-budget route; the authoritative budget gate must evaluate it separately.
type Budget int

const (
	BudgetUnknown Budget = iota
	BudgetAvailable
	BudgetTight
	BudgetExhausted
)

// Outcome is an explicit, reviewed result from a prior comparable task.
// Unreviewed execution telemetry must be represented as OutcomeUnknown.
type Outcome int

const (
	OutcomeUnknown Outcome = iota
	OutcomeSucceeded
	OutcomeRetry
	OutcomeFailed
)

// Signals contains only structured metadata for one advisory evaluation.
// BaseTier must be the output of the deterministic router after its hard gates.
type Signals struct {
	BaseTier                core.Tier
	ClassifierConfidence    float64
	Risk                    Risk
	Sensitivity             Sensitivity
	Budget                  Budget
	PreviousReviewedOutcome Outcome
}

// Recommendation is explainable advisory output. Apply is always false: a
// caller must run the deterministic policy before using it to select a model.
type Recommendation struct {
	Tier              core.Tier
	Confidence        float64
	Reasons           []string
	RequiresReview    bool
	BudgetConstrained bool
	Apply             bool
}

// Evaluate emits a monotonic recommendation from structured signals. It can
// keep or elevate BaseTier, never lower it. That makes it safe to run in
// shadow mode before a policy integration exists.
func Evaluate(s Signals) Recommendation {
	tier := validTier(s.BaseTier)
	confidence := clamp(s.ClassifierConfidence)
	reasons := make([]string, 0, 4)
	requiresReview := false

	if s.Risk >= RiskHigh {
		tier = atLeast(tier, core.TierFrontier)
		reasons = append(reasons, "high_risk")
		requiresReview = true
		confidence = atLeastFloat(confidence, 0.8)
	}
	if s.Sensitivity >= SensitivityConfidential {
		reasons = append(reasons, "sensitive_data")
		requiresReview = true
		confidence = atLeastFloat(confidence, 0.8)
	}
	if s.PreviousReviewedOutcome == OutcomeRetry || s.PreviousReviewedOutcome == OutcomeFailed {
		tier = atLeast(tier, nextTier(tier))
		reasons = append(reasons, "reviewed_rework")
		confidence = atLeastFloat(confidence, 0.7)
	}
	budgetConstrained := s.Budget == BudgetTight || s.Budget == BudgetExhausted
	if budgetConstrained {
		reasons = append(reasons, "budget_constrained")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "no_advisory_signal")
	}

	return Recommendation{
		Tier:              tier,
		Confidence:        confidence,
		Reasons:           reasons,
		RequiresReview:    requiresReview,
		BudgetConstrained: budgetConstrained,
		Apply:             false,
	}
}

func validTier(t core.Tier) core.Tier {
	if t < core.TierSmall || t > core.TierFrontier {
		return core.TierMid
	}
	return t
}

func atLeast(current, floor core.Tier) core.Tier {
	if floor > current {
		return floor
	}
	return current
}

func nextTier(t core.Tier) core.Tier {
	if t < core.TierFrontier {
		return t + 1
	}
	return core.TierFrontier
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func atLeastFloat(current, floor float64) float64 {
	if floor > current {
		return floor
	}
	return current
}
