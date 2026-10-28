// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

const ccHarness contract.HarnessID = "claude-code"

func defaultRow(t *testing.T, agentType string) policy.DefaultProfile {
	t.Helper()
	d, ok := policy.DefaultFor(ccHarness, agentType)
	if !ok {
		t.Fatalf("no default row for %q", agentType)
	}
	return d
}

// TestEconomyTier holds the economy step of each row to its bounds: one tier
// down, never below small (mid on risk), never above the requested tier.
func TestEconomyTier(t *testing.T) {
	explore := defaultRow(t, "Explore")
	general := defaultRow(t, "general-purpose")
	cases := []struct {
		name      string
		row       policy.DefaultProfile
		risky     bool
		requested contract.Tier
		want      contract.Tier
		ok        bool
	}{
		{"row with a tier goes one down", explore, false, contract.TierFrontier, contract.TierSmall, true},
		{"row at the floor stays small", explore, false, contract.TierMid, contract.TierSmall, true},
		{"row on small stays small", explore, false, contract.TierSmall, contract.TierSmall, true},
		{"risk row never goes below mid", explore, true, contract.TierFrontier, contract.TierMid, true},
		{"risk row on mid stays mid", explore, true, contract.TierMid, contract.TierMid, true},
		{"risk row on small asks for nothing: mid is above the request", explore, true, contract.TierSmall, contract.TierUnknown, false},
		{"keeping row goes one below the request", general, false, contract.TierFrontier, contract.TierMid, true},
		{"keeping row on mid goes small", general, false, contract.TierMid, contract.TierSmall, true},
		{"keeping row on small stays small", general, false, contract.TierSmall, contract.TierSmall, true},
		{"keeping risk row on frontier goes to mid, not small", general, true, contract.TierFrontier, contract.TierMid, true},
		{"keeping risk row on mid stays mid", general, true, contract.TierMid, contract.TierMid, true},
		{"keeping risk row on small asks for nothing", general, true, contract.TierSmall, contract.TierUnknown, false},
		{"unknown request asks for nothing", explore, false, contract.TierUnknown, contract.TierUnknown, false},
		{"unknown request asks for nothing on a keeping row", general, false, contract.TierUnknown, contract.TierUnknown, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := economyTier(c.row, c.risky, c.requested)
			if ok != c.ok || got != c.want {
				t.Errorf("economyTier = (%q, %v), want (%q, %v)", got, ok, c.want, c.ok)
			}
		})
	}
}

// TestEconomyTierInvariants sweeps every default row, every risk state and
// every requested tier, so a new row cannot break the bounds unnoticed.
func TestEconomyTierInvariants(t *testing.T) {
	for _, row := range policy.DefaultProfiles() {
		for _, risky := range []bool{false, true} {
			for _, requested := range contract.Tiers() {
				got, ok := economyTier(row, risky, requested)
				if !ok {
					continue
				}
				if rank(got) > rank(requested) {
					t.Errorf("%s risky=%v requested=%s: tier %s is above the request", row.AgentType, risky, requested, got)
				}
				if risky && got == contract.TierSmall {
					t.Errorf("%s risky=%v requested=%s: risk went to small", row.AgentType, risky, requested)
				}
			}
		}
	}
}

