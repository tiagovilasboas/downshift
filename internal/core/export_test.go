// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// Test-only helpers. ShouldRewriteModel and ShouldApplyEffort are not the
// production gate: no adapter calls them. The real gate is
// selectSessionTarget (session_select.go), which returns no target when
// Check set SafeVerdict to OK, plus the adapters' Corrections checks. These
// predicates stay as a compact oracle for the invariant tests in core_test;
// they mirror the guardrail conditions Check enforces (R1, R6, R7, R9).

// ShouldRewriteModel reports whether the decision's action, after
// self-correction, calls for replacing the harness model input. It is
// harness-agnostic: upshifts, downshifts, and missing source models behave
// the same for every harness.
//
// The gate consults the self-corrected action, so a decision held by a
// guardrail is never applied even when the classified verdict suggests a
// rewrite. Hand-built Decisions without a Check report fall back to the
// classified verdict for backward compatibility.
func (d Decision) ShouldRewriteModel() bool {
	if d.Model.ID == "" {
		return false
	}
	action := d.Verdict
	if d.Checked {
		action = d.SafeVerdict
	}
	// An uncertain classifier result must not spend money in either direction.
	// A downgrade stays put so a hard task is not underpowered. An upshift
	// stays put so a small model is not replaced by a dearer tier on a weak
	// signal. Confident rewrites still apply. Plan falls back when this
	// returns false.
	// Exception: a risk-floored or graphify-escalated task may move up — both
	// are deterministic rules, not weak signals — and so may an NB upshift the
	// user opted into with DOWNSHIFT_NB_UPSHIFT=1.
	if action == VerdictUpshift && (d.RiskFloor || d.GraphEscalated || d.NBUpshift.Applied) {
		return true
	}
	if (action == VerdictDownshift || action == VerdictUpshift) && !d.Confident {
		return false
	}
	// No current model known: the child would inherit the harness default
	// (often the parent's model). An unconfident small-tier recommendation
	// must not be written over that default (guardrail R6), and neither may an
	// unconfident mid (R9) or frontier (R7): a doubtful result must not spend
	// money, or save it, in either direction. Confident results and tasks a
	// deterministic rule lifted still apply.
	if action == VerdictUnknown && (unconfidentUnknownBelowMid(d) || unconfidentUnknownMid(d) || unconfidentUnknownFrontier(d)) {
		return false
	}
	return action == VerdictDownshift || action == VerdictUpshift || action == VerdictUnknown
}

// ShouldApplyEffort reports whether the decision carries an effort that a
// harness with native effort controls could inject. This includes VerdictOK so a child does
// not accidentally inherit a lower effort from a same-tier parent.
func (d Decision) ShouldApplyEffort() bool {
	return d.Model.ID != ""
}
