// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

// TestPlan_FallbackToHarnessSmallest proves the shared safety net: a blocked
// downgrade or an unknown target id lands on that harness's smallest catalog
// model, and never on another harness's slug.
func TestPlan_FallbackToHarnessSmallest(t *testing.T) {
	harnesses := []struct {
		name string
		caps core.HarnessCapabilities
	}{
		{name: "cursor", caps: core.CursorCaps},
		{name: "claude-code", caps: core.ClaudeCodeCaps},
		{name: "codex", caps: core.CodexCaps},
	}

	for _, h := range harnesses {
		t.Run(h.name+"/blocked-downshift", func(t *testing.T) {
			current := cat.ModelFor(h.name, core.TierFrontier)
			small := cat.ModelFor(h.name, core.TierSmall)
			if small.ID == "" || small.ID == current.ID {
				t.Fatalf("catalog fixture: small=%q current=%q", small.ID, current.ID)
			}
			d := core.Decision{
				Harness:      h.name,
				Verdict:      core.VerdictDownshift,
				Confident:    false,
				Tier:         core.TierSmall,
				Model:        core.Model{ID: "not-a-catalog-id", Tier: core.TierSmall, Harness: h.name},
				CurrentModel: current,
			}
			if d.ShouldRewriteModel() {
				t.Fatal("precondition: classified downgrade must stay blocked")
			}
			plan := d.Plan(h.caps, cat)
			assertSmallest(t, h.name, plan, small.ID)
		})

		t.Run(h.name+"/empty-target", func(t *testing.T) {
			current := cat.ModelFor(h.name, core.TierFrontier)
			small := cat.ModelFor(h.name, core.TierSmall)
			d := core.Decision{
				Harness:      h.name,
				Verdict:      core.VerdictUnknown,
				Confident:    false,
				Tier:         core.TierSmall,
				Model:        core.Model{Tier: core.TierSmall, Harness: h.name},
				CurrentModel: current,
			}
			plan := d.Plan(h.caps, cat)
			assertSmallest(t, h.name, plan, small.ID)
		})
	}
}

// TestPlan_CursorRejectsForeignAlias ensures Cursor emits its catalog small
// id (claude-4.5-haiku-thinking), not the claude-code slug claude-haiku-4,
// which is only an alias on the cursor row.
func TestPlan_CursorRejectsForeignAlias(t *testing.T) {
	current := cat.ModelFor("cursor", core.TierFrontier)
	d := core.Decision{
		Harness:      "cursor",
		Verdict:      core.VerdictDownshift,
		Confident:    true,
		Tier:         core.TierSmall,
		Model:        core.Model{ID: "claude-haiku-4", Tier: core.TierSmall, Harness: "cursor"},
		CurrentModel: current,
	}
	plan := d.Plan(core.CursorCaps, cat)
	if plan.Model.ID != "claude-4.5-haiku-thinking" {
		t.Fatalf("model = %q, want claude-4.5-haiku-thinking", plan.Model.ID)
	}
	if plan.Model.ID == "claude-haiku-4" {
		t.Fatal("cursor must not emit the claude-code catalog id")
	}
	if !plan.RewriteModel {
		t.Fatal("foreign target must still rewrite to the cursor catalog id")
	}
}

// TestPlan_ExplicitOnlySkipsSmallestFallback keeps a deliberate model even
// when the classified downgrade is blocked.
func TestPlan_ExplicitOnlySkipsSmallestFallback(t *testing.T) {
	d := core.Decision{
		Harness:      "codex",
		Verdict:      core.VerdictDownshift,
		Confident:    false,
		Tier:         core.TierSmall,
		Model:        core.Model{ID: "", Tier: core.TierSmall, Harness: "codex"},
		CurrentModel: core.Model{ID: "gpt-6-astra", Tier: core.TierFrontier, Harness: "codex"},
	}
	plan := d.Plan(core.CodexCaps, cat)
	if !plan.PreserveExplicit {
		t.Fatal("explicit_only current model must be preserved")
	}
	if plan.RewriteModel {
		t.Fatal("explicit_only model must not fall back to the smallest catalog id")
	}
}

func assertSmallest(t *testing.T, harness string, plan core.RewritePlan, wantID string) {
	t.Helper()
	if plan.PreserveExplicit {
		t.Fatalf("%s: fallback must not set PreserveExplicit", harness)
	}
	if !plan.RewriteModel {
		t.Fatalf("%s: fallback must rewrite when the smallest id differs from current", harness)
	}
	if plan.Model.ID != wantID {
		t.Fatalf("%s: model = %q, want %q", harness, plan.Model.ID, wantID)
	}
	if harness == "cursor" && plan.Model.ID == "claude-haiku-4" {
		t.Fatal("cursor fell back to another harness id")
	}
	looked, ok := cat.LookupByID(harness, plan.Model.ID)
	if !ok || looked.ID != plan.Model.ID {
		t.Fatalf("%s: emitted %q is not a catalog id for this harness", harness, plan.Model.ID)
	}
}
