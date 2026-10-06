// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// A confident MEDIUM task that touches a risk category (here crypto) must
// not be downshifted from frontier to mid: R5 holds it.
func TestRoute_RiskCategoryHoldsMediumDownshift(t *testing.T) {
	prompt := "add a feature to encrypt exported reports with the tenant key"
	d := core.Route(prompt, "claude-code", catID("claude-code", core.TierFrontier), cat)
	if d.Verdict != core.VerdictDownshift || d.RiskFloor {
		t.Fatalf("precondition: want a classified downshift without risk floor, got verdict=%v floor=%v", d.Verdict, d.RiskFloor)
	}
	if d.ShouldRewriteModel() {
		t.Fatal("risk-category downshift must be held")
	}
	held := false
	for _, r := range d.Corrections {
		if r == core.RuleRiskDowngrade {
			held = true
		}
	}
	if !held {
		t.Fatalf("Corrections = %v, want %s", d.Corrections, core.RuleRiskDowngrade)
	}
}

// A downshift with no risk category is unaffected.
func TestRoute_NoRiskCategoryDownshiftStillApplies(t *testing.T) {
	d := core.Route("rename the userId variable to userIdentifier", "claude-code", catID("claude-code", core.TierFrontier), cat)
	if len(d.Risk) != 0 || !d.ShouldRewriteModel() {
		t.Fatalf("plain rename must still downshift, risk=%v corrections=%v", d.Risk, d.Corrections)
	}
}
