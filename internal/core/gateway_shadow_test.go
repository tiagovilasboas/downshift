// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// gatewayRows is a Resolver that also lists shadow gateway rows.
type gatewayRows struct {
	core.Resolver
	rows []core.Model
}

func (g gatewayRows) ShadowGateways() []core.Model { return g.rows }

// useEmptyHome points the state dir at a fresh temp home so the real user
// files cannot leak into the test.
func useEmptyHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func writeStateFileAt(t *testing.T, name func() (string, error), body string) {
	t.Helper()
	p, err := name()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

const gatewayAllowlist = `{"version":1,"harness":"opencode","models":[
  {"id":"deepseek/deepseek-v4.1-flash","tier":"mid","max_effort":"medium","zdr":true},
  {"id":"x-ai/grok-x","tier":"small","max_effort":"low"},
  {"id":"x-ai/grok-x-large","tier":"frontier","max_effort":"high"}]}`

// An allowlisted row is shadow only: the live route, the rewrite plan and the
// model written to the spawn are the same with and without the allowlist.
func TestGatewayShadow_AllowlistNeverChangesRoute(t *testing.T) {
	useEmptyHome(t)
	bare := catalog.Load()

	writeStateFileAt(t, paths.OpenRouterAllowlistPath, gatewayAllowlist)
	withList := catalog.Load()

	prompts := []string{
		"fix a typo in the README",
		"rename a variable in one file",
		"refactor the parser module across several packages and update its tests",
		"design a migration plan for the billing ledger",
	}
	currents := []string{"", bare.ModelFor("opencode", core.TierFrontier).ID}

	for _, prompt := range prompts {
		for _, current := range currents {
			want := core.Route(prompt, "opencode", current, bare)
			got := core.Route(prompt, "opencode", current, withList)
			if got.Model.ID != want.Model.ID || got.Tier != want.Tier || got.Verdict != want.Verdict ||
				got.Effort != want.Effort || got.Savings != want.Savings {
				t.Errorf("Route(%q, current %q) changed with the allowlist: got %s/%v/%v, want %s/%v/%v",
					prompt, current, got.Model.ID, got.Tier, got.Verdict, want.Model.ID, want.Tier, want.Verdict)
			}

			wantPlan := want.PlanSession(core.OpenCodeCaps, bare, core.UnknownSession(), current)
			gotPlan := got.PlanSession(core.OpenCodeCaps, withList, core.UnknownSession(), current)
			if gotPlan.Model.ID != wantPlan.Model.ID || gotPlan.RewriteModel != wantPlan.RewriteModel {
				t.Errorf("plan(%q, current %q) changed with the allowlist: got %s rewrite=%v, want %s rewrite=%v",
					prompt, current, gotPlan.Model.ID, gotPlan.RewriteModel, wantPlan.Model.ID, wantPlan.RewriteModel)
			}
			if gotPlan.Model.ID == "deepseek/deepseek-v4.1-flash" || gotPlan.Model.ID == "x-ai/grok-x" {
				t.Errorf("plan wrote an allowlisted row %q as the spawn model", gotPlan.Model.ID)
			}
		}
	}
}

// The would_route pick is the smallest allowlisted row of the decision tier.
func TestGatewayShadow_PicksSmallestRowOfTheTier(t *testing.T) {
	cases := []struct {
		name string
		tier core.Tier
		rows []core.Model
		want string
	}{
		{"priced row beats unpriced", core.TierMid, []core.Model{
			{ID: "a/unpriced", Tier: core.TierMid},
			{ID: "z/priced", Tier: core.TierMid, InputM: 0.1, OutputM: 0.2},
		}, "z/priced"},
		{"lower price wins", core.TierMid, []core.Model{
			{ID: "a/dear", Tier: core.TierMid, InputM: 1, OutputM: 2},
			{ID: "b/cheap", Tier: core.TierMid, InputM: 0.1, OutputM: 0.2},
		}, "b/cheap"},
		{"unpriced ties break on id", core.TierSmall, []core.Model{
			{ID: "y/two", Tier: core.TierSmall},
			{ID: "x/one", Tier: core.TierSmall},
		}, "x/one"},
		{"other tiers are ignored", core.TierFrontier, []core.Model{
			{ID: "a/small", Tier: core.TierSmall, InputM: 0.01},
		}, ""},
		{"no rows", core.TierMid, nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := core.Decision{Tier: tc.tier, Harness: "opencode"}
			d.RecordGatewayWould(gatewayRows{Resolver: catalog.Load(), rows: tc.rows})
			if d.Shadow.GatewayWould != tc.want {
				t.Errorf("GatewayWould = %q, want %q", d.Shadow.GatewayWould, tc.want)
			}
		})
	}
}

