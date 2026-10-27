// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// RoutingPolicy names the deterministic routing policy whose decision a
// reason-code trace explains. Bump it when the meaning of a code changes or a
// rule that can hold or lift a decision is added, so old traces stay readable.
const RoutingPolicy = "deterministic-v1"

// Reason codes are stable, prompt-free names for the signals and rules that
// shaped one decision. They are derived only from decision fields (internal/telemetry/fromcore); no
// prompt text, matched word, risk category name or path ever enters a code.
const (
	ReasonClassifierConfident   = "CLASSIFIER_CONFIDENT"
	ReasonClassifierUnconfident = "CLASSIFIER_UNCONFIDENT"
	ReasonNoSignal              = "NO_SIGNAL"
	ReasonTaskNotVisible        = "TASK_NOT_VISIBLE"
	ReasonSemanticBoost         = "SEMANTIC_BOOST"
	// ReasonTaskFocus marks a long brief whose actionable task, with the rest
	// of the brief weighed as context, lowered the class or its confidence.
	ReasonTaskFocus           = "TASK_FOCUS"
	ReasonUnsupportedLanguage = "UNSUPPORTED_LANGUAGE"
	ReasonRiskCategory        = "RISK_CATEGORY"
	ReasonRiskFloor           = "RISK_FLOOR"
	ReasonNBUpshiftApplied    = "NB_UPSHIFT_APPLIED"
	ReasonNBDownshiftApplied  = "NB_DOWNSHIFT_APPLIED"
	ReasonTierFloor           = "TIER_FLOOR"
	ReasonCreditHeld          = "CREDIT_HELD"
	// ReasonGuessHeld marks a downshift held because its baseline was guessed
	// and the target was not the small tier (Decision.GuessHeld).
	ReasonGuessHeld = "HELD_GUESSED_BASELINE"
	// ReasonPaused marks a decision made while the developer had routing paused
	// for the session (`downshift session --pause`): classified and logged as a
	// control-group baseline, the spawn left unchanged.
	ReasonPaused = "PAUSED"
	// ReasonBaselineFromSettings marks a decision whose baseline model was
	// inferred from the configured session model (env or settings) because the
	// transcript had none; a --model flag or /model can make it wrong.
	ReasonBaselineFromSettings = "BASELINE_FROM_SETTINGS"

	// ReasonAgentPinsModel marks a decision for a child whose agent definition
	// pins its model: the baseline is known and the spawn is left unchanged.
	ReasonAgentPinsModel = "AGENT_PINS_MODEL"
	// ReasonAgentTypeHint marks a decision whose tier came from the subagent
	// type (HarnessCapabilities.AgentTypeHints) because the text read was unsure.
	ReasonAgentTypeHint = "AGENT_TYPE_HINT"
	// ReasonHeldUnmapped stands for a guardrail rule this version has no code
	// for, so a hold is never silently dropped from the trace.
	ReasonHeldUnmapped = "HELD_UNMAPPED_RULE"

	// Hold codes name the guardrail rule that held a decision (Decision.Corrections).
	ReasonHeldUnconfidentDownshift       = "HELD_UNCONFIDENT_DOWNSHIFT"
	ReasonHeldComplexDownshift           = "HELD_COMPLEX_DOWNSHIFT"
	ReasonHeldForeignModel               = "HELD_FOREIGN_MODEL"
	ReasonHeldRiskDownshift              = "HELD_RISK_DOWNSHIFT"
	ReasonHeldUnconfidentUnknownSmall    = "HELD_UNCONFIDENT_UNKNOWN_SMALL"
	ReasonHeldUnconfidentUnknownFrontier = "HELD_UNCONFIDENT_UNKNOWN_FRONTIER"
	ReasonHeldUnconfidentUpshift         = "HELD_UNCONFIDENT_UPSHIFT"
	ReasonHeldUnconfidentUnknownMid      = "HELD_UNCONFIDENT_UNKNOWN_MID"
)
