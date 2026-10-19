// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// One decision has no tokens yet: try says where its tokens would run against
// the requested model, and never quotes a price or a percentage.
func TestTryHeadline_TokensFirstNoPrice(t *testing.T) {
	move := func(v core.Verdict, tier core.Tier, cur, rec string, savings float64) core.Decision {
		return core.Decision{Verdict: v, Tier: tier, Savings: savings,
			Model: core.Model{ID: rec}, Baseline: core.Baseline{CurrentModel: core.Model{ID: cur}}}
	}
	cases := []struct {
		name string
		d    core.Decision
		want string
	}{
		{"priced downshift", move(core.VerdictDownshift, core.TierSmall, "claude-opus-5-5", "claude-haiku-4-5", 0.75),
			"→ small: opus → haiku · its tokens would run on a cheaper model than requested"},
		{"unpriced downshift names the tier only", move(core.VerdictDownshift, core.TierSmall, "opus", "haiku", 0),
			"→ small: opus → haiku · its tokens would run on a lower tier than requested"},
		{"upshift", move(core.VerdictUpshift, core.TierFrontier, "claude-haiku-4-5", "claude-opus-5-5", 0),
			"→ frontier: haiku → opus · its tokens would run on a pricier tier than requested (needs more torque)"},
		{"ok", move(core.VerdictOK, core.TierSmall, "claude-haiku-4-5", "claude-haiku-4-5", 0),
			"→ small: stays on haiku"},
	}
	for _, tc := range cases {
		got := tryHeadline(tc.d)
		if got != tc.want {
			t.Errorf("%s: headline = %q, want %q", tc.name, got, tc.want)
		}
		if strings.ContainsAny(got, "%$") {
			t.Errorf("%s: headline quotes a price or percentage: %q", tc.name, got)
		}
	}
	d := move(core.VerdictDownshift, core.TierSmall, "claude-opus-5-5", "claude-haiku-4-5", 0.75)
	if got, want := tryHeadlineNotApplied(d), "→ small: opus → haiku (not applied, tokens stay on opus)"; got != want {
		t.Errorf("not applied = %q, want %q", got, want)
	}
}
