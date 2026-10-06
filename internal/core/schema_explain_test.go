// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

// A live schema change without downtime is mid-tier work even when the edit
// is "add a column": it needs a backfill, a concurrent index build and lock
// timeouts. It used to route to small on the "add ... column" signal.
func TestRoute_ZeroDowntimeSchemaChangeIsNotSmall(t *testing.T) {
	t.Setenv("DOWNSHIFT_GRAPHIFY_CMD", "")
	prompts := []string{
		"Add a non-null status text column defaulting to 'active', and index it, on a large Postgres table that serves live traffic, with no downtime.",
		"We need a zero-downtime way to add a required region column to the busy customers table.",
	}
	for _, p := range prompts {
		d := core.Route(p, "claude-code", "claude-opus-5-5", cat)
		if d.Tier != core.TierMid {
			t.Errorf("tier %s, want mid for %q", d.Tier, p)
		}
	}
}

// "explain" is a simple-request signal only when it leads the prompt. Buried
// in a hard analysis it used to pull frontier work down to small.
func TestRoute_ExplainInsideAnalysisIsNotSmall(t *testing.T) {
	t.Setenv("DOWNSHIFT_GRAPHIFY_CMD", "")
	prompts := []string{
		"Several worker threads read a shared settings map and occasionally see an old value right after a reload; explain the visibility bug and fix it without a global mutex.",
		"After a consumer group rebalance some payment events get handled twice. Explain every path that allows this and propose an end-to-end exactly-once scheme.",
	}
	for _, p := range prompts {
		if d := core.Route(p, "claude-code", "claude-opus-5-5", cat); d.Tier == core.TierSmall {
			t.Errorf("routed to small: %q", p)
		}
	}
	for _, p := range []string{"explain what a closure is in JavaScript", "Can you explain how this list comprehension works?"} {
		if d := core.Route(p, "claude-code", "claude-opus-5-5", cat); d.Tier != core.TierSmall {
			t.Errorf("leading explain should stay small, got %s for %q", d.Tier, p)
		}
	}
}
