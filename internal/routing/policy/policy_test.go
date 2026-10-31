// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// Rules are first-match-wins, in declaration order. A disabled rule is skipped.
// A rule with a kind or effort list admits only a call that declares a value in
// it; a call with no declared value matches only rules open on that dimension.
func TestFirstMatchWinsInDeclarationOrder(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "code-small", Enabled: true, Kinds: []contract.Kind{contract.KindCode}, Tier: contract.TierSmall},
		{ID: "deep-frontier", Enabled: true, Efforts: []contract.EffortClass{contract.EffortDeep}, Tier: contract.TierFrontier},
		{ID: "open-mid", Enabled: true, Tier: contract.TierMid},
	}}
	cases := []struct {
		kind   contract.Kind
		effort contract.EffortClass
		want   contract.RuleID
	}{
		{contract.KindCode, contract.EffortDeep, "code-small"},
		{contract.KindDocs, contract.EffortDeep, "deep-frontier"},
		{contract.KindDocs, contract.EffortLight, "open-mid"},
		{contract.KindNone, contract.EffortNone, "open-mid"},
		{contract.KindCode, contract.EffortNone, "code-small"},
	}
	for _, c := range cases {
		r, ok := p.Match(MatchContext{Kind: c.kind, Effort: c.effort})
		if !ok || r.ID != c.want {
			t.Errorf("Match(%q,%q) = %q,%v; want %q", c.kind, c.effort, r.ID, ok, c.want)
		}
	}

	p.Rules[0].Enabled = false
	if r, _ := p.Match(MatchContext{Kind: contract.KindCode, Effort: contract.EffortDeep}); r.ID != "deep-frontier" {
		t.Errorf("with code-small disabled, Match = %q, want deep-frontier", r.ID)
	}
	p.Rules = p.Rules[1:2]
	if _, ok := p.Match(MatchContext{Kind: contract.KindNone, Effort: contract.EffortNone}); ok {
		t.Error("an effort-only rule matched a call with no declared effort")
	}
}

// A rule with a classes gate admits a call only when the classifier's class is
// in the set, the classification is confident, and the task has no risk. A
// rule without a classes gate ignores all three, so earlier policies behave
// exactly as before.
func TestClassGateMatchesOnlyConfidentRiskFree(t *testing.T) {
	p := &Policy{Rules: []Rule{
		{ID: "small-when-easy", Enabled: true, Classes: []contract.Complexity{contract.ComplexityTrivial, contract.ComplexitySimple}, Tier: contract.TierSmall},
		{ID: "open", Enabled: true, Tier: contract.TierMid},
	}}
	cases := []struct {
		name string
		ctx  MatchContext
		want contract.RuleID
	}{
		// Happy path: a confident, risk-free class in the set takes the class rule.
		{"confident TRIVIAL no risk goes to the class rule",
			MatchContext{Class: contract.ComplexityTrivial, Confident: true}, "small-when-easy"},
		{"confident SIMPLE no risk goes to the class rule",
			MatchContext{Class: contract.ComplexitySimple, Confident: true}, "small-when-easy"},
		// Unhappy paths: each one falls through to the open rule.
		{"TRIVIAL with risk falls through",
			MatchContext{Class: contract.ComplexityTrivial, Confident: true, HasRisk: true}, "open"},
		{"TRIVIAL not confident falls through",
			MatchContext{Class: contract.ComplexityTrivial, Confident: false}, "open"},
		{"MEDIUM is not in the set and falls through",
			MatchContext{Class: contract.ComplexityMedium, Confident: true}, "open"},
		{"an unknown class falls through",
			MatchContext{Class: contract.ComplexityUnknown, Confident: true}, "open"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, ok := p.Match(c.ctx)
			if !ok || r.ID != c.want {
				t.Fatalf("Match(%+v) = %q,%v; want %q", c.ctx, r.ID, ok, c.want)
			}
		})
	}

	// With only the class rule, an unconfident call matches nothing: the
	// requested model is kept.
	p.Rules = p.Rules[:1]
	if _, ok := p.Match(MatchContext{Class: contract.ComplexityTrivial, Confident: false}); ok {
		t.Error("an unconfident class matched a class rule; doubt must keep")
	}
}

func TestCheckProfileIsTheOnlySource(t *testing.T) {
	p := &Policy{Profiles: []Profile{
		{Name: "reviewer", Tier: contract.TierFrontier, Harness: "claude-code"},
	}}
	cases := []struct {
		name   string
		req    contract.Request
		reason contract.Reason
	}{
		{"no profile passes", contract.Request{Harness: "claude-code", RequestedModel: "x", Pinned: true}, contract.ReasonNone},
		{"profile alone passes", contract.Request{Harness: "claude-code", Profile: "reviewer"}, contract.ReasonNone},
		{"profile with kind and effort passes", contract.Request{Harness: "claude-code", Profile: "reviewer", Hint: contract.Hint{Kind: contract.KindReview, Effort: contract.EffortDeep}}, contract.ReasonNone},
		{"profile with pinned model is refused", contract.Request{Harness: "claude-code", Profile: "reviewer", Pinned: true}, contract.ReasonProfileOverride},
		{"profile with hint tier is refused", contract.Request{Harness: "claude-code", Profile: "reviewer", Hint: contract.Hint{Tier: contract.TierSmall}}, contract.ReasonProfileOverride},
		{"unknown profile is refused", contract.Request{Harness: "claude-code", Profile: "ghost"}, contract.ReasonUnknownProfile},
		{"profile on another harness is refused", contract.Request{Harness: "kirocrew", Profile: "reviewer"}, contract.ReasonForeignID},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := p.CheckProfile(c.req)
			if c.reason == contract.ReasonNone {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			ce, ok := err.(*ConfigError)
			if !ok || ce.Reason != c.reason {
				t.Fatalf("CheckProfile = %v, want reason %q", err, c.reason)
			}
		})
	}
	var nilPolicy *Policy
	if err := nilPolicy.CheckProfile(contract.Request{Profile: "reviewer"}); err == nil {
		t.Error("a nil policy accepted a named profile")
	}
}

func TestBothCriticalEnumIsClosed(t *testing.T) {
	for _, v := range BothCriticals() {
		if !v.Valid() {
			t.Errorf("%q not valid", v)
		}
	}
	for _, bad := range []BothCritical{"", "Block", "cheapest", "run cheapest"} {
		if bad.Valid() {
			t.Errorf("%q accepted", bad)
		}
	}
}