// Without an allowlist, or with a resolver that lists no gateway rows, the
// decision records nothing.
func TestGatewayShadow_NoAllowlistRecordsNothing(t *testing.T) {
	useEmptyHome(t)
	res := catalog.Load()
	d := core.Decision{Tier: core.TierSmall, Harness: "opencode"}
	d.PlanSession(core.OpenCodeCaps, res, core.UnknownSession(), "")
	if d.Shadow.GatewayWould != "" {
		t.Fatalf("GatewayWould = %q without an allowlist", d.Shadow.GatewayWould)
	}
}

// explicit_only rows, from a user catalog, are never recorded as would_route.
func TestGatewayShadow_ExplicitOnlyRowIsNeverRecorded(t *testing.T) {
	useEmptyHome(t)
	writeStateFileAt(t, paths.CatalogPath, `{"entries":[
		{"id":"z-ai/glm","harness":"opencode","provider":"openrouter","tier":"small"}]}`)
	res := catalog.Load()
	d := core.Decision{Tier: core.TierSmall, Harness: "opencode"}
	d.RecordGatewayWould(res)
	if d.Shadow.GatewayWould != "" {
		t.Fatalf("explicit_only row recorded as %q", d.Shadow.GatewayWould)
	}
}

// A bad allowlist is ignored: no row is recorded and the route is unchanged.
func TestGatewayShadow_BadAllowlistIsIgnored(t *testing.T) {
	bad := map[string]string{
		"invalid JSON":  `{"version":1,`,
		"wrong harness": `{"version":1,"harness":"claude-code","models":[{"id":"x/y","tier":"small","max_effort":"low"}]}`,
		"unknown tier":  `{"version":1,"harness":"opencode","models":[{"id":"x/y","tier":"huge","max_effort":"low"}]}`,
		"bare model id": `{"version":1,"harness":"opencode","models":[{"id":"grok","tier":"small","max_effort":"low"}]}`,
	}
	for name, body := range bad {
		t.Run(name, func(t *testing.T) {
			useEmptyHome(t)
			bare := catalog.Load()
			writeStateFileAt(t, paths.OpenRouterAllowlistPath, body)
			res := catalog.Load()

			d := core.Decision{Tier: core.TierSmall, Harness: "opencode"}
			d.RecordGatewayWould(res)
			if d.Shadow.GatewayWould != "" {
				t.Errorf("bad allowlist recorded %q", d.Shadow.GatewayWould)
			}
			want := core.Route("fix a typo", "opencode", "", bare)
			got := core.Route("fix a typo", "opencode", "", res)
			if got.Model.ID != want.Model.ID {
				t.Errorf("route changed with a bad allowlist: %s, want %s", got.Model.ID, want.Model.ID)
			}
		})
	}
}

// The production path wraps the catalog in a session resolver whenever the
// session is known (every hook payload that names its session). The shadow
// record must survive that wrapper, and the route must stay the same with
// the allowlist present.
func TestGatewayShadow_RecordsThroughKnownSessionResolver(t *testing.T) {
	useEmptyHome(t)
	bare := catalog.Load()
	writeStateFileAt(t, paths.OpenRouterAllowlistPath, gatewayAllowlist)
	withList := catalog.Load()

	session := core.KnownSession([]string{"claude-sonnet-5"})
	prompt := "fix a typo in the README"

	want := core.Route(prompt, "opencode", "", bare)
	if want.Tier != core.TierSmall {
		t.Fatalf("prompt tier = %v, want small", want.Tier)
	}
	wantPlan := want.PlanSession(core.OpenCodeCaps, core.ResolveSessionModels("opencode", session, bare), session, "")

	got := core.Route(prompt, "opencode", "", core.ResolveSessionModels("opencode", session, withList))
	gotPlan := got.PlanSession(core.OpenCodeCaps, core.ResolveSessionModels("opencode", session, withList), session, "")

	if got.Shadow.GatewayWould != "x-ai/grok-x" {
		t.Errorf("GatewayWould = %q through the session resolver, want x-ai/grok-x", got.Shadow.GatewayWould)
	}
	if got.Model.ID != want.Model.ID || got.Tier != want.Tier || got.Verdict != want.Verdict {
		t.Errorf("route changed with the allowlist: got %s/%v/%v, want %s/%v/%v",
			got.Model.ID, got.Tier, got.Verdict, want.Model.ID, want.Tier, want.Verdict)
	}
	if gotPlan.Model.ID != wantPlan.Model.ID || gotPlan.RewriteModel != wantPlan.RewriteModel {
		t.Errorf("plan changed with the allowlist: got %s rewrite=%v, want %s rewrite=%v",
			gotPlan.Model.ID, gotPlan.RewriteModel, wantPlan.Model.ID, wantPlan.RewriteModel)
	}
}
