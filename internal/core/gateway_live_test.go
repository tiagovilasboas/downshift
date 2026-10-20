// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

const liveSlug = "deepseek/deepseek-v4.1-flash-1017"

// liveAllowlist writes an openrouter.json whose one row carries the given
// tier and route, in a fresh state dir.
func liveAllowlist(t *testing.T, tier, route string) {
	t.Helper()
	useEmptyHome(t)
	body := `{"version":1,"harness":"opencode","models":[{"id":"` + liveSlug + `","tier":"` + tier + `","max_effort":"medium","route":"` + route + `"}]}`
	writeStateFileAt(t, paths.GatewayAllowlistPath, body)
}

// A spawn that the plan moves onto a live row of its tier lands on the exact
// gateway id; the write guard accepts that id and only that id.
func TestGatewayLive_RewriteTargetsLiveRow(t *testing.T) {
	liveAllowlist(t, "small", "live")
	res := catalog.Load()
	small := res.ModelFor("opencode", core.TierSmall).ID
	frontier := res.ModelFor("opencode", core.TierFrontier).ID
	session := core.KnownSession([]string{small, frontier})

	d := core.Route("fix a typo in the README", "opencode", frontier, res)
	if d.Tier != core.TierSmall {
		t.Fatalf("decision tier = %v, want small", d.Tier)
	}
	plan := d.PlanSession(core.OpenCodeCaps, res, session, frontier)
	if !plan.RewriteModel {
		t.Fatalf("plan does not rewrite: %+v", plan)
	}
	if plan.Model.ID != liveSlug || plan.Model.Gateway != "openrouter" || plan.Model.Harness != "opencode" {
		t.Fatalf("plan.Model = %+v, want the live row openrouter/%s", plan.Model, liveSlug)
	}
	if plan.WriteName != liveSlug {
		t.Errorf("WriteName = %q, want %q", plan.WriteName, liveSlug)
	}
	if !core.CanWriteSessionID("opencode", plan.Model.ID, plan.Session, res) {
		t.Errorf("write guard refused the live row the planner chose")
	}
	if core.CanWriteSessionID("opencode", "deepseek/some-other-row", plan.Session, res) {
		t.Errorf("write guard accepted an id that is not a live row")
	}
}

// A shadow row is observation only: the plan keeps the native target.
func TestGatewayLive_ShadowRowNeverTargets(t *testing.T) {
	liveAllowlist(t, "small", "shadow")
	res := catalog.Load()
	small := res.ModelFor("opencode", core.TierSmall).ID
	frontier := res.ModelFor("opencode", core.TierFrontier).ID
	session := core.KnownSession([]string{small, frontier})

	d := core.Route("fix a typo in the README", "opencode", frontier, res)
	plan := d.PlanSession(core.OpenCodeCaps, res, session, frontier)
	if plan.Model.Gateway != "" || plan.Model.ID != small {
		t.Fatalf("plan.Model = %+v, want the native %s", plan.Model, small)
	}
}

// A live row names one harness: a Claude Code decision never takes it.
func TestGatewayLive_OtherHarnessNeverTargets(t *testing.T) {
	liveAllowlist(t, "small", "live")
	res := catalog.Load()
	small := res.ModelFor("claude-code", core.TierSmall).ID
	frontier := res.ModelFor("claude-code", core.TierFrontier).ID
	session := core.KnownSession([]string{small, frontier})

	d := core.Route("fix a typo in the README", "claude-code", frontier, res)
	plan := d.PlanSession(core.ClaudeCodeCaps, res, session, frontier)
	if plan.Model.Gateway != "" || plan.Model.ID == liveSlug {
		t.Fatalf("claude-code plan took the opencode live row: %+v", plan.Model)
	}
}

// A live row never adds a rewrite: a spawn already on its target stays put.
func TestGatewayLive_NoRewriteStaysPut(t *testing.T) {
	liveAllowlist(t, "small", "live")
	res := catalog.Load()
	small := res.ModelFor("opencode", core.TierSmall).ID
	frontier := res.ModelFor("opencode", core.TierFrontier).ID
	session := core.KnownSession([]string{small, frontier})

	d := core.Route("fix a typo in the README", "opencode", small, res)
	plan := d.PlanSession(core.OpenCodeCaps, res, session, small)
	if plan.RewriteModel || plan.Model.ID == liveSlug {
		t.Fatalf("plan rewrote a spawn already on its target: %+v", plan)
	}
}

// Without a known session nothing is rewritten, live rows included.
func TestGatewayLive_UnknownSessionNeverTargets(t *testing.T) {
	liveAllowlist(t, "small", "live")
	res := catalog.Load()
	frontier := res.ModelFor("opencode", core.TierFrontier).ID

	d := core.Route("fix a typo in the README", "opencode", frontier, res)
	plan := d.PlanSession(core.OpenCodeCaps, res, core.UnknownSession(), frontier)
	if plan.RewriteModel || plan.Model.ID == liveSlug {
		t.Fatalf("unknown session rewrote the spawn: %+v", plan)
	}
}
