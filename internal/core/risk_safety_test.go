// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// appliedSmallDownshift reports whether the final decision writes a small-tier model.
func appliedSmallDownshift(d core.Decision) bool {
	return d.Tier == core.TierSmall && checkWrites(d)
}

// A risk signal past the 32 KB classification cap must still keep the task off
// the small tier. Route sees it (d.Risk uses the full prompt) but the risk floor
// only sees the head, and R5 covers only a DOWNSHIFT verdict, so with no known
// current model the small model is written.
func TestRiskSafety_SignalPastClassifyCapKeepsTaskOffSmall(t *testing.T) {
	small := catID("claude-code", core.TierSmall)
	session := core.KnownSession([]string{small, catID("claude-code", core.TierMid), catID("claude-code", core.TierFrontier)})
	signal := " and rotate the production database credentials"
	control := "rename the variable foo to bar" + signal
	long := "rename the variable foo to bar. " + strings.Repeat("lorem ipsum dolor sit amet ", 1500) + signal
	if strings.Index(long, signal) < 32<<10 {
		t.Fatal("precondition: signal must sit past the 32 KB cap")
	}
	for name, p := range map[string]string{"control": control, "past-cap": long} {
		d := core.Route(p, "claude-code", "", cat)
		plan := d.PlanSession(core.ClaudeCodeCaps, cat, session, "")
		if plan.RewriteModel && plan.Model.ID == small {
			t.Errorf("%s: wrote small model %s (tier=%v risk=%v floor=%v verdict=%v safe=%v)",
				name, plan.Model.ID, d.Tier, d.Risk, d.RiskFloor, d.Verdict, d.SafeVerdict)
		}
	}
}

// "drop the users table" is destructive, but the pattern only matches "drop
// [the] table", so a named table falls through and a rename downshifts.
func TestRiskSafety_DropNamedTableIsDestructive(t *testing.T) {
	d := core.Route("rename the variable and drop the users table", "claude-code", catID("claude-code", core.TierFrontier), cat)
	if appliedSmallDownshift(d) {
		t.Errorf("applied small downshift: cx=%v risk=%v corrections=%v", d.Complexity, d.Risk, d.Corrections)
	}
}

// PT-BR phrasings of risks the English table catches end on the small tier.
// Each EN control is held by the risk floor today.
func TestRiskSafety_PortugueseParity(t *testing.T) {
	front := catID("claude-code", core.TierFrontier)
	pairs := []struct{ en, pt string }{
		{"rename the variable in the code that charges the customer card", "renomeie a variável no código que cobra o cartão do cliente"},
		{"rename the variable and delete the production data", "renomeie a variável e apague os dados de produção"},
		{"rename the access token field", "renomeie o campo token de acesso"},
		{"rename the variable and disable the certificate check", "renomeie a variável e desative a verificação do certificado"},
	}
	for _, p := range pairs {
		if d := core.Route(p.en, "claude-code", front, cat); appliedSmallDownshift(d) {
			t.Fatalf("EN control downshifted: %q", p.en)
		}
		if d := core.Route(p.pt, "claude-code", front, cat); appliedSmallDownshift(d) {
			t.Errorf("PT applied small downshift: %q cx=%v risk=%v", p.pt, d.Complexity, d.Risk)
		}
	}
}

// Concurrency changes ride a confident TRIVIAL "rename" to the small tier.
func TestRiskSafety_ConcurrencyIsNotTrivial(t *testing.T) {
	front := catID("claude-code", core.TierFrontier)
	for _, p := range []string{
		"rename the variable and fix the race",
		"rename the variable and remove the mutex lock in Get",
		"rename the counter and replace atomic.AddInt64 with count++",
	} {
		if d := core.Route(p, "claude-code", front, cat); appliedSmallDownshift(d) {
			t.Errorf("applied small downshift: %q cx=%v conf=%v risk=%v", p, d.Complexity, d.Confident, d.Risk)
		}
	}
}

// Guardrail check (passes before and after the fix): one risky sentence in the middle of a
// long trivial prompt, inside the cap, is held.
func TestRiskSafety_MixedLongPromptInsideCapHeld(t *testing.T) {
	pad := strings.Repeat("lorem ipsum dolor sit amet ", 200)
	p := "rename the variable foo to bar. " + pad + " also rotate the database credentials. " + pad
	d := core.Route(p, "claude-code", catID("claude-code", core.TierFrontier), cat)
	if appliedSmallDownshift(d) || !d.RiskFloor {
		t.Errorf("mixed prompt: floor=%v risk=%v writes=%v", d.RiskFloor, d.Risk, checkWrites(d))
	}
}
