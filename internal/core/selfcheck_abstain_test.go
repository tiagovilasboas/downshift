// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

// A risk abstention exempts R7/R9 only while the tier is the deterministic
// read. When the semantic boost lifted the tier after the abstention, the
// frontier (or mid) pick is a guess and the hold must fire.
func TestCheck_RiskAbstainDoesNotCoverSemanticLift(t *testing.T) {
	base := Decision{Harness: "claude-code", Tier: TierFrontier, Verdict: VerdictUnknown, Confident: false, Guards: Guards{RiskAbstain: true}}

	if rep := Check(base); rep.Corrected {
		t.Fatalf("abstained deterministic frontier must stay exempt, got %v", rep.Violations)
	}

	lifted := base
	lifted.Guards.SemanticLift = true
	rep := Check(lifted)
	if !rep.Corrected || !hasRule(rep, RuleUnconfidentUnknownFrontier) {
		t.Fatalf("semantic lift after abstention must trip R7, got %+v", rep)
	}

	mid := lifted
	mid.Tier = TierMid
	rep = Check(mid)
	if !rep.Corrected || !hasRule(rep, RuleUnconfidentUnknownMid) {
		t.Fatalf("semantic lift to mid after abstention must trip R9, got %+v", rep)
	}
}

func hasRule(rep Report, rule string) bool {
	for _, v := range rep.Violations {
		if v.Rule == rule {
			return true
		}
	}
	return false
}
