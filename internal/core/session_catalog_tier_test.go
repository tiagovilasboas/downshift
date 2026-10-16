// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

const (
	tHaiku  = "claude-haiku-4-5"  // catalog: small (native 'haiku' alias)
	tHaiku5 = "claude-haiku-5-5"  // catalog: small, but no native name: never written
	tSonnet = "claude-sonnet-5-5" // catalog: mid
	tOpus   = "claude-opus-5-5"   // catalog: frontier
	tFable  = "claude-fable-5-1"  // catalog: frontier
)

// A confident MEDIUM task on an opus parent must land on the mid-tier model
// whenever the catalog says one is mid, wherever it sits in the list.
func TestPlanForSession_MidTaskPicksCatalogMidNotListPosition(t *testing.T) {
	d := core.Route("implement the feature flag integration in the billing module", "claude-code", tOpus, cat)
	if d.Tier != core.TierMid {
		t.Fatalf("precondition: tier=%s complexity=%s", d.Tier, d.Complexity)
	}
	cases := []struct {
		name    string
		session []string
		want    string
	}{
		{"four models with fable", []string{tHaiku, tSonnet, tOpus, tFable}, tSonnet},
		{"two models haiku+opus", []string{tHaiku, tOpus}, tOpus},
		{"two models sonnet+opus", []string{tSonnet, tOpus}, tSonnet},
		{"sonnet only", []string{tSonnet}, tSonnet},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := d.PlanForSession(core.ClaudeCodeCaps, cat, core.KnownSession(tc.session))
			if plan.Model.ID != tc.want {
				t.Fatalf("got %q, want %q", plan.Model.ID, tc.want)
			}
		})
	}
}

// selector drives the downshift path with a hand-built decision so every tier
// is reachable without depending on the classifier.
func selectorPlan(harness string, caps core.HarnessCapabilities, tier core.Tier, s core.SessionList) core.RewritePlan {
	d := core.Decision{Harness: harness, Tier: tier, Verdict: core.VerdictDownshift}
	return d.PlanForSession(caps, cat, s)
}

func TestSessionSelect_ByCatalogTier(t *testing.T) {
	cases := []struct {
		name    string
		tier    core.Tier
		session []string
		want    string // "" means no rewrite
	}{
		{"small, four models", core.TierSmall, []string{tHaiku, tSonnet, tOpus, tFable}, tHaiku},
		{"small, opus+haiku order: first at or above small", core.TierSmall, []string{tOpus, tHaiku}, tOpus},
		{"mid, four models", core.TierMid, []string{tHaiku, tSonnet, tOpus, tFable}, tSonnet},
		{"mid, haiku+opus", core.TierMid, []string{tHaiku, tOpus}, tOpus},
		{"mid, sonnet+opus", core.TierMid, []string{tSonnet, tOpus}, tSonnet},
		{"mid, sonnet only", core.TierMid, []string{tSonnet}, tSonnet},
		{"frontier, four models", core.TierFrontier, []string{tHaiku, tSonnet, tOpus, tFable}, tOpus},
		{"frontier, sonnet only: hold, never below the floor", core.TierFrontier, []string{tHaiku, tSonnet}, ""},
		{"mid, haiku only: hold, never below the floor", core.TierMid, []string{tHaiku}, ""},
		{"small, only haiku 5.5 listed: hold, the native haiku alias is 4.5 and may not be listed", core.TierSmall, []string{tHaiku5, tSonnet}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := selectorPlan("claude-code", core.ClaudeCodeCaps, tc.tier, core.KnownSession(tc.session))
			if tc.want == "" {
				if plan.RewriteModel {
					t.Fatalf("rewrite to %q, want hold", plan.Model.ID)
				}
				return
			}
			if !plan.RewriteModel || plan.Model.ID != tc.want {
				t.Fatalf("got %q (rewrite %v), want %q", plan.Model.ID, plan.RewriteModel, tc.want)
			}
		})
	}
}

