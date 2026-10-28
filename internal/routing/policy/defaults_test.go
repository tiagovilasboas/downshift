// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

func TestDefaultFor(t *testing.T) {
	const cc contract.HarnessID = "claude-code"
	cases := []struct {
		name      string
		harness   contract.HarnessID
		agentType string
		found     bool
		keeps     bool
		tier      contract.Tier
		riskTier  contract.Tier
	}{
		{"Explore goes small", cc, "Explore", true, false, contract.TierSmall, contract.TierMid},
		{"claude-code-guide goes small", cc, "claude-code-guide", true, false, contract.TierSmall, contract.TierMid},
		{"statusline-setup goes small", cc, "statusline-setup", true, false, contract.TierSmall, contract.TierMid},
		{"general-purpose keeps", cc, "general-purpose", true, true, contract.TierUnknown, contract.TierUnknown},
		{"Plan keeps", cc, "Plan", true, true, contract.TierUnknown, contract.TierUnknown},
		{"match ignores case", cc, "explore", true, false, contract.TierSmall, contract.TierMid},
		{"unknown type has no row", cc, "mechanical-worker", false, false, contract.TierUnknown, contract.TierUnknown},
		{"empty type has no row", cc, "", false, false, contract.TierUnknown, contract.TierUnknown},
		{"other harness has no row", "cursor", "Explore", false, false, contract.TierUnknown, contract.TierUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, ok := DefaultFor(c.harness, c.agentType)
			if ok != c.found {
				t.Fatalf("DefaultFor(%q, %q) found = %v, want %v", c.harness, c.agentType, ok, c.found)
			}
			if !ok {
				return
			}
			if d.Keeps() != c.keeps {
				t.Errorf("Keeps() = %v, want %v", d.Keeps(), c.keeps)
			}
			if got := d.TierFor(false); got != c.tier {
				t.Errorf("TierFor(no risk) = %q, want %q", got, c.tier)
			}
			if got := d.TierFor(true); got != c.riskTier {
				t.Errorf("TierFor(risk) = %q, want %q", got, c.riskTier)
			}
		})
	}
}

// TestDefaultProfilesInvariants holds the bounds every row must keep: a known
// harness, a type, and a risk tier that is never small.
func TestDefaultProfilesInvariants(t *testing.T) {
	rows := DefaultProfiles()
	if len(rows) == 0 {
		t.Fatal("default profile table is empty")
	}
	seen := map[string]bool{}
	for _, d := range rows {
		key := string(d.Harness) + "/" + d.AgentType
		if seen[key] {
			t.Errorf("duplicate row %s", key)
		}
		seen[key] = true
		if d.Harness == "" || d.AgentType == "" {
			t.Errorf("row %s lacks a harness or a type", key)
		}
		if d.Keeps() {
			if d.RiskTier != contract.TierUnknown {
				t.Errorf("row %s keeps but declares a risk tier", key)
			}
			continue
		}
		if !d.RiskTier.Valid() || d.RiskTier == contract.TierSmall {
			t.Errorf("row %s risk tier = %q, want mid or above", key, d.RiskTier)
		}
	}
}

func TestDefaultProfilesReturnsCopy(t *testing.T) {
	rows := DefaultProfiles()
	rows[0].Tier = contract.TierFrontier
	if DefaultProfiles()[0].Tier == contract.TierFrontier {
		t.Error("DefaultProfiles exposes the shared table")
	}
}
