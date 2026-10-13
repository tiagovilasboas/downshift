// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"fmt"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// finalDecisionError re-runs the guardrails on the decision Route returned.
// Route validates before the tier floor and adapt-memory hooks run, so a hook
// that changes the tier without revalidating would leave SafeVerdict stale.
// The invariant: the safe verdict a caller reads is the one Check gives for the
// final fields, and the recorded corrections are exactly Check's violations.
func finalDecisionError(d core.Decision) error {
	rep := core.Check(d)
	if rep.Safe != d.SafeVerdict {
		return fmt.Errorf("stale SafeVerdict: Route=%s Check(final)=%s (tier=%s verdict=%s conf=%v corr=%v)",
			d.SafeVerdict, rep.Safe, d.Tier, d.Verdict, d.Confident, d.Corrections)
	}
	if len(rep.Violations) != len(d.Corrections) {
		return fmt.Errorf("stale Corrections: Route=%v Check(final)=%d violations", d.Corrections, len(rep.Violations))
	}
	for i, v := range rep.Violations {
		if d.Corrections[i] != v.Rule {
			return fmt.Errorf("Corrections[%d]=%s, Check says %s", i, d.Corrections[i], v.Rule)
		}
	}
	return nil
}

func finalInvariantPrompts() []string {
	var ps []string
	for _, c := range corpus {
		ps = append(ps, c.prompt)
	}
	return append(ps, "help", "hook", "", "rename the userId variable", "Atualize a descrição do botão de login")
}

var finalInvariantCurrents = []string{
	"", "claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5",
}

// Every hook and opt-in layer that runs inside Route: tier floor (any target),
// naive-Bayes upshift and downshift, with the real catalog.
func TestFinalDecision_GuardrailsHoldAfterEveryRouteLayer(t *testing.T) {
	floors := map[string]func(*core.Decision, string, core.Resolver){
		"none":      nil,
		"floor-mid": func(d *core.Decision, cur string, r core.Resolver) { d.RaiseTier(core.TierMid, cur, r) },
		"floor-frontier": func(d *core.Decision, cur string, r core.Resolver) {
			d.RaiseTier(core.TierFrontier, cur, r)
		},
	}
	old := core.TierFloorHook
	defer func() { core.TierFloorHook = old }()
	n := 0
	for fname, hook := range floors {
		for _, nb := range []string{"", "1"} {
			t.Run(fmt.Sprintf("%s/nb=%q", fname, nb), func(t *testing.T) {
				t.Setenv(core.NBUpshiftEnv, nb)
				t.Setenv(core.NBDownshiftEnv, nb)
				core.TierFloorHook = hook
				for _, p := range finalInvariantPrompts() {
					for _, cur := range finalInvariantCurrents {
						n++
						d := core.Route(p, "claude-code", cur, cat)
						if err := finalDecisionError(d); err != nil {
							t.Errorf("prompt=%.40q current=%q: %v", p, cur, err)
						}
					}
				}
			})
		}
	}
	t.Logf("%d decisions checked", n)
}

// Meta-test: the invariant must catch a hook that moves the tier without
// revalidating, which is what promoting adapt memory out of shadow would do.
func TestFinalDecision_InvariantCatchesAnUnvalidatedTierChange(t *testing.T) {
	old := core.MemoryAdjustHook
	defer func() { core.MemoryAdjustHook = old }()
	core.MemoryAdjustHook = func(_ string, d *core.Decision, _ core.Resolver) { d.Tier = core.TierFrontier }
	caught := 0
	for _, p := range finalInvariantPrompts() {
		d := core.Route(p, "claude-code", "", cat)
		if finalDecisionError(d) != nil {
			caught++
		}
	}
	if caught == 0 {
		t.Fatal("an unvalidated tier change slipped past the invariant")
	}
}
