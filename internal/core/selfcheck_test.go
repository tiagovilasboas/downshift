// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

func TestCheck_CleanDecision(t *testing.T) {
	d := core.Decision{
		Complexity:   core.Simple,
		Tier:         core.TierMid,
		Verdict:      core.VerdictDownshift,
		Confident:    true,
		Harness:      "claude-code",
		Model:        core.Model{ID: "m", Harness: "claude-code"},
		CurrentModel: core.Model{ID: "f", Harness: "claude-code"},
	}
	r := core.Check(d)
	if r.Corrected {
		t.Fatalf("clean decision reported corrected: %+v", r.Violations)
	}
	if r.Safe != d.Verdict {
		t.Errorf("Safe = %s, want %s", r.Safe, d.Verdict)
	}
	if len(r.Violations) != 0 {
		t.Errorf("violations = %v, want none", r.Violations)
	}
}

func TestCheck_Rules(t *testing.T) {
	base := core.Decision{
		Complexity:   core.Simple,
		Tier:         core.TierMid,
		Verdict:      core.VerdictDownshift,
		Confident:    true,
		Harness:      "claude-code",
		Model:        core.Model{ID: "m", Harness: "claude-code"},
		CurrentModel: core.Model{ID: "f", Harness: "claude-code"},
	}
	cases := []struct {
		name     string
		mutate   func(*core.Decision)
		wantRule string
	}{
		{"R1 unconfident downgrade", func(d *core.Decision) { d.Confident = false }, core.RuleUnconfidentDowngrade},
		{"R2 complex downgrade", func(d *core.Decision) { d.Complexity = core.Complex }, core.RuleComplexDowngrade},
		{"R4 foreign model", func(d *core.Decision) { d.Model.Harness = "cursor" }, core.RuleForeignModel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := base
			tc.mutate(&d)
			r := core.Check(d)
			if !r.Corrected {
				t.Fatalf("expected correction, got %+v", r)
			}
			if r.Safe != core.VerdictOK {
				t.Errorf("Safe = %s, want OK", r.Safe)
			}
			found := false
			for _, v := range r.Violations {
				if v.Rule == tc.wantRule {
					found = true
				}
			}
			if !found {
				t.Errorf("violations = %v, want rule %s", r.Violations, tc.wantRule)
			}
		})
	}
}

func TestRoute_SelfCheckWired(t *testing.T) {
	d := core.Route("rename the variable", "claude-code", catID("claude-code", core.TierFrontier), cat)
	if !d.Checked {
		t.Fatal("Route decision must carry a Check report")
	}
	if len(d.Corrections) == 0 && d.SafeVerdict != d.Verdict {
		t.Errorf("clean decision: SafeVerdict = %s, want %s", d.SafeVerdict, d.Verdict)
	}
	if len(d.Corrections) > 0 && d.SafeVerdict != core.VerdictOK {
		t.Errorf("held decision: SafeVerdict = %s, want OK", d.SafeVerdict)
	}
}

func TestShouldRewriteModel_HeldDecision(t *testing.T) {
	held := core.Decision{
		Model:       core.Model{ID: "small", Harness: "claude-code"},
		Harness:     "claude-code",
		Verdict:     core.VerdictDownshift,
		SafeVerdict: core.VerdictOK,
		Corrections: []string{core.RuleUnconfidentDowngrade},
		Checked:     true,
		Confident:   true,
	}
	if held.ShouldRewriteModel() {
		t.Error("held decision must not rewrite even when confident")
	}
}
