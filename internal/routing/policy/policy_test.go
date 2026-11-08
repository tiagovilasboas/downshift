// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/routing/contract"
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
		r, ok := p.Match(c.kind, c.effort)
		if !ok || r.ID != c.want {
			t.Errorf("Match(%q,%q) = %q,%v; want %q", c.kind, c.effort, r.ID, ok, c.want)
		}
	}

	p.Rules[0].Enabled = false
	if r, _ := p.Match(contract.KindCode, contract.EffortDeep); r.ID != "deep-frontier" {
		t.Errorf("with code-small disabled, Match = %q, want deep-frontier", r.ID)
	}
	p.Rules = p.Rules[1:2]
	if _, ok := p.Match(contract.KindNone, contract.EffortNone); ok {
		t.Error("an effort-only rule matched a call with no declared effort")
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
