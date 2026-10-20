// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

// Economy only lowers a confident, risk-free task one step. Risk and doubt
// must still hold, so economy never moves a tier up and never touches a risky
// or unconfident task.
func TestEconomyLowersOnlyConfidentRiskFreeTasks(t *testing.T) {
	prompts := []string{
		"rename the variable total to sum in utils.go",
		"fix a typo in the README heading",
		"add a unit test for the parser",
		"refactor the billing module to use a new payment provider",
		"migrate the production database schema and drop the legacy table",
		"review the authentication flow for security issues",
		"do something",
	}
	lowered := 0
	for _, p := range prompts {
		off := withEconomy(t, false, func() Decision { return Route(p, "claude-code", "") })
		on := withEconomy(t, true, func() Decision { return Route(p, "claude-code", "") })
		if on.Tier < off.Tier {
			lowered++
		}
		if on.Tier > off.Tier {
			t.Errorf("%q: economy raised the tier %v -> %v", p, off.Tier, on.Tier)
		}
		if len(on.Risk) > 0 || on.Guards.RiskFloor || !on.Confident {
			if on.Tier != off.Tier {
				t.Errorf("%q: economy changed a risky or unconfident task (%v -> %v)", p, off.Tier, on.Tier)
			}
		}
	}
	if lowered == 0 {
		t.Fatal("economy lowered no task; the guard checks are vacuous")
	}
}

func withEconomy(t *testing.T, on bool, route func() Decision) Decision {
	t.Helper()
	restore := SetConfig(Config{Economy: on})
	defer restore()
	return route()
}