// Ids the catalog does not know cannot be ranked by tier: the index mapping
// (first / midpoint / last) stays as the fallback. One unknown id is enough.
func TestSessionSelect_UnknownIDsFallBackToIndexMapping(t *testing.T) {
	cases := []struct {
		name    string
		tier    core.Tier
		session []string
		want    string
	}{
		{"all unknown, small", core.TierSmall, []string{"op-a", "op-b", "op-c"}, "op-a"},
		{"all unknown, mid", core.TierMid, []string{"op-a", "op-b", "op-c"}, "op-b"},
		{"all unknown, frontier", core.TierFrontier, []string{"op-a", "op-b", "op-c"}, "op-c"},
		{"mixed known and unknown, mid", core.TierMid, []string{tHaiku, "op-b", tOpus}, "op-b"},
		{"mixed, frontier", core.TierFrontier, []string{tSonnet, "op-b"}, "op-b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Strict-name harnesses refuse to write ids without a native name, so
			// assert the selection (plan.Model), not the wire rewrite.
			plan := selectorPlan("claude-code", core.ClaudeCodeCaps, tc.tier, core.KnownSession(tc.session))
			if plan.Model.ID != tc.want {
				t.Fatalf("got %q, want %q", plan.Model.ID, tc.want)
			}
		})
	}
}

// An exhausted id never lowers the floor: a mid task with the only mid-or-above
// ids exhausted must not fall to the small model, and an exhausted mid id lets
// the next id at or above mid win.
func TestSessionSelect_ExhaustedDoesNotLowerFloor(t *testing.T) {
	t.Run("exhausted mid skips to next at or above", func(t *testing.T) {
		s := core.KnownSession([]string{tHaiku, tSonnet, tOpus})
		s.Exhausted = []string{tSonnet}
		plan := selectorPlan("claude-code", core.ClaudeCodeCaps, core.TierMid, s)
		if !plan.RewriteModel || plan.Model.ID != tOpus {
			t.Fatalf("got %q (rewrite %v), want %q", plan.Model.ID, plan.RewriteModel, tOpus)
		}
	})
	t.Run("everything at or above exhausted holds", func(t *testing.T) {
		s := core.KnownSession([]string{tHaiku, tSonnet, tOpus})
		s.Exhausted = []string{tSonnet, tOpus}
		plan := selectorPlan("claude-code", core.ClaudeCodeCaps, core.TierMid, s)
		if plan.RewriteModel || plan.Model.ID == tHaiku {
			t.Fatalf("got %q (rewrite %v), want hold", plan.Model.ID, plan.RewriteModel)
		}
	})
	t.Run("exhausted frontier does not make small the floor", func(t *testing.T) {
		s := core.KnownSession([]string{tHaiku, tSonnet, tOpus})
		s.Exhausted = []string{tOpus}
		plan := selectorPlan("claude-code", core.ClaudeCodeCaps, core.TierFrontier, s)
		if plan.RewriteModel {
			t.Fatalf("rewrite to %q, want hold", plan.Model.ID)
		}
	})
}

// explicit_only ids are skipped by ordinary tier selection (cursor catalog).
func TestSessionSelect_ExplicitOnlySkipped(t *testing.T) {
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "")
	t.Setenv("DOWNSHIFT_SESSION_MODELS", "")
	const explicit = "claude-fable-5-1-thinking-high"
	if !cat.IsExplicitOnly("cursor", explicit) {
		t.Fatalf("precondition: %s must be explicit_only on cursor", explicit)
	}
	const sonnet = "claude-sonnet-5-5-high"
	s := core.KnownSession([]string{sonnet, explicit})
	if plan := selectorPlan("cursor", core.CursorCaps, core.TierMid, s); plan.Model.ID != sonnet {
		t.Fatalf("mid: got %q, want %q", plan.Model.ID, sonnet)
	}
	if plan := selectorPlan("cursor", core.CursorCaps, core.TierFrontier, s); plan.RewriteModel {
		t.Fatalf("frontier: explicit_only %q must not be chosen automatically", plan.Model.ID)
	}
}
