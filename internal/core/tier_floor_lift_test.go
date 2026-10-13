// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"slices"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// An operator tier floor is a deterministic lift: R7 and R8 must not hold it,
// whatever the classifier thinks of a vague prompt. Reproduces the review bug
// where a haiku parent (R8) or an unknown parent (R7) kept the floor from ever
// being written. Goes through Route + PlanSession, as every adapter does.
func TestTierFloor_UnconfidentPromptStillRewrites(t *testing.T) {
	prev := core.TierFloorHook
	t.Cleanup(func() { core.TierFloorHook = prev })
	core.TierFloorHook = func(d *core.Decision, current string, r core.Resolver) {
		d.RaiseTier(core.TierFrontier, current, r)
	}

	small := catID(ccHarness, core.TierSmall)
	mid := catID(ccHarness, core.TierMid)
	frontier := catID(ccHarness, core.TierFrontier)
	ladder := core.KnownSession([]string{small, mid, frontier})

	for _, tc := range []struct {
		name    string
		current string
		session core.SessionList
	}{
		{"haiku parent, known ladder", small, ladder},
		{"unknown parent, known ladder", "", ladder},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := core.Route("help", ccHarness, tc.current, cat)
			if d.Confident || !d.PolicyFloor || d.Tier != core.TierFrontier {
				t.Fatalf("precondition: confident=%v floor=%v tier=%v", d.Confident, d.PolicyFloor, d.Tier)
			}
			for _, r := range []string{core.RuleUnconfidentUnknownFrontier, core.RuleUnconfidentUpshift, core.RuleUnconfidentUnknownMid} {
				if slices.Contains(d.Corrections, r) {
					t.Fatalf("tier floor held by %s: %v", r, d.Corrections)
				}
			}
			if d.SafeVerdict == core.VerdictOK && d.Verdict != core.VerdictOK {
				t.Fatalf("SafeVerdict held a floored decision: verdict=%s corrections=%v", d.Verdict, d.Corrections)
			}
			plan := d.PlanSession(core.ClaudeCodeCaps, cat, tc.session, tc.current)
			if !plan.RewriteModel || plan.Model.ID != frontier {
				t.Fatalf("floor not written: want %s, got %+v", frontier, plan)
			}
		})
	}
}

// deterministicLift is the single set R7, R8 and R9 exempt; each member must
// lift an unconfident decision out of all three rules.
func TestDeterministicLift_ExemptsEveryRule(t *testing.T) {
	lifts := map[string]func(*core.Decision){
		"risk":  func(d *core.Decision) { d.RiskFloor = true },
		"graph": func(d *core.Decision) { d.GraphEscalated = true },
		"nb":    func(d *core.Decision) { d.NBUpshift.Applied = true },
		"floor": func(d *core.Decision) { d.PolicyFloor = true },
	}
	cases := []struct {
		verdict core.Verdict
		tier    core.Tier
		rule    string
	}{
		{core.VerdictUnknown, core.TierFrontier, core.RuleUnconfidentUnknownFrontier},
		{core.VerdictUpshift, core.TierFrontier, core.RuleUnconfidentUpshift},
		{core.VerdictUnknown, core.TierMid, core.RuleUnconfidentUnknownMid},
	}
	for _, c := range cases {
		base := core.Decision{
			Harness: ccHarness, Verdict: c.verdict, Tier: c.tier,
			Model: core.Model{ID: catID(ccHarness, c.tier), Tier: c.tier, Harness: ccHarness},
		}
		if rep := core.Check(base); !slices.ContainsFunc(rep.Violations, func(v core.Violation) bool { return v.Rule == c.rule }) {
			t.Fatalf("%s: precondition, %s must fire without a lift", c.verdict, c.rule)
		}
		for name, lift := range lifts {
			d := base
			lift(&d)
			if rep := core.Check(d); rep.Corrected {
				t.Errorf("%s/%s with lift %s still held: %v", c.verdict, c.tier, name, rep.Violations)
			}
		}
	}
}
