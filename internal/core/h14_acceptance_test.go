// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// Downshift by Tiago de Carvalho Vilas Boas
// https://github.com/tiagovilasboas/downshift

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

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
	for _, p := range h14PureEnglishTrivialPrompts() {
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

// M3 unsafe confident from a frontier baseline: confident rewrite strictly below frontier tier.
func TestH14Acceptance_NoUnsafeConfidentAcrossHarnesses(t *testing.T) {
	for _, h := range core.AllHarnessCaps() {
		t.Run(h.Harness, func(t *testing.T) {
			assertH14NoUnsafeConfidentFromFrontier(t, h.Harness, h.Caps, h14UnsafeBucketPrompts())
		})
	}
}

func assertH14NoUnsafeConfidentFromFrontier(t *testing.T, harness string, caps core.HarnessCapabilities, prompts []string) {
	t.Helper()
	small := catID(harness, core.TierSmall)
	mid := catID(harness, core.TierMid)
	frontier := catID(harness, core.TierFrontier)
	session := core.KnownSession([]string{small, mid, frontier})
	for _, p := range prompts {
		d := core.Route(p, harness, frontier, cat)
		if d.Confident && d.Tier < core.TierFrontier && checkWrites(d) {
			t.Errorf("unsafe confident below frontier: prompt=%.50q d=%+v", p, d)
		}
		plan := d.PlanForSession(caps, cat, session)
		if plan.RewriteModel && d.Confident && d.Tier < core.TierFrontier {
			t.Errorf("plan rewrites below frontier while confident: harness=%s prompt=%.50q plan=%+v", harness, p, plan)
		}
	}
}

// (3) Route tier invariant on H14 prompts (Check matches SafeVerdict/Corrections).
func TestH14Acceptance_RouteFinalDecisionInvariant(t *testing.T) {
	prompts := append(h14UnsafeBucketPrompts(), h14PureEnglishTrivialPrompts()...)
	for _, p := range prompts {
		for _, cur := range finalInvariantCurrents {
			d := core.Route(p, "claude-code", cur, cat)
			if err := finalDecisionError(d); err != nil {
				t.Errorf("prompt=%.60q current=%q: %v", p, cur, err)
			}
		}
	}
}
