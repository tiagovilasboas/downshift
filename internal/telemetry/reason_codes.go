// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"sort"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// RoutingPolicy names the deterministic routing policy whose decision a
// reason-code trace explains. Bump it when the meaning of a code changes or a
// rule that can hold or lift a decision is added, so old traces stay readable.
const RoutingPolicy = "deterministic-v1"

// Reason codes are stable, prompt-free names for the signals and rules that
// shaped one decision. They are derived only from core.Decision fields; no
// prompt text, matched word, risk category name or path ever enters a code.
const (
	ReasonClassifierConfident   = "CLASSIFIER_CONFIDENT"
	ReasonClassifierUnconfident = "CLASSIFIER_UNCONFIDENT"
	ReasonNoSignal              = "NO_SIGNAL"
	ReasonSemanticBoost         = "SEMANTIC_BOOST"
	ReasonUnsupportedLanguage   = "UNSUPPORTED_LANGUAGE"
	ReasonRiskCategory          = "RISK_CATEGORY"
	ReasonRiskFloor             = "RISK_FLOOR"
	ReasonNBUpshiftApplied      = "NB_UPSHIFT_APPLIED"
	ReasonNBDownshiftApplied    = "NB_DOWNSHIFT_APPLIED"
	ReasonTierFloor             = "TIER_FLOOR"
	ReasonCreditHeld            = "CREDIT_HELD"
	// ReasonGuessHeld marks a downshift held because its baseline was guessed
	// and the target was not the small tier (Decision.GuessHeld).
	ReasonGuessHeld = "HELD_GUESSED_BASELINE"
	// ReasonPaused marks a decision made while the developer had routing paused
	// for the session (`downshift session --pause`): classified and logged as a
	// control-group baseline, the spawn left unchanged.
	ReasonPaused = "PAUSED"
	// ReasonScoreRouted marks a model chosen by the public-score capability
	// path (DOWNSHIFT_SCORE_ROUTING=1). A shadow pick is never a reason: it
	// has its own event field, score_would.
	ReasonScoreRouted = "SCORE_ROUTED"
	// ReasonBaselineFromSettings marks a decision whose baseline model was
	// inferred from the configured session model (env or settings) because the
	// transcript had none; a --model flag or /model can make it wrong.
	ReasonBaselineFromSettings = "BASELINE_FROM_SETTINGS"
	// ReasonHeldUnmapped stands for a guardrail rule this version has no code
	// for, so a hold is never silently dropped from the trace.
	ReasonHeldUnmapped = "HELD_UNMAPPED_RULE"
)

// heldReason maps each self-check rule ID to its hold code.
var heldReason = map[string]string{
	core.RuleUnconfidentDowngrade:       "HELD_UNCONFIDENT_DOWNSHIFT",
	core.RuleComplexDowngrade:           "HELD_COMPLEX_DOWNSHIFT",
	core.RuleForeignModel:               "HELD_FOREIGN_MODEL",
	core.RuleRiskDowngrade:              "HELD_RISK_DOWNSHIFT",
	core.RuleUnconfidentUnknownSmall:    "HELD_UNCONFIDENT_UNKNOWN_SMALL",
	core.RuleUnconfidentUnknownFrontier: "HELD_UNCONFIDENT_UNKNOWN_FRONTIER",
	core.RuleUnconfidentUpshift:         "HELD_UNCONFIDENT_UPSHIFT",
	core.RuleUnconfidentUnknownMid:      "HELD_UNCONFIDENT_UNKNOWN_MID",
}

// ReasonCodes returns the sorted, de-duplicated reason codes for d: the
// signals that fired, the deterministic lifts that applied and the guardrails
// that held the decision. Shadow-only opinions (NB not applied, adapt memory)
// are not reasons; they have their own event fields.
func ReasonCodes(d core.Decision) []string {
	set := make(map[string]bool)
	if d.Confident {
		set[ReasonClassifierConfident] = true
	} else {
		set[ReasonClassifierUnconfident] = true
	}
	if d.NoSignal {
		set[ReasonNoSignal] = true
	}
	if d.UnsupportedLanguage {
		set[ReasonUnsupportedLanguage] = true
	}
	if d.Guards.SemanticLift {
		set[ReasonSemanticBoost] = true
	}
	if len(d.Risk) > 0 {
		set[ReasonRiskCategory] = true
	}
	if d.Guards.RiskFloor {
		set[ReasonRiskFloor] = true
	}
	if d.Shadow.NBUpshift.Applied {
		set[ReasonNBUpshiftApplied] = true
	}
	if d.Shadow.NBDownshift.Applied {
		set[ReasonNBDownshiftApplied] = true
	}
	if d.Guards.PolicyFloor {
		set[ReasonTierFloor] = true
	}
	if d.Session.CreditHeld {
		set[ReasonCreditHeld] = true
	}
	if d.Baseline.GuessHeld {
		set[ReasonGuessHeld] = true
	}
	if d.Shadow.ScoreApplied {
		set[ReasonScoreRouted] = true
	}
	if d.Baseline.FromSettings {
		set[ReasonBaselineFromSettings] = true
	}
	for _, rule := range d.Corrections {
		if code, ok := heldReason[rule]; ok {
			set[code] = true
		} else {
			set[ReasonHeldUnmapped] = true
		}
	}
	codes := make([]string, 0, len(set))
	for code := range set {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}
