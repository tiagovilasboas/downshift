// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// Test-only helper. ShouldApplyEffort is not the production gate: no adapter
// calls it. Whether a model is written is decided by Check (guardrails R1 to
// R9, selfcheck.go) and enforced by selectSessionTarget (session_select.go);
// tests assert on Check(d).Safe, not on a second predicate.

// ShouldApplyEffort reports whether the decision carries an effort that a
// harness with native effort controls could inject. This includes VerdictOK so a child does
// not accidentally inherit a lower effort from a same-tier parent.
func (d Decision) ShouldApplyEffort() bool {
	return d.Model.ID != ""
}
