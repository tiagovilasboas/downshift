// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// TestShiftMatrix checks the universal gear rule on the real catalog:
// downshift only when the current tier is above the task, upshift only when
// it is below, and an uncertain call must not rewrite the model.
func TestShiftMatrix(t *testing.T) {
	t.Setenv("DOWNSHIFT_MINILM", "0")
	cases := []struct {
		name       string
		prompt     string
		complexity core.Complexity
		tier       core.Tier
	}{
		{"trivial rename", "rename the userId variable to user_id", core.Trivial, core.TierSmall},
		{"simple field", "add a field email to the User struct", core.Simple, core.TierSmall},
		{"cube", "implement a three.js scene with a rotating cube", core.Simple, core.TierSmall},
		{"medium feature", "implement the feature to export sales as CSV", core.Medium, core.TierMid},
		{"complex rearchitecture", "rearchitect the auth module to support multi-tenant organisations", core.Complex, core.TierFrontier},
		{"vague", "help me with this", core.Medium, core.TierMid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := classifiedRoute(tc.prompt, "claude-code", "")
			if d.Complexity != tc.complexity || d.Tier != tc.tier {
				t.Fatalf("complexity %s tier %s, want %s %s", d.Complexity, d.Tier, tc.complexity, tc.tier)
			}
		})
	}
}
