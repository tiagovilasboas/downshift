// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// coverage_test.go covers low-traffic paths not reached by the main test suite.
package core_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
)

var covCat = catalog.Load()

// --- Verdict.String() ---

func TestVerdictString(t *testing.T) {
	cases := map[core.Verdict]string{
		core.VerdictOK:        "OK",
		core.VerdictDownshift: "DOWNSHIFT",
		core.VerdictUpshift:   "UPSHIFT",
		core.VerdictUnknown:   "UNKNOWN",
		core.Verdict(99):      "UNKNOWN", // default branch
	}
	for v, want := range cases {
		if got := v.String(); got != want {
			t.Errorf("Verdict(%d).String() = %q, want %q", v, got, want)
		}
	}
}

// --- Effort.String() default branch ---

func TestEffortString_DefaultBranch(t *testing.T) {
	// Effort(99) hits the default case → "medium"
	if got := core.Effort(99).String(); got != "medium" {
		t.Errorf("Effort(99).String() = %q, want medium", got)
	}
}

// --- EscalationIntent.String() default branch ---

func TestEscalationIntentString_DefaultBranch(t *testing.T) {
	if got := core.EscalationIntent(99).String(); got != "unknown" {
		t.Errorf("EscalationIntent(99).String() = %q, want unknown", got)
	}
}

// --- Plan() with PreserveExplicit ---

func TestPlan_PreserveExplicit_SkipsRewrite(t *testing.T) {
	// Route a trivial task on an explicit_only model (gpt-6-astra).
	d := core.Route("rename the variable", "codex", "gpt-6-astra", covCat)
	plan := d.Plan(core.CodexCaps, covCat)
	if !plan.PreserveExplicit {
		t.Error("Plan must set PreserveExplicit for explicit_only current model")
	}
	if plan.RewriteModel {
		t.Error("Plan must not rewrite model when PreserveExplicit")
	}
	if plan.ApplyEffort {
		t.Error("Plan must not apply effort when PreserveExplicit")
	}
}

func TestPlan_NilResolver_DoesNotPreserve(t *testing.T) {
	// Without a resolver, PreserveExplicit must be false (fail-open).
	d := core.Route("rename the variable", "codex", "gpt-6-astra")
	plan := d.Plan(core.CodexCaps) // no resolver
	if plan.PreserveExplicit {
		t.Error("Plan without resolver must not set PreserveExplicit")
	}
}

// --- Tier.String() and Effort.String() all branches ---

func TestTierString_AllBranches(t *testing.T) {
	cases := map[core.Tier]string{
		core.TierSmall:    "small",
		core.TierMid:      "mid",
		core.TierFrontier: "frontier",
		core.Tier(99):     "unknown",
	}
	for tier, want := range cases {
		if got := tier.String(); got != want {
			t.Errorf("Tier(%d).String() = %q, want %q", tier, got, want)
		}
	}
}

func TestEffortString_AllBranches(t *testing.T) {
	cases := map[core.Effort]string{
		core.EffortLow:  "low",
		core.EffortMid:  "medium",
		core.EffortHigh: "high",
		core.Effort(99): "medium", // default branch
	}
	for effort, want := range cases {
		if got := effort.String(); got != want {
			t.Errorf("Effort(%d).String() = %q, want %q", effort, got, want)
		}
	}
}

// --- ShouldPreserveExplicitModel edge: r != nil, empty model ID ---

func TestShouldPreserveExplicitModel_EmptyModelID_NilResolver(t *testing.T) {
	d := core.Route("rename", "codex", "", covCat)
	// r is provided but currentModelID is empty → must return false
	if d.ShouldPreserveExplicitModel("", covCat) {
		t.Error("empty model ID must not trigger preservation even with a resolver")
	}
}

func TestShouldPreserveExplicitModel_NonExplicitModel(t *testing.T) {
	frontierID := covCat.ModelFor("codex", core.TierFrontier).ID
	d := core.Route("rename", "codex", frontierID, covCat)
	if d.ShouldPreserveExplicitModel(frontierID, covCat) {
		t.Errorf("normal frontier model %q must not be preserved", frontierID)
	}
}

func TestShouldPreserveExplicitModel_ExplicitOnly(t *testing.T) {
	d := core.Route("rename", "codex", "gpt-6-astra", covCat)
	if !d.ShouldPreserveExplicitModel("gpt-6-astra", covCat) {
		t.Error("explicit_only model must be preserved")
	}
}

func TestShouldPreserveExplicitModel_IgnoresSemanticLift(t *testing.T) {
	d := core.Decision{Harness: "codex", Guards: core.Guards{SemanticLift: true, RiskAbstain: true}}
	if !d.ShouldPreserveExplicitModel("gpt-6-astra", covCat) {
		t.Error("explicit_only must still be preserved after a semantic lift")
	}
}

// A Decision with no recommended model must never trigger a rewrite.
func TestCheckWrites_EmptyModelID(t *testing.T) {
	d := core.Decision{}
	if checkWrites(d) {
		t.Error("empty model ID must not be written")
	}
}

// --- resolveModel with no resolver (nil path) ---

// --- Summary: PreservedIntent suffix ---

func TestDecisionSummary_PreservedIntentSuffix(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	d := core.Route("rename the variable", "codex", "gpt-6-astra", covCat)
	s := d.Summary()
	if !strings.Contains(s, "preserved") {
		t.Errorf("Summary for PreservedIntent should contain 'preserved'; got %q", s)
	}
}

// --- isReviewTask via Route for each review signal ---

// --- contains edge cases ---
