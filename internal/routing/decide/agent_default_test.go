// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

// The agent's declared tier applies only to an inherited model, only downward,
// and routing.agent_default_tier: false turns it off.
func TestDecideAgentDefault(t *testing.T) {
	on, off := testPolicy(), testPolicy()
	on.AgentDefault, off.AgentDefault = true, false
	build := func(pol *policy.Policy, inherited bool, tier contract.Tier) Input {
		return Input{
			Request: req(func(r *contract.Request) { r.ModelInherited = inherited }),
			Policy:  pol, Now: now, Catalog: newCatalog(), AgentDefaultTier: tier,
		}
	}
	cases := []struct {
		name    string
		in      Input
		verdict contract.Verdict
	}{
		{"inherited + small default moves down", build(on, true, contract.TierSmall), contract.VerdictAgentDefault},
		{"inherited + mid default moves down", build(on, true, contract.TierMid), contract.VerdictAgentDefault},
		{"default equal to inherited keeps", build(on, true, contract.TierFrontier), contract.VerdictKept},
		{"named model wins", build(on, false, contract.TierSmall), contract.VerdictKept},
		{"policy switch off keeps", build(off, true, contract.TierSmall), contract.VerdictKept},
		{"no declared tier keeps", build(on, true, contract.TierUnknown), contract.VerdictKept},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, n := Decide(c.in)
			if n.Verdict != c.verdict {
				t.Fatalf("verdict = %q, want %q", n.Verdict, c.verdict)
			}
		})
	}
}
