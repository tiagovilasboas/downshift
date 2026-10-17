// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

func TestRaiseTierOnlyRaises(t *testing.T) {
	d := Route("rename the variable userId to accountId in auth.ts", "claude-code", "")
	if d.Tier != TierSmall {
		t.Skipf("fixture prompt no longer classifies small (got %v)", d.Tier)
	}
	if !d.RaiseTier(TierFrontier, "", nil) {
		t.Fatal("expected a change")
	}
	if d.Tier != TierFrontier || d.Effort != EffortFor(TierFrontier) || !d.Guards.PolicyFloor {
		t.Fatalf("decision not lifted: %+v", d)
	}
	if d.Model.Tier != TierFrontier {
		t.Fatalf("model must follow the tier, got %v", d.Model.Tier)
	}
	if d.RaiseTier(TierSmall, "", nil) || d.RaiseTier(TierFrontier, "", nil) {
		t.Fatal("equal or lower floor must not change the decision")
	}
	if d.Tier != TierFrontier {
		t.Fatal("tier must not be lowered")
	}
}

func TestRaiseTierLeavesHigherTierAlone(t *testing.T) {
	d := Route("rearchitect the payment flow across all services with a migration plan", "claude-code", "")
	before := d
	if d.Tier == TierSmall {
		t.Skip("fixture prompt unexpectedly small")
	}
	if d.RaiseTier(TierSmall, "", nil) {
		t.Fatal("a lower floor must be a no-op")
	}
	if d.Tier != before.Tier || d.Guards.PolicyFloor {
		t.Fatal("no-op must not touch the decision")
	}
}

func TestRaiseTierClearsAppliedNBDownshift(t *testing.T) {
	d := Decision{
		Harness: "claude-code",
		Tier:    TierSmall,
		Shadow: Shadow{
			NBDownshift: NBDownshift{Would: true, Applied: true, Margin: 1},
		},
	}
	if !d.RaiseTier(TierMid, "", nil) {
		t.Fatal("expected a change")
	}
	if d.Shadow.NBDownshift.Applied {
		t.Fatal("a raised tier must not report the NB downshift as applied")
	}
	if !d.Shadow.NBDownshift.Would {
		t.Fatal("the opinion itself must stay recorded")
	}
}
