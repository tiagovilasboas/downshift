// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

func rd(id contract.HarnessID, w contract.Window, pct float64, age time.Duration) contract.Reading {
	return contract.Reading{Harness: id, Window: w, UsedPct: pct, ObservedAt: now.Add(-age), TTL: 5 * time.Minute}
}

func TestDecideGates(t *testing.T) {
	docs := req(func(r *contract.Request) { r.Hint.Kind = contract.KindDocs })
	cases := []struct {
		name     string
		readings []contract.Reading
		wantRule contract.RuleID
		wantID   string
	}{
		{"gate passes below the limit", []contract.Reading{rd(h, contract.WindowShort, 30, time.Minute)}, "gated-docs", "small-model"},
		{"gate fails at the limit and the next rule is tried", []contract.Reading{rd(h, contract.WindowShort, 50, time.Minute)}, "docs", "mid-model"},
		{"gate on another window is not read", []contract.Reading{rd(h, contract.WindowWeek, 99, time.Minute)}, "gated-docs", "small-model"},
		{"stale reading passes", []contract.Reading{rd(h, contract.WindowShort, 99, time.Hour)}, "gated-docs", "small-model"},
		{"no reading passes", nil, "gated-docs", "small-model"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, n := Decide(Input{Request: docs, Policy: testPolicy(), Readings: tc.readings, Now: now, Catalog: newCatalog()})
			if n.RuleID != tc.wantRule || d.Model.ID != tc.wantID {
				t.Fatalf("rule %q model %q, want %q %q", n.RuleID, d.Model.ID, tc.wantRule, tc.wantID)
			}
		})
	}
}

func TestDecideAllProvidersCritical(t *testing.T) {
	scrap := req(func(r *contract.Request) { r.Hint.Kind = contract.KindScrap })
	hot := rd(h, contract.WindowShort, 95, time.Minute)
	cases := []struct {
		name     string
		readings []contract.Reading
		wantID   string
		verdict  contract.Verdict
	}{
		{"all critical keeps the request and warns",
			[]contract.Reading{hot, rd("other", contract.WindowWeek, 90, time.Minute)}, "frontier-model", contract.VerdictKept},
		{"one provider with room leaves routing open",
			[]contract.Reading{hot, rd("other", contract.WindowWeek, 10, time.Minute)}, "small-model", contract.VerdictRule},
		{"a provider with no reading is not critical",
			[]contract.Reading{hot}, "small-model", contract.VerdictRule},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, n := Decide(Input{Request: scrap, Policy: testPolicy(), Readings: tc.readings, Now: now, Catalog: newCatalog()})
			if d.Model.ID != tc.wantID || n.Verdict != tc.verdict {
				t.Fatalf("model %q verdict %q, want %q %q", d.Model.ID, n.Verdict, tc.wantID, tc.verdict)
			}
			if tc.verdict == contract.VerdictKept && (n.Headroom != contract.HeadroomIgnored || n.HeadroomWindow != contract.WindowShort) {
				t.Fatalf("warning missing from the note: %+v", n)
			}
		})
	}
}

func TestDecideProfileFallback(t *testing.T) {
	pol := func() *policy.Policy {
		p := testPolicy()
		p.Profiles = append(p.Profiles,
			policy.Profile{Name: "careful", Tier: contract.TierMid, Harness: h},
			policy.Profile{Name: "deep", Tier: contract.TierFrontier, Harness: h, Fallback: "careful"},
			policy.Profile{Name: "wide", Tier: contract.TierSmall, Harness: h, Fallback: "top"},
			policy.Profile{Name: "abroad", Tier: contract.TierMid, Harness: h, Fallback: "elsewhere"})
		return p
	}
	hot := []contract.Reading{rd(h, contract.WindowShort, 95, time.Minute), rd("other", contract.WindowWeek, 10, time.Minute)}
	cases := []struct {
		name     string
		profile  contract.ProfileName
		readings []contract.Reading
		want     contract.ProfileName
	}{
		{"critical provider takes the fallback", "deep", hot, "careful"},
		{"healthy provider keeps the profile", "deep", []contract.Reading{rd(h, contract.WindowShort, 10, time.Minute)}, "deep"},
		{"fallback above the tier is ignored", "wide", hot, "wide"},
		{"fallback of another harness is ignored", "abroad", hot, "abroad"},
		{"profile without a fallback stays", "top", hot, "top"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, n := Decide(Input{Request: req(func(r *contract.Request) { r.Profile = tc.profile }),
				Policy: pol(), Readings: tc.readings, Now: now, Catalog: newCatalog()})
			if n.Profile != tc.want {
				t.Fatalf("profile %q, want %q", n.Profile, tc.want)
			}
		})
	}
}

func TestDecideFloor(t *testing.T) {
	min := func(tier string) *policy.FloorPolicy { return &policy.FloorPolicy{Version: 1, MinTier: tier} }
	cases := []struct {
		name    string
		request contract.Request
		floor   *policy.FloorPolicy
		risk    []string
		wantID  string
		verdict contract.Verdict
		core    core.Verdict
	}{
		{"floor raises a downshift",
			req(func(r *contract.Request) { r.Profile = "cheap" }), min("mid"),
			nil, "mid-model", contract.VerdictGuardFloor, core.VerdictDownshift},
		{"floor raises even when never_widen refused the rule",
			req(func(r *contract.Request) { r.RequestedModel = "small-model"; r.Profile = "top" }), min("mid"),
			nil, "mid-model", contract.VerdictGuardFloor, core.VerdictUpshift},
		{"floor below the decided tier never lowers",
			req(func(r *contract.Request) { r.Profile = "reviewer" }), min("small"),
			nil, "mid-model", contract.VerdictProfile, core.VerdictDownshift},
		{"floor by risk category",
			req(func(r *contract.Request) { r.RequestedModel = "small-model" }),
			&policy.FloorPolicy{Version: 1, MinTierByRisk: map[string]string{"payment": "frontier"}},
			[]string{"payment"}, "frontier-model", contract.VerdictGuardFloor, core.VerdictUpshift},
		{"no floor leaves the decision", req(func(r *contract.Request) { r.Profile = "cheap" }), nil,
			nil, "small-model", contract.VerdictProfile, core.VerdictDownshift},
		{"unknown requested model is not floored", req(func(r *contract.Request) { r.RequestedModel = "who-knows" }), min("frontier"),
			nil, "", contract.VerdictKept, core.VerdictUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, n := Decide(Input{Request: tc.request, Policy: testPolicy(), Now: now, Catalog: newCatalog(),
				Floor: tc.floor, Class: "MEDIUM", Risk: tc.risk})
			if d.Model.ID != tc.wantID || n.Verdict != tc.verdict || d.Verdict != tc.core {
				t.Fatalf("model %q verdict %q/%v, want %q %q/%v", d.Model.ID, n.Verdict, d.Verdict, tc.wantID, tc.verdict, tc.core)
			}
			if tc.verdict == contract.VerdictGuardFloor && (!d.Guards.PolicyFloor || n.TierTo != contract.TierOf(d.Tier)) {
				t.Fatalf("floor not recorded: %+v %+v", d.Guards, n)
			}
		})
	}
}
