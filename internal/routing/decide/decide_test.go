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

type want struct {
	verdict  contract.Verdict
	reason   contract.Reason
	modelID  string // model on the Decision
	rule     contract.RuleID
	profile  contract.ProfileName
	tierTo   contract.Tier
	coreVerd core.Verdict
}

func TestDecide(t *testing.T) {
	reading := func(w contract.Window, pct float64, age time.Duration) []contract.Reading {
		return []contract.Reading{{Harness: h, Window: w, UsedPct: pct, ObservedAt: now.Add(-age), TTL: 5 * time.Minute}}
	}
	cases := []struct {
		name     string
		cat      func() fakeCatalog
		request  contract.Request
		pol      *policy.Policy
		readings []contract.Reading
		risk     []string
		want     want
	}{
		// Step 1: an explicit model.
		{name: "pinned model is kept even when a rule would move it",
			request: req(func(r *contract.Request) { r.Pinned = true; r.Hint.Kind = contract.KindScrap }),
			pol:     testPolicy(),
			want:    want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "pinned model plus a named profile is a config error",
			request: req(func(r *contract.Request) { r.Pinned = true; r.Profile = "cheap" }),
			pol:     testPolicy(),
			want:    want{contract.VerdictRefusedConfig, contract.ReasonProfileOverride, "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		// Step 2: profiles.
		{name: "named profile moves down",
			request: req(func(r *contract.Request) { r.Profile = "cheap" }),
			pol:     testPolicy(),
			want:    want{contract.VerdictProfile, "", "small-model", "", "cheap", contract.TierSmall, core.VerdictDownshift}},
		{name: "profile matched by agent type",
			request: req(func(r *contract.Request) { r.AgentType = "reviewer" }),
			pol:     testPolicy(),
			want:    want{contract.VerdictProfile, "", "mid-model", "", "reviewer", contract.TierMid, core.VerdictDownshift}},
		{name: "profile matched by agent type ignoring case (Explore vs explore)",
			request: req(func(r *contract.Request) { r.AgentType = "Reviewer" }),
			pol:     testPolicy(),
			want:    want{contract.VerdictProfile, "", "mid-model", "", "reviewer", contract.TierMid, core.VerdictDownshift}},
		{name: "profile beats rule and hint",
			request: req(func(r *contract.Request) {
				r.Profile = "reviewer"
				r.Hint = contract.Hint{Kind: contract.KindScrap, Tier: contract.TierSmall}
			}),
			pol:  testPolicy(),
			want: want{contract.VerdictProfile, "", "mid-model", "", "reviewer", contract.TierMid, core.VerdictDownshift}},
		{name: "unknown named profile is refused and the spawn is unchanged",
			request: req(func(r *contract.Request) { r.Profile = "nope" }),
			pol:     testPolicy(),
			want:    want{contract.VerdictRefusedConfig, contract.ReasonUnknownProfile, "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "profile of another harness is refused",
			request: req(func(r *contract.Request) { r.Profile = "elsewhere" }),
			pol:     testPolicy(),
			want:    want{contract.VerdictRefusedConfig, contract.ReasonUnknownProfile, "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "agent type without a profile falls through to the next step",
			request: req(func(r *contract.Request) { r.AgentType = "stranger"; r.Hint.Tier = contract.TierMid }),
			pol:     testPolicy(),
			want:    want{contract.VerdictHintDown, "", "mid-model", "", "", contract.TierMid, core.VerdictDownshift}},
		// Step 3: rules.
		{name: "first matching rule moves down by tier",
			request: req(func(r *contract.Request) { r.Hint.Kind = contract.KindDocs }),
			pol:     testPolicy(),
			want:    want{contract.VerdictRule, "", "small-model", "gated-docs", "", contract.TierSmall, core.VerdictDownshift}},
		{name: "rule gate past its limit skips to the next rule",
			request:  req(func(r *contract.Request) { r.Hint.Kind = contract.KindDocs }),
			pol:      testPolicy(),
			readings: reading(contract.WindowShort, 60, time.Minute),
			want:     want{contract.VerdictRule, "", "mid-model", "docs", "", contract.TierMid, core.VerdictDownshift}},
		{name: "rate limited window fails the gate",
			request: req(func(r *contract.Request) { r.Hint.Kind = contract.KindDocs }),
			pol:     testPolicy(),
			readings: []contract.Reading{{Harness: h, Window: contract.WindowShort, RateLimited: true,
				ObservedAt: now, TTL: time.Minute}},
			want: want{contract.VerdictRule, "", "mid-model", "docs", "", contract.TierMid, core.VerdictDownshift}},
		{name: "stale reading counts as missing and the gate passes",
			request:  req(func(r *contract.Request) { r.Hint.Kind = contract.KindDocs }),
			pol:      testPolicy(),
			readings: reading(contract.WindowShort, 99, time.Hour),
			want:     want{contract.VerdictRule, "", "small-model", "gated-docs", "", contract.TierSmall, core.VerdictDownshift}},
		{name: "rule that names a profile uses its tier",
			request: req(func(r *contract.Request) { r.Hint.Kind = contract.KindScrap }),
			pol:     testPolicy(),
			want:    want{contract.VerdictRule, "", "small-model", "scrap", "cheap", contract.TierSmall, core.VerdictDownshift}},
		{name: "rule naming a missing profile is refused",
			request: req(func(r *contract.Request) { r.Hint.Kind = contract.KindCommand }),
			pol:     testPolicy(),
			want:    want{contract.VerdictRefusedConfig, contract.ReasonUnknownProfile, "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "rule beats the hint tier",
			request: req(func(r *contract.Request) { r.Hint = contract.Hint{Kind: contract.KindDocs, Tier: contract.TierMid} }),
			pol:     testPolicy(),
			want:    want{contract.VerdictRule, "", "small-model", "gated-docs", "", contract.TierSmall, core.VerdictDownshift}},
		// Step 4: the hint only goes down.
		{name: "hint tier moves down",
			request: req(func(r *contract.Request) { r.Hint.Tier = contract.TierSmall }),
			pol:     testPolicy(),
			want:    want{contract.VerdictHintDown, "", "small-model", "", "", contract.TierSmall, core.VerdictDownshift}},
		{name: "hint tier above the request is ignored",
			request: req(func(r *contract.Request) { r.RequestedModel = "mid-model"; r.Hint.Tier = contract.TierFrontier }),
			want:    want{contract.VerdictKept, "", "mid-model", "", "", contract.TierMid, core.VerdictOK}},
		{name: "hint tier equal to the request keeps",
			request: req(func(r *contract.Request) { r.Hint.Tier = contract.TierFrontier }),
			want:    want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		// Step 5: nothing declared.
		{name: "nothing declared keeps the requested model",
			request: req(nil),
			pol:     testPolicy(),
			want:    want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "nil policy keeps the requested model",
			request: req(nil),
			want:    want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		// Guard rules.
		{name: "risk does not block a profile downshift (O8: no built-in floor)",
			request: req(func(r *contract.Request) { r.Profile = "cheap" }), pol: testPolicy(), risk: []string{"secret"},
			want: want{contract.VerdictProfile, "", "small-model", "", "cheap", contract.TierSmall, core.VerdictDownshift}},
		{name: "risk does not block a hint downshift (O8)",
			request: req(func(r *contract.Request) { r.Hint.Tier = contract.TierMid }),
			risk:    []string{"payment"},
			want:    want{contract.VerdictHintDown, "", "mid-model", "", "", contract.TierMid, core.VerdictDownshift}},
		{name: "risk does not block a rule downshift (O8)",
			request: req(func(r *contract.Request) { r.Hint.Kind = contract.KindDocs }),
			pol:     testPolicy(), risk: []string{"secret", "payment"},
			want: want{contract.VerdictRule, "", "small-model", "gated-docs", "", contract.TierSmall, core.VerdictDownshift}},
		{name: "a rule is not applied to a requested model the catalog does not know",
			request: req(func(r *contract.Request) { r.RequestedModel = "who-knows"; r.Hint.Kind = contract.KindDocs }),
			pol:     testPolicy(),
			want:    want{contract.VerdictKept, "", "", "gated-docs", "", contract.TierUnknown, core.VerdictUnknown}},
		{name: "explicit_only target is refused",
			cat: func() fakeCatalog {
				c := newCatalog()
				c.byTier[core.TierSmall] = model("explicit-model", core.TierSmall)
				return c
			},
			request: req(func(r *contract.Request) { r.Profile = "cheap" }),
			pol:     testPolicy(),
			want:    want{contract.VerdictHeldUnknown, contract.ReasonExplicitOnly, "frontier-model", "", "cheap", contract.TierFrontier, core.VerdictOK}},
		{name: "foreign id is refused",
			cat: func() fakeCatalog {
				c := newCatalog()
				c.byTier[core.TierSmall] = model("gpt-elsewhere", core.TierSmall)
				return c
			},
			request: req(func(r *contract.Request) { r.Profile = "cheap" }),
			pol:     testPolicy(),
			want:    want{contract.VerdictHeldUnknown, contract.ReasonForeignID, "frontier-model", "", "cheap", contract.TierFrontier, core.VerdictOK}},
		{name: "never widen: a profile above the request is refused",
			request: req(func(r *contract.Request) { r.RequestedModel = "mid-model"; r.Profile = "top" }),
			pol:     testPolicy(),
			want:    want{contract.VerdictHeldUnknown, contract.ReasonNeverWiden, "mid-model", "", "top", contract.TierMid, core.VerdictOK}},
		{name: "profile at the requested tier moves nothing",
			request: req(func(r *contract.Request) { r.Profile = "top" }),
			pol:     testPolicy(),
			want:    want{contract.VerdictKept, "", "frontier-model", "", "top", contract.TierFrontier, core.VerdictOK}},
		{name: "tier with no candidate keeps the request",
			cat: func() fakeCatalog {
				c := newCatalog()
				delete(c.byTier, core.TierSmall)
				return c
			},
			request: req(func(r *contract.Request) { r.Profile = "cheap" }),
			pol:     testPolicy(),
			want:    want{contract.VerdictHeldUnknown, contract.ReasonNoCandidate, "frontier-model", "", "cheap", contract.TierFrontier, core.VerdictOK}},
		// Doubt.
		{name: "requested model the catalog does not know is kept",
			request: req(func(r *contract.Request) { r.RequestedModel = "who-knows"; r.Profile = "cheap" }),
			pol:     testPolicy(),
			want:    want{contract.VerdictKept, "", "", "", "cheap", contract.TierUnknown, core.VerdictUnknown}},
		{name: "no requested model is kept",
			request: req(func(r *contract.Request) { r.RequestedModel = ""; r.Hint.Tier = contract.TierSmall }),
			want:    want{contract.VerdictKept, "", "", "", "", contract.TierUnknown, core.VerdictUnknown}},
		{name: "a panic fails open",
			cat:     func() fakeCatalog { c := newCatalog(); c.boom = true; return c },
			request: req(func(r *contract.Request) { r.Profile = "cheap" }),
			pol:     testPolicy(),
			want:    want{contract.VerdictRefusedConfig, contract.ReasonFailOpen, "", "", "", contract.TierUnknown, core.VerdictUnknown}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mk := tc.cat
			if mk == nil {
				mk = newCatalog
			}
			d, n := Decide(Input{Request: tc.request, Policy: tc.pol, Readings: tc.readings, Now: now,
				Risk: tc.risk, Catalog: mk()})
			got := want{n.Verdict, n.Reason, d.Model.ID, n.RuleID, n.Profile, n.TierTo, d.Verdict}
			if got != tc.want {
				t.Fatalf("got  %+v\nwant %+v", got, tc.want)
			}
			if d.Reason != string(n.Reason) {
				t.Errorf("Decision.Reason = %q, note reason = %q", d.Reason, n.Reason)
			}
			if n.Harness != h {
				t.Errorf("note harness = %q, want %q", n.Harness, h)
			}
		})
	}
}

func TestDecideNilCatalogKeeps(t *testing.T) {
	d, n := Decide(Input{Request: req(func(r *contract.Request) { r.Profile = "cheap" }), Policy: testPolicy(), Now: now})
	if n.Verdict != contract.VerdictKept || d.Verdict != core.VerdictUnknown || d.Model.ID != "" {
		t.Fatalf("nil catalog must keep: note %+v decision %+v", n, d)
	}
}

func TestDecideFillsDecision(t *testing.T) {
	d, n := Decide(Input{Request: req(func(r *contract.Request) { r.Profile = "cheap" }), Policy: testPolicy(),
		Now: now, Catalog: newCatalog()})
	if d.Tier != core.TierSmall || d.Effort != core.EffortLow || !d.Confident || !d.Checked ||
		d.SafeVerdict != core.VerdictDownshift || d.Harness != string(h) ||
		d.Baseline.CurrentModel.ID != "frontier-model" || d.Baseline.RequestedID != "frontier-model" {
		t.Fatalf("decision not filled as expected: %+v", d)
	}
	if n.TierFrom != contract.TierFrontier || n.TierTo != contract.TierSmall {
		t.Fatalf("note tiers = %q -> %q", n.TierFrom, n.TierTo)
	}
}

func TestDecideRecordsHeadroom(t *testing.T) {
	cases := []struct {
		name string
		rs   []contract.Reading
		want contract.HeadroomVerdict
		win  contract.Window
	}{
		{"no reading is unknown", nil, contract.HeadroomUnknown, ""},
		{"below the threshold is chosen",
			[]contract.Reading{{Harness: h, Window: contract.WindowWeek, UsedPct: 30, ObservedAt: now, TTL: time.Minute}},
			contract.HeadroomChosen, contract.WindowWeek},
		{"past the threshold is ignored but never reroutes",
			[]contract.Reading{{Harness: h, Window: contract.WindowShort, UsedPct: 95, ObservedAt: now, TTL: time.Minute}},
			contract.HeadroomIgnored, contract.WindowShort},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, n := Decide(Input{Request: req(nil), Policy: testPolicy(), Readings: tc.rs, Now: now, Catalog: newCatalog()})
			if n.Headroom != tc.want || n.HeadroomWindow != tc.win {
				t.Fatalf("headroom = %q/%q, want %q/%q", n.Headroom, n.HeadroomWindow, tc.want, tc.win)
			}
			if d.Model.ID != "frontier-model" {
				t.Fatalf("strain rerouted the spawn to %q", d.Model.ID)
			}
		})
	}
}

// Doubt keeps: a decision that moved nothing carries a safe verdict of OK, so
// the channel plan cannot pick a model for it (PlanForSession writes a target
// for an UNKNOWN safe verdict).
func TestDecideKeptNeverAsksForATarget(t *testing.T) {
	boom := newCatalog()
	boom.boom = true
	cases := []struct {
		name    string
		request contract.Request
		cat     fakeCatalog
	}{
		{"unknown requested model", req(func(r *contract.Request) { r.RequestedModel = "who-knows"; r.Profile = "cheap" }), newCatalog()},
		{"no requested model", req(func(r *contract.Request) { r.RequestedModel = ""; r.Hint.Tier = contract.TierSmall }), newCatalog()},
		{"fail open after a panic", req(func(r *contract.Request) { r.Profile = "cheap" }), boom},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, _ := Decide(Input{Request: tc.request, Policy: testPolicy(), Now: now, Catalog: tc.cat})
			if d.Verdict != core.VerdictUnknown || d.SafeVerdict != core.VerdictOK || d.Model.ID != "" {
				t.Fatalf("verdict %v safe %v model %q, want UNKNOWN, OK and no model", d.Verdict, d.SafeVerdict, d.Model.ID)
			}
		})
	}
}
