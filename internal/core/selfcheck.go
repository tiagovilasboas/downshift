// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package core

// Self-correction validates a routing Decision against the established
// guardrails after classification, before any adapter acts on it.
//
// The core stays deterministic: Check is a pure function with no network,
// no LLM, and no side effects. It reports violations and computes the safe
// action, but it never rewrites history — Decision.Verdict keeps the
// classified value so telemetry shows what the classifier said, while
// Decision.SafeVerdict carries what adapters are allowed to apply.
//
// Enforcement note: the safe action flows through ShouldRewriteModel, so
// every adapter using PlanForSession inherits it with no code changes.
// Deliberate session fallbacks (e.g. cheapest-session-model on a blocked
// downgrade) are untouched — Check only observes and records those paths.
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
)

// Violation is one guardrail breach found in a Decision.
type Violation struct {
	Rule   string // one of the Rule* constants above
	Detail string // human-readable context for logs and telemetry
}

// Report is the outcome of validating a Decision.
type Report struct {
	Violations []Violation // empty when the decision is clean
	Safe       Verdict     // action adapters may apply
	Corrected  bool        // true when Safe differs from the verdict
}

// Check validates d against the guardrail rules and returns the safe action.
// It is pure and total: every Decision maps to exactly one Report, and a
// clean decision reports Safe == d.Verdict with Corrected == false.
func Check(d Decision) Report {
	var violations []Violation

	if d.Verdict == VerdictDownshift && !d.Confident {
		violations = append(violations, Violation{
			Rule:   RuleUnconfidentDowngrade,
			Detail: "downshift verdict without confident classification",
		})
	}
	if d.Verdict == VerdictDownshift && d.Complexity == Complex {
		violations = append(violations, Violation{
			Rule:   RuleComplexDowngrade,
			Detail: "complex task must not move to a cheaper tier",
		})
	}
	if d.Model.ID != "" && d.Model.Harness != "" && d.Harness != "" &&
		d.Model.Harness != d.Harness {
		violations = append(violations, Violation{
			Rule:   RuleForeignModel,
			Detail: "recommended model belongs to another harness",
		})
	}

	if len(violations) == 0 {
		return Report{Safe: d.Verdict}
	}
	return Report{Violations: violations, Safe: VerdictOK, Corrected: true}
}
