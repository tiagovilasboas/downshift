// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

// An unconfident small-tier decision with no current model must never be
// written: the child would otherwise run below the harness default (R6).
func TestRoute_UnknownModelUnconfidentSmallIsHeld(t *testing.T) {
	d := core.Route("write a function to parse dates", "claude-code", "", cat)
	if d.Tier != core.TierSmall || d.Confident {
		t.Fatalf("precondition: want unconfident small, got tier=%v confident=%v", d.Tier, d.Confident)
	}
	if d.ShouldRewriteModel() {
		t.Fatal("unconfident small recommendation without a current model must not rewrite")
	}
	if d.SafeVerdict != core.VerdictOK {
		t.Fatalf("SafeVerdict = %v, want OK", d.SafeVerdict)
	}
	found := false
	for _, r := range d.Corrections {
		if r == core.RuleUnconfidentUnknownSmall {
			found = true
		}
	}
	if !found {
		t.Fatalf("Corrections = %v, want %s", d.Corrections, core.RuleUnconfidentUnknownSmall)
	}
	session := core.KnownSession([]string{catID("claude-code", core.TierSmall), catID("claude-code", core.TierMid), catID("claude-code", core.TierFrontier)})
	plan := d.PlanForSession(core.ClaudeCodeCaps, cat, session)
	if plan.RewriteModel {
		t.Fatalf("plan must not rewrite, got %+v", plan)
	}
}

// Mid-tier recommendations stay writable on doubt: they never move a task
// below the mid tier.
func TestRoute_UnknownModelUnconfidentMidStillRoutes(t *testing.T) {
	d := core.Route("look at the logs folder and tell me what you see", "claude-code", "", cat)
	if d.Tier != core.TierMid || d.Confident {
		t.Fatalf("precondition: want unconfident mid, got tier=%v confident=%v", d.Tier, d.Confident)
	}
	if !d.ShouldRewriteModel() {
		t.Fatal("unconfident mid recommendation may be written")
	}
	if len(d.Corrections) != 0 {
		t.Fatalf("unexpected corrections %v", d.Corrections)
	}
}

// A confident small recommendation without a current model still routes.
func TestRoute_UnknownModelConfidentSmallRoutes(t *testing.T) {
	d := core.Route("rename the userId variable to userIdentifier", "claude-code", "", cat)
	if d.Tier != core.TierSmall || !d.Confident {
		t.Fatalf("precondition: want confident small, got tier=%v confident=%v", d.Tier, d.Confident)
	}
	if !d.ShouldRewriteModel() {
		t.Fatal("confident small recommendation must rewrite")
	}
}