// TestDecideEconomy runs economy mode through the full decision, guard
// included, and checks that the precedence above the table still wins.
func TestDecideEconomy(t *testing.T) {
	economy := &policy.Policy{Mode: policy.ModeEconomy}
	cc := func(agentType, requested string) contract.Request {
		return contract.Request{Harness: ccHarness, RequestedModel: requested, AgentType: agentType}
	}
	withHint := func(r contract.Request, hint contract.Tier) contract.Request {
		r.Hint.Tier = hint
		return r
	}
	pinned := cc("Explore", "frontier-model")
	pinned.Pinned = true
	cases := []struct {
		name    string
		request contract.Request
		pol     *policy.Policy
		risk    []string
		want    want
	}{
		{name: "Explore on frontier goes small",
			request: cc("Explore", "frontier-model"), pol: economy,
			want: want{contract.VerdictDefault, "", "small-model", "", "", contract.TierSmall, core.VerdictDownshift}},
		{name: "Explore on mid goes small",
			request: cc("Explore", "mid-model"), pol: economy,
			want: want{contract.VerdictDefault, "", "small-model", "", "", contract.TierSmall, core.VerdictDownshift}},
		{name: "keeping type on frontier goes mid",
			request: cc("general-purpose", "frontier-model"), pol: economy,
			want: want{contract.VerdictDefault, "", "mid-model", "", "", contract.TierMid, core.VerdictDownshift}},
		{name: "Plan on frontier goes mid",
			request: cc("Plan", "frontier-model"), pol: economy,
			want: want{contract.VerdictDefault, "", "mid-model", "", "", contract.TierMid, core.VerdictDownshift}},
		{name: "keeping type on mid goes small",
			request: cc("general-purpose", "mid-model"), pol: economy,
			want: want{contract.VerdictDefault, "", "small-model", "", "", contract.TierSmall, core.VerdictDownshift}},
		{name: "keeping type on small is kept",
			request: cc("general-purpose", "small-model"), pol: economy,
			want: want{contract.VerdictKept, "", "small-model", "", "", contract.TierSmall, core.VerdictOK}},
		{name: "standard mode keeps the keeping type",
			request: cc("general-purpose", "frontier-model"), pol: &policy.Policy{Mode: policy.ModeStandard},
			want: want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "no policy is standard mode",
			request: cc("general-purpose", "frontier-model"),
			want:    want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		// Risk: the unchanged guard holds any move, and the risk bound keeps mid.
		{name: "risky Explore on frontier is held by the guard",
			request: cc("Explore", "frontier-model"), pol: economy, risk: []string{"secret"},
			want: want{contract.VerdictGuardFloor, contract.ReasonRiskFloor, "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "risky keeping type on frontier is held by the guard, not sent to small",
			request: cc("general-purpose", "frontier-model"), pol: economy, risk: []string{"secret"},
			want: want{contract.VerdictGuardFloor, contract.ReasonRiskFloor, "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "risky keeping type on mid stays mid",
			request: cc("general-purpose", "mid-model"), pol: economy, risk: []string{"auth"},
			want: want{contract.VerdictKept, "", "mid-model", "", "", contract.TierMid, core.VerdictOK}},
		{name: "risky Explore on small is kept, never above the request",
			request: cc("Explore", "small-model"), pol: economy, risk: []string{"auth"},
			want: want{contract.VerdictKept, "", "small-model", "", "", contract.TierSmall, core.VerdictOK}},
		// Precedence above the table: economy never overrides these.
		{name: "declared agent-type profile beats economy",
			request: cc("Explore", "frontier-model"),
			pol: &policy.Policy{Mode: policy.ModeEconomy, Profiles: []policy.Profile{
				{Name: "Explore", Tier: contract.TierMid, Harness: ccHarness}}},
			want: want{contract.VerdictProfile, "", "mid-model", "", "Explore", contract.TierMid, core.VerdictDownshift}},
		{name: "hint beats economy",
			request: withHint(cc("general-purpose", "frontier-model"), contract.TierMid), pol: economy,
			want: want{contract.VerdictHintDown, "", "mid-model", "", "", contract.TierMid, core.VerdictDownshift}},
		{name: "equal hint blocks economy",
			request: withHint(cc("general-purpose", "frontier-model"), contract.TierFrontier), pol: economy,
			want: want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "pinned model beats economy",
			request: pinned, pol: economy,
			want: want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "profile of another harness with the type name blocks economy",
			request: cc("Explore", "frontier-model"),
			pol: &policy.Policy{Mode: policy.ModeEconomy, Profiles: []policy.Profile{
				{Name: "Explore", Tier: contract.TierSmall, Harness: "other"}}},
			want: want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		// An unknown requested model is doubt: nothing moves.
		{name: "unknown requested model asks for nothing",
			request: cc("general-purpose", "stranger-model"), pol: economy,
			want: want{contract.VerdictKept, "", "", "", "", contract.TierUnknown, core.VerdictUnknown}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, n := Decide(Input{Request: c.request, Policy: c.pol, Now: now, Risk: c.risk, Catalog: newCatalog()})
			got := want{n.Verdict, n.Reason, d.Model.ID, n.RuleID, n.Profile, n.TierTo, d.Verdict}
			if got != c.want {
				t.Errorf("got %+v\nwant %+v", got, c.want)
			}
		})
	}
}

// TestEconomyNeverPicksExplicitOnly checks that the guard still refuses an
// explicit_only target chosen by economy mode.
func TestEconomyNeverPicksExplicitOnly(t *testing.T) {
	cat := newCatalog()
	cat.explicit["mid-model"] = true
	d, n := Decide(Input{
		Request: contract.Request{Harness: ccHarness, RequestedModel: "frontier-model", AgentType: "general-purpose"},
		Policy:  &policy.Policy{Mode: policy.ModeEconomy},
		Now:     now,
		Catalog: cat,
	})
	if n.Verdict != contract.VerdictHeldUnknown || n.Reason != contract.ReasonExplicitOnly {
		t.Fatalf("verdict %s reason %s, want held_unknown explicit_only", n.Verdict, n.Reason)
	}
	if d.Model.ID != "frontier-model" {
		t.Fatalf("model %q, want the requested frontier-model kept", d.Model.ID)
	}
}
