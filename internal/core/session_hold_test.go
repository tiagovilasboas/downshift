// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

const holdTask = "rename the userId variable to userIdentifier"

// With no session list (no listing, no payload, no operator file), no harness
// rewrites unless it declares SessionFromCatalog: the catalog tier ladder is
// never a candidate source. The catalog candidates path has its own table
// (TestCatalogSessionCandidates).
func TestNoListingNoPayloadHoldsForEveryHarness(t *testing.T) {
	for _, n := range core.AllHarnessCaps() {
		t.Run(n.Harness, func(t *testing.T) {
			if n.Caps.SessionFromCatalog {
				t.Skip("declares SessionFromCatalog")
			}
			current := cat.ModelFor(n.Harness, core.TierFrontier).ID
			if current == "" {
				t.Skipf("catalog has no frontier model for %s", n.Harness)
			}
			d := core.Route(holdTask, n.Harness, current, cat)
			plan := d.PlanSession(n.Caps, cat, core.UnknownSession(), current)
			if plan.RewriteModel || plan.Model.ID != current {
				t.Fatalf("no session list must hold, got %+v", plan)
			}
			if !d.Session.Unknown {
				t.Fatalf("decision must record the session as unknown")
			}
		})
	}
}

// A rewrite never writes an id outside the session list the harness reported,
// even when the catalog knows other ids for the same tier.
func TestRewriteNeverWritesOutsideSessionList(t *testing.T) {
	session := core.KnownSession([]string{"opaque-small-a1", "opaque-frontier-b2"})
	for _, n := range core.AllHarnessCaps() {
		t.Run(n.Harness, func(t *testing.T) {
			current := "opaque-frontier-b2"
			d := core.Route(holdTask, n.Harness, current, cat)
			plan := d.PlanSession(n.Caps, cat, session, current)
			if !plan.RewriteModel {
				return
			}
			if plan.Model.ID != "opaque-small-a1" && plan.Model.ID != current {
				t.Fatalf("wrote %q, which the session list does not contain", plan.Model.ID)
			}
		})
	}
}

func TestPlanSession_NoCurrentModelStillHolds(t *testing.T) {
	d := core.Route(holdTask, "claude-code", "", cat)
	plan := d.PlanSession(core.ClaudeCodeCaps, cat, core.UnknownSession(), "")
	if plan.RewriteModel || !d.Session.Unknown {
		t.Fatalf("no current model must not rewrite: %+v", plan)
	}
}

// A baseline pinned by the harness's agent definition is the owner's explicit
// choice: whatever the verdict and whether or not the session list is known,
// the plan keeps it (no model and no effort rewrite), like an explicit_only model.
func TestPlanSession_PinnedBaselineIsNeverRewritten(t *testing.T) {
	for _, session := range []core.SessionList{
		core.UnknownSession(),
		core.KnownSession([]string{"claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5"}),
	} {
		d := core.Route("rename the userId variable to userIdentifier", "claude-code", "claude-opus-5-5", cat)
		d.Baseline.Pinned = true
		plan := d.PlanSession(core.ClaudeCodeCaps, cat, session, "claude-opus-5-5")
		if !plan.PreserveExplicit || plan.RewriteModel || plan.ApplyEffort {
			t.Errorf("known=%v: a pinned baseline must be preserved, got %+v", session.Known, plan)
		}
	}
}
