// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// Guardrail rule IDs. They name why a spawn was held and are stable
// telemetry history (telemetry/fromcore maps each to a reason code), so they
// are never renumbered: R3 was retired, so numbering runs R1, R2, R4 by design.
//
// Enforcement lives in the production decider, not here: internal/routing/guard
// refuses a move on doubt (unconfident decision or unknown requested tier) and
// refuses any target above the requested tier (never widen), and
// internal/routing/decide keeps the requested model when the guard refuses.
// The only upshifts are deterministic: the project tier floor and the opt-in
// raise of a decisive COMPLEX task (routing/decide floor.go, class.go).
const (
	// RuleUnconfidentDowngrade fires when a DOWNSHIFT verdict rests on a
	// low-confidence classification. Acting on doubt trades quality for
	// savings; the safe action holds the current model.
	RuleUnconfidentDowngrade = "R1_UNCONFIDENT_DOWNSHIFT"
	// RuleComplexDowngrade fires when a COMPLEX task would be downshifted.
	// Complex work must never move to a cheaper tier on any signal.
	RuleComplexDowngrade = "R2_COMPLEX_DOWNSHIFT"
	// RuleForeignModel fires when the recommended model belongs to a
	// different harness than the decision. Adapters must never write a
	// model ID that does not belong to the calling harness.
	RuleForeignModel = "R4_FOREIGN_MODEL"
	// RuleRiskDowngrade fires when a task that touches a risk category
	// (secrets, auth, crypto, payments, PII, destructive ops, prompt
	// injection; see risk.go) would be downshifted.
	RuleRiskDowngrade = "R5_RISK_DOWNSHIFT"
	// RuleUnconfidentUnknownSmall fires when no current model is known
	// (VerdictUnknown), the classification is not confident, and the
	// recommended tier is below mid. The child would otherwise inherit the
	// harness default (often the parent's model), so writing a small model
	// on doubt is an unconfident downshift in disguise. The safe action
	// leaves the spawn unchanged.
	RuleUnconfidentUnknownSmall = "R6_UNCONFIDENT_UNKNOWN_SMALL"
	// RuleUnconfidentUnknownFrontier fires when no current model is known, the
	// classification is not confident and the recommendation is the frontier
	// tier. The child would inherit the harness default, so writing the most
	// expensive tier on doubt is an unconfident upshift in disguise: an
	// uncertain result must not spend money in either direction. A task a
	// deterministic lift put there (deterministicLift) is not a weak signal and
	// stays writable.
	RuleUnconfidentUnknownFrontier = "R7_UNCONFIDENT_UNKNOWN_FRONTIER"
	// RuleUnconfidentUpshift fires when an UPSHIFT verdict rests on a
	// low-confidence classification. It is the mirror of R1: acting on doubt
	// spends money for a quality the task may not need. The safe action holds
	// the current model. A task a deterministic lift
	// put there (deterministicLift) is not a weak signal.
	RuleUnconfidentUpshift = "R8_UNCONFIDENT_UPSHIFT"
	// RuleUnconfidentUnknownMid fires when no current model is known, the
	// classification is not confident and the recommendation is the mid tier.
	// A prompt with no recognised signal falls to MEDIUM by default, and the
	// child would inherit the harness default, so writing mid on doubt can
	// move a task up for no reason (an unnecessary upshift in disguise). The
	// safe action leaves the spawn unchanged. A task a deterministic lift put
	// there (deterministicLift) is not a weak signal and stays writable.
	RuleUnconfidentUnknownMid = "R9_UNCONFIDENT_UNKNOWN_MID"
)
