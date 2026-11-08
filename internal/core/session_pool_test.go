// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

// poolGatedID is the cursor catalog entry flagged pool_gated; the slug a
// session lists for it is one of its aliases.
func poolGatedID(t *testing.T) (id, alias string) {
	t.Helper()
	m, ok := cat.LookupByID("cursor", "muse-spark")
	if !ok || !m.PoolGated {
		t.Fatalf("catalog entry for the cursor pool-gated model: %+v ok=%v", m, ok)
	}
	return m.ID, "muse-spark-1.3-high"
}

func planPoolGated(t *testing.T, included, unavailable *[]string) (core.RewritePlan, core.Decision) {
	t.Helper()
	_, alias := poolGatedID(t)
	session := core.KnownSession([]string{"composer-2.5-fast", "composer-2.5", alias}).
		WithHookQuota(included, unavailable)
	d := core.Decision{
		Harness: "cursor", Verdict: core.VerdictUpshift, Confident: true, Tier: core.TierFrontier,
		Model:    core.Model{ID: "composer-2.5", Harness: "cursor"},
		Baseline: core.Baseline{RequestedID: "composer-2.5", CurrentModel: core.Model{ID: "composer-2.5", Harness: "cursor"}},
	}
	plan := d.PlanSession(core.CursorCaps, cat, session, "composer-2.5")
	return plan, d
}

func TestPlanSession_PoolGatedModelNeedsPositiveEvidence(t *testing.T) {
	id, alias := poolGatedID(t)
	t.Run("pool included by catalog id writes the model", func(t *testing.T) {
		plan, d := planPoolGated(t, &[]string{id}, nil)
		// The plan names the slug the session listed; SpawnWriteName turns it
		// into the id the harness's spawn tool accepts.
		got, ok := core.SpawnWriteName("cursor", plan.WriteName, cat)
		if !plan.RewriteModel || !ok || got != id || d.Session.CreditHeld {
			t.Fatalf("plan = %+v write=%q creditHeld=%v, want rewrite to %s", plan, got, d.Session.CreditHeld, id)
		}
	})
	t.Run("unavailable holds with credit evidence", func(t *testing.T) {
		plan, d := planPoolGated(t, &[]string{id}, &[]string{alias})
		if plan.RewriteModel || !d.Session.CreditHeld {
			t.Fatalf("plan = %+v creditHeld=%v, want held by credit", plan, d.Session.CreditHeld)
		}
	})
	t.Run("unobserved pool holds without credit evidence", func(t *testing.T) {
		plan, d := planPoolGated(t, nil, nil)
		if plan.RewriteModel || d.Session.CreditHeld {
			t.Fatalf("plan = %+v creditHeld=%v, want held, not credit", plan, d.Session.CreditHeld)
		}
		if d.Session.QuotaStatus == "available" {
			t.Fatalf("quota status = %q, want not available", d.Session.QuotaStatus)
		}
	})
}

func TestSpawnWriteName_ReplacesTheRejectedName(t *testing.T) {
	id, alias := poolGatedID(t)
	if got, ok := core.SpawnWriteName("cursor", alias, cat); !ok || got != id {
		t.Fatalf("rejected name: got %q ok=%v, want %q", got, ok, id)
	}
	if got, ok := core.SpawnWriteName("cursor", id, cat); !ok || got != id {
		t.Fatalf("canonical id: got %q ok=%v", got, ok)
	}
	if got, ok := core.SpawnWriteName("cursor", "composer-2.5", cat); !ok || got != "composer-2.5" {
		t.Fatalf("unrelated id: got %q ok=%v", got, ok)
	}
	if _, ok := core.SpawnWriteName("cursor", "", cat); ok {
		t.Fatal("empty name must be held")
	}
}

// A foreign or pinned baseline holds before the pool gate runs, so the gate
// must not add credit evidence of its own.
func TestPlanSession_PoolGateSkippedWhenAlreadyHeld(t *testing.T) {
	id, alias := poolGatedID(t)
	for name, mutate := range map[string]func(*core.Decision){
		"foreign":  func(d *core.Decision) { d.Baseline.RequestedID = "some-foreign-model-9" },
		"explicit": func(d *core.Decision) { d.Baseline.Pinned = true },
	} {
		session := core.KnownSession([]string{"composer-2.5-fast", "composer-2.5", alias}).
			WithHookQuota(&[]string{id}, &[]string{alias})
		d := core.Decision{
			Harness: "cursor", Verdict: core.VerdictUpshift, Confident: true, Tier: core.TierFrontier,
			Model:    core.Model{ID: "composer-2.5", Harness: "cursor"},
			Baseline: core.Baseline{RequestedID: "composer-2.5", CurrentModel: core.Model{ID: "composer-2.5", Harness: "cursor"}},
		}
		mutate(&d)
		plan := d.PlanSession(core.CursorCaps, cat, session, "composer-2.5")
		if !(plan.HoldForeign || plan.PreserveExplicit) || d.Session.CreditHeld {
			t.Errorf("%s: plan = %+v creditHeld=%v, want an earlier hold and no gate credit", name, plan, d.Session.CreditHeld)
		}
	}
}

func TestSpawnWriteName_NilResolverWritesTheNameUnchanged(t *testing.T) {
	_, alias := poolGatedID(t)
	if got, ok := core.SpawnWriteName("cursor", alias, nil); !ok || got != alias {
		t.Fatalf("got %q ok=%v, want the name unchanged", got, ok)
	}
}
