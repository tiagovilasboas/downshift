// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
)

// cat is the test-scope resolver — same embedded catalog the binary uses.
var cat = catalog.Load()

// catID reads the current model ID from the catalog for a given harness+tier.
func catID(harness string, tier core.Tier) string {
	return cat.ModelFor(harness, tier).ID
}

// --- Route verdict tests ---

func TestRoute_OK(t *testing.T) {
	midID := catID("claude-code", core.TierMid)
	d := core.Route("implement the CSV export feature", "claude-code", midID, cat)
	if d.Verdict != core.VerdictOK {
		t.Fatalf("verdict = %s, want OK", d.Verdict)
	}
	if d.Savings != 0 {
		t.Errorf("savings = %.2f, want 0 for OK", d.Savings)
	}
}

func TestRoute_UnknownCurrentModel(t *testing.T) {
	d := core.Route("rename the variable", "claude-code", "", cat)
	if d.Verdict != core.VerdictUnknown {
		t.Fatalf("verdict = %s, want UNKNOWN", d.Verdict)
	}
	if d.Tier != core.TierSmall {
		t.Errorf("recommended tier = %s, want small", d.Tier)
	}
}

func TestRoute_UnknownHarness(t *testing.T) {
	d := core.Route("rename the variable", "unknown-harness", "", cat)
	if d.Tier != core.TierSmall {
		t.Errorf("recommended tier = %s, want small", d.Tier)
	}
	if d.Verdict != core.VerdictUnknown {
		t.Errorf("verdict = %s, want UNKNOWN", d.Verdict)
	}
}

// --- Effort field tests ---

// --- SavingsRatio contract tests ---

func TestSavingsRatio_CheaperTarget(t *testing.T) {
	frontier := cat.ModelFor("claude-code", core.TierFrontier)
	small := cat.ModelFor("claude-code", core.TierSmall)
	got := cat.SavingsRatio(frontier, small)
	if got <= 0 || got >= 1 {
		t.Errorf("SavingsRatio(frontier→small) = %.4f, want in (0,1)", got)
	}
}

func TestSavingsRatio_MoreExpensiveTarget(t *testing.T) {
	small := cat.ModelFor("claude-code", core.TierSmall)
	frontier := cat.ModelFor("claude-code", core.TierFrontier)
	if r := cat.SavingsRatio(small, frontier); r != 0 {
		t.Errorf("SavingsRatio(small→frontier) = %.4f, want 0", r)
	}
}

// --- Summary contract tests ---

func TestDecisionSummary_Downshift(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	d := core.Route("rename the variable", "claude-code", catID("claude-code", core.TierFrontier), cat)
	s := d.Summary()
	if !strings.Contains(strings.ToLower(s), "downshift") {
		t.Errorf("Summary() = %q, want 'downshift'", s)
	}
}

// Regression: the claude-code small target is the model the native 'haiku'
// alias runs. The note must name it and must not print a percentage: a
// list-price ratio is a cost claim, and savings are reported in tokens only.
func TestDecisionSummary_ClaudeCodeTrivialNamesHaiku55WithoutPercentage(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	for _, current := range []string{"claude-sonnet-5-5", "claude-opus-5-5"} {
		d := core.Route("rename the variable", "claude-code", current, cat)
		if d.Verdict != core.VerdictDownshift {
			t.Fatalf("%s: verdict = %s, want DOWNSHIFT", current, d.Verdict)
		}
		s := d.Summary()
		if want := "downshift to claude-haiku-5-5 (smaller model)"; !strings.Contains(s, want) {
			t.Errorf("%s: Summary() = %q, want it to contain %q", current, s, want)
		}
		if strings.Contains(s, "%") {
			t.Errorf("%s: Summary() = %q, must not print a percentage", current, s)
		}
	}
}

func TestDecisionSummary_Upshift(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierFrontier))
	d := core.Route("rearchitect the payment system", "claude-code", catID("claude-code", core.TierSmall), cat)
	s := d.Summary()
	if !strings.Contains(strings.ToLower(s), "upshift") {
		t.Errorf("Summary() = %q, want 'upshift'", s)
	}
}

func TestDecisionSummary_OK(t *testing.T) {
	d := core.Route("implement the CSV export feature", "claude-code", catID("claude-code", core.TierMid), cat)
	s := d.Summary()
	if !strings.Contains(strings.ToLower(s), "right gear") {
		t.Errorf("Summary() = %q, want 'right gear'", s)
	}
}

func TestDecisionRewritePolicy(t *testing.T) {
	if !checkWrites(core.Decision{Model: core.Model{ID: "target"}, Verdict: core.VerdictUnknown, Confident: true}) {
		t.Error("unknown source model must be routed on a confident classification")
	}
	if checkWrites(core.Decision{Model: core.Model{ID: "target"}, Verdict: core.VerdictUnknown, Tier: core.TierMid}) {
		t.Error("unknown source model must not be routed to mid on an unconfident classification (R9)")
	}
	if !checkWrites(core.Decision{Model: core.Model{ID: "target"}, Verdict: core.VerdictUnknown, Tier: core.TierMid, Guards: core.Guards{RiskFloor: true}}) {
		t.Error("a deterministic lift (risk floor) keeps an unconfident mid writable")
	}
	if checkWrites(core.Decision{Model: core.Model{ID: "target"}, Verdict: core.VerdictUnknown, Tier: core.TierSmall}) {
		t.Error("unknown source model must not be routed to small on an unconfident classification")
	}
	if checkWrites(core.Decision{Model: core.Model{ID: "target"}, Verdict: core.VerdictOK}) {
		t.Error("same-tier model must not be replaced by generic adapters")
	}
	if !(core.Decision{Model: core.Model{ID: "target"}, Verdict: core.VerdictOK}).ShouldApplyEffort() {
		t.Error("same-tier model must receive native effort")
	}
}

func TestDecisionRewritePolicy_UncertainDowngradePreservesCurrentModel(t *testing.T) {
	d := core.Decision{Model: core.Model{ID: "small"}, Verdict: core.VerdictDownshift, Confident: false}
	if checkWrites(d) {
		t.Fatal("uncertain downgrade must not apply the classified target")
	}
}

func TestDecisionRewritePolicy_UncertainUpshiftPreservesCurrentModel(t *testing.T) {
	d := core.Decision{Model: core.Model{ID: "mid"}, Verdict: core.VerdictUpshift, Confident: false}
	if checkWrites(d) {
		t.Fatal("uncertain upshift must not replace the current model with a dearer tier")
	}
}

func TestDecisionRewritePolicy_ConfidentUpshiftCanProceed(t *testing.T) {
	d := core.Decision{Model: core.Model{ID: "mid"}, Verdict: core.VerdictUpshift, Confident: true}
	if !checkWrites(d) {
		t.Fatal("confident upshift should be allowed")
	}
}

func TestDecisionRewritePolicy_ConfidentDowngradeCanProceed(t *testing.T) {
	d := core.Decision{Model: core.Model{ID: "small"}, Verdict: core.VerdictDownshift, Confident: true}
	if !checkWrites(d) {
		t.Fatal("confident downgrade should be allowed")
	}
}
