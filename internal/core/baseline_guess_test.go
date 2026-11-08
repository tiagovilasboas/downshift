// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

func TestHoldGuessedDownshift(t *testing.T) {
	cases := []struct {
		name    string
		guessed bool
		verdict core.Verdict
		tier    core.Tier
		held    bool
	}{
		{"guessed downshift to mid is held", true, core.VerdictDownshift, core.TierMid, true},
		{"guessed downshift to small is written", true, core.VerdictDownshift, core.TierSmall, false},
		{"seen baseline downshift to mid is written", false, core.VerdictDownshift, core.TierMid, false},
		{"guessed upshift is not this rule", true, core.VerdictUpshift, core.TierFrontier, false},
	}
	for _, c := range cases {
		d := core.Decision{Verdict: c.verdict}
		d.Baseline.Guessed = c.guessed
		if got := d.HoldGuessedDownshift(core.Model{Tier: c.tier}); got != c.held || d.Baseline.GuessHeld != c.held {
			t.Errorf("%s: held=%v GuessHeld=%v, want %v", c.name, got, d.Baseline.GuessHeld, c.held)
		}
	}
}
