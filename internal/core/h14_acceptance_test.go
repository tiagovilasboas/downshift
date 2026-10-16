// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// harness-downshift by Tiago de Carvalho Vilas Boas
// https://github.com/tiagovilasboas/downshift

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// h14MixedRiskPrompts mirror the M3 mixed-01 bucket: a trivial English stub,
// one foreign prose cue (partialForeignLanguage, not unsupportedLanguage),
// and risk vocabulary the EN/PT tables may miss in the foreign tail.
func h14MixedRiskPrompts() []string {
	return []string{
		"rename this helper; Revisa los permisos de acceso a los datos de los clientes.",
		"rename this helper; Lösche personenbezogene Kundendaten gemäß Aufbewahrungsrichtlinie.",
		"format this helper; Behebe SQL-Injektion in der Kundensuche.",
	}
}

func h14PureEnglishTrivial() []string {
	return []string{
		"rename the variable userId to userIdentifier",
		"rename foo to bar",
		"format main.go",
	}
}

// (1) Mixed EN + foreign risk must not produce a confident small-tier write.
func TestH14Acceptance_MixedWithRisk_LosesConfidenceOrLiftsTier(t *testing.T) {
	frontier := catID("claude-code", core.TierFrontier)
	for _, p := range h14MixedRiskPrompts() {
		cls := core.Classify(p)
		if !cls.UnsupportedLanguage {
			t.Errorf("mixed H14 prompt must set UnsupportedLanguage: %q cls=%+v", p, cls)
		}
		d := core.Route(p, "claude-code", frontier, cat)
		if cls.Confident && d.Tier == core.TierSmall && checkWrites(d) {
			t.Errorf("unsafe confident small downshift on %q: cls=%+v tier=%s", p, cls, d.Tier)
		}
		if checkWrites(d) && d.Tier == core.TierSmall {
			t.Errorf("Route(%q) wrote small tier from frontier: %+v", p, d)
		}
	}
}

// (2) Pure English trivial prompts keep confident classification and may downshift.
func TestH14Acceptance_PureEnglishTrivial_Unchanged(t *testing.T) {
	frontier := catID("claude-code", core.TierFrontier)
	for _, p := range h14PureEnglishTrivial() {
		cls := core.Classify(p)
		if !cls.Confident || cls.UnsupportedLanguage || cls.Complexity != core.Trivial {
			t.Errorf("Classify(%q): want confident TRIVIAL, got cx=%s conf=%v unsupported=%v",
				p, cls.Complexity, cls.Confident, cls.UnsupportedLanguage)
		}
		d := core.Route(p, "claude-code", frontier, cat)
		if d.Tier != core.TierSmall || !checkWrites(d) {
			t.Errorf("Route(%q): want confident trivial downshift to small, got tier=%s writes=%v",
				p, d.Tier, checkWrites(d))
		}
	}
}

// (3) Route tier invariant on H14 prompts (Check matches SafeVerdict/Corrections).
func TestH14Acceptance_RouteFinalDecisionInvariant(t *testing.T) {
	prompts := append(h14MixedRiskPrompts(), h14PureEnglishTrivial()...)
	prompts = append(prompts,
		"Implement idempotent payment refunds with audit evidence.",
		"Revise as permissões de acesso aos dados dos clientes.",
	)
	for _, p := range prompts {
		for _, cur := range finalInvariantCurrents {
			d := core.Route(p, "claude-code", cur, cat)
			if err := finalDecisionError(d); err != nil {
				t.Errorf("prompt=%.60q current=%q: %v", p, cur, err)
			}
		}
	}
}
