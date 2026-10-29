// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
)

// explicitOnlyRes marks one model explicit_only on top of the embedded
// catalog, to prove a catalog candidate with that routing is never added.
type explicitOnlyRes struct {
	*catalog.Catalog
	explicit string
}

func (r explicitOnlyRes) IsExplicitOnly(harness, id string) bool {
	if m, ok := r.LookupByID(harness, id); ok && m.ID == r.explicit {
		return true
	}
	return r.Catalog.IsExplicitOnly(harness, id)
}

const riskyCandidateTask = "find where the auth token is stored and how passwords are hashed"

// Claude Code sends no session list. The catalog's session candidates, cut at
// the current model, stand in for it on that harness only.
func TestCatalogSessionCandidates(t *testing.T) {
	small := cat.ModelFor("claude-code", core.TierSmall)
	cases := []struct {
		name      string
		harness   string
		caps      core.HarnessCapabilities
		res       core.Resolver
		session   core.SessionList
		current   string
		task      string
		rewrite   bool
		write     string // expected WriteName when rewrite
		minTier   core.Tier
		fromCat   bool
		unknown   bool
		forbidden string // catalog id the plan must never target
	}{
		{name: "no list, opus, trivial -> smallest family", harness: "claude-code", caps: core.ClaudeCodeCaps, res: cat,
			session: core.UnknownSession(), current: "opus", task: holdTask, rewrite: true, write: small.Native, fromCat: true},
		{name: "no list, opus, mid decision -> mid family, never the smallest", harness: "claude-code", caps: core.ClaudeCodeCaps, res: cat,
			session: core.UnknownSession(), current: "opus", task: riskyCandidateTask, rewrite: true, write: "sonnet", minTier: core.TierMid, fromCat: true, forbidden: small.ID},
		{name: "unknown current model -> unknown session, hold", harness: "claude-code", caps: core.ClaudeCodeCaps, res: cat,
			session: core.UnknownSession(), current: "no-such-model", task: holdTask, unknown: true},
		{name: "empty current model -> unknown session, hold", harness: "claude-code", caps: core.ClaudeCodeCaps, res: cat,
			session: core.UnknownSession(), current: "", task: holdTask, unknown: true},
		{name: "payload list wins over the catalog candidates", harness: "claude-code", caps: core.ClaudeCodeCaps, res: cat,
			session: core.KnownSession([]string{"claude-sonnet-5-5", "claude-opus-5-5"}), current: "claude-opus-5-5", task: holdTask,
			rewrite: true, write: "sonnet", forbidden: small.ID},
		{name: "harness without the capability holds", harness: "cursor", caps: core.CursorCaps, res: cat,
			session: core.UnknownSession(), current: cat.ModelFor("cursor", core.TierFrontier).ID, task: holdTask, unknown: true},
		{name: "explicit_only candidate is never chosen", harness: "claude-code", caps: core.ClaudeCodeCaps,
			res: explicitOnlyRes{Catalog: cat, explicit: small.ID}, session: core.UnknownSession(), current: "opus", task: holdTask,
			rewrite: true, write: "sonnet", fromCat: true, forbidden: small.ID},
		{name: "list ends at current: complex task on sonnet never upshifts", harness: "claude-code", caps: core.ClaudeCodeCaps, res: cat,
			session: core.UnknownSession(), current: "sonnet",
			task:    "Rearchitect the payment flow across services, migrate the auth tokens and redesign the webhook idempotency guarantees for the distributed system",
			fromCat: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			decidertest.Use(t, decidertest.Tier(candidateTier(c.task)))
			d := core.Route(c.task, c.harness, c.current, c.res)
			d.Baseline.RequestedID = c.current
			plan := d.PlanSession(c.caps, c.res, c.session, c.current)
			if plan.RewriteModel != c.rewrite {
				t.Fatalf("rewrite=%v, want %v: %+v", plan.RewriteModel, c.rewrite, plan)
			}
			if c.rewrite && plan.WriteName != c.write {
				t.Fatalf("wrote %q, want %q", plan.WriteName, c.write)
			}
			if c.rewrite && !core.CanWriteSessionID(c.harness, plan.Model.ID, plan.Session, c.res) {
				t.Fatalf("target %q fails the write guard", plan.Model.ID)
			}
			if plan.RewriteModel && plan.Model.Tier < c.minTier {
				t.Fatalf("target %q tier %s below %s", plan.Model.ID, plan.Model.Tier, c.minTier)
			}
			if c.forbidden != "" && plan.RewriteModel && plan.Model.ID == c.forbidden {
				t.Fatalf("target %q is forbidden here", plan.Model.ID)
			}
			if plan.Session.FromCatalog != c.fromCat || d.Session.Unknown != c.unknown {
				t.Fatalf("fromCatalog=%v unknown=%v, want %v/%v", plan.Session.FromCatalog, d.Session.Unknown, c.fromCat, c.unknown)
			}
			if c.fromCat {
				if last := plan.Session.IDs[len(plan.Session.IDs)-1]; last != c.current {
					t.Fatalf("catalog list must end at the current model, got %v", plan.Session.IDs)
				}
			}
		})
	}
}

// Every embedded candidate resolves to an automatic claude-code model with a
// native name, so StrictModelName never drops a catalog-built target.
func TestEmbeddedSessionCandidatesResolve(t *testing.T) {
	names := cat.SessionCandidates("claude-code")
	if len(names) == 0 {
		t.Fatal("claude-code declares no session_candidates")
	}
	for _, n := range names {
		m, ok := cat.LookupByID("claude-code", n)
		if !ok || m.Native == "" || cat.IsExplicitOnly("claude-code", m.ID) {
			t.Errorf("candidate %q: resolved=%v native=%q explicit_only=%v", n, ok, m.Native, cat.IsExplicitOnly("claude-code", m.ID))
		}
	}
}

// candidateTier is the tier the stub decider picks for each task of the table:
// the planning under test is the same whatever policy chose the tier.
func candidateTier(task string) core.Tier {
	switch task {
	case holdTask:
		return core.TierSmall
	case riskyCandidateTask:
		return core.TierMid
	}
	return core.TierFrontier
}
