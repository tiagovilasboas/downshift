// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// TestKiroCrewAnthropicIDsResolveExactly covers the Anthropic ids kiro-cli
// listed on 2026-10-23. Each must be an exact id or alias with its own row: a
// family-prefix match would price Opus 4.5 as Opus 5.5 and record it under
// that model.
func TestKiroCrewAnthropicIDsResolveExactly(t *testing.T) {
	c, err := parse(embeddedJSON)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		kiroID, catalogID string
		tier              core.Tier
	}{
		{"claude-opus-5.5", "claude-opus-5-5", core.TierFrontier},
		{"claude-opus-5", "claude-opus-5", core.TierFrontier},
		{"claude-opus-4.8", "claude-opus-4-8", core.TierFrontier},
		{"claude-opus-4.7", "claude-opus-4-7", core.TierFrontier},
		{"claude-opus-4.6", "claude-opus-4-6", core.TierFrontier},
		{"claude-opus-4.5", "claude-opus-4-5", core.TierFrontier},
		{"claude-sonnet-5.5", "claude-sonnet-5-5", core.TierMid},
		{"claude-sonnet-5", "claude-sonnet-5", core.TierMid},
		{"claude-sonnet-4.6", "claude-sonnet-4-6", core.TierMid},
		{"claude-sonnet-4.5", "claude-sonnet-4-5", core.TierMid},
		{"claude-haiku-4.5", "claude-haiku-4-5", core.TierSmall},
	}
	for _, tc := range cases {
		if !c.IsExactID("kirocrew", tc.kiroID) {
			t.Errorf("%s: not an exact kirocrew id or alias", tc.kiroID)
			continue
		}
		m, ok := c.LookupByID("kirocrew", tc.kiroID)
		if !ok || m.ID != tc.catalogID || m.Tier != tc.tier {
			t.Errorf("%s resolves to %q (%v), want %q (%v)", tc.kiroID, m.ID, m.Tier, tc.catalogID, tc.tier)
		}
	}
	// The automatic targets stay the 5.5 rows: the older ids are kept only when
	// a spawn already asks for them.
	if got := c.ModelFor("kirocrew", core.TierFrontier).ID; got != "claude-opus-5-5" {
		t.Errorf("kirocrew frontier target = %q, want claude-opus-5-5", got)
	}
	if got := c.ModelFor("kirocrew", core.TierMid).ID; got != "claude-sonnet-5-5" {
		t.Errorf("kirocrew mid target = %q, want claude-sonnet-5-5", got)
	}
}
