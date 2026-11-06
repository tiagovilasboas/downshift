// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kirocrew_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/kirocrew"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
)

// The block message names the tier of the model it asks for, not the decision tier.
func TestHandle_BlockMessageTierMatchesTarget(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	writeSessionFile(t, []string{midID, frontierID})
	out, _, d := kirocrew.Handle(spawnEvent("rename the userId variable to userIdentifier", frontierID), cat)
	if !out.Block {
		t.Fatalf("setup: expected block, got allow (verdict=%v corr=%v)", d.Verdict, d.Corrections)
	}
	if strings.Contains(out.Message, "model="+midID) && (strings.Contains(out.Message, "tier small") || strings.Contains(out.Message, "trim context")) {
		t.Fatalf("message names %s (mid) but says small tier / trim hint:\n%s", midID, out.Message)
	}
}
