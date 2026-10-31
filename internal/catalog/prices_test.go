// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"math"
	"strings"
	"testing"
	"time"
)

// Price coherence of the embedded catalog. These tests are the cheap half of
// the "prices never go stale silently" policy: the weekly sync-prices workflow
// refreshes the numbers, and these fail the PR if the refreshed (or hand-edited)
// data contradicts itself.

func embeddedEntries(t *testing.T) []Entry {
	t.Helper()
	c, err := parse(embeddedJSON)
	if err != nil {
		t.Fatal(err)
	}
	var out []Entry
	for _, e := range c.Entries() {
		if e.ID != "" && e.Harness != "" {
			out = append(out, e)
		}
	}
	return out
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// normID folds an id or alias to the form used to compare across harnesses.
func normID(s string) string {
	return strings.ToLower(strings.ReplaceAll(normaliseModelID(s), ".", "-"))
}

func TestPricesAsOf_ParsesAndIsNotInTheFuture(t *testing.T) {
	c := Load()
	asOf := c.PricesAsOf()
	d, err := time.Parse("2006-01-02", asOf)
	if err != nil {
		t.Fatalf("prices_as_of %q must be YYYY-MM-DD: %v", asOf, err)
	}
	if d.After(time.Now().UTC().Add(24 * time.Hour)) {
		t.Errorf("prices_as_of %s is in the future", asOf)
	}
}

// Every routable entry needs both prices; a zero silently turns a saving into
// nothing (or an overspend into a saving). opencode zen models are the one
// documented exception: they have no published list price.
func TestEmbeddedTieredEntriesHaveInputAndOutputPrices(t *testing.T) {
	for _, e := range embeddedEntries(t) {
		if _, ok := parseTier(e.Tier); !ok || e.Harness == "opencode" {
			continue
		}
		if e.InputCostM <= 0 || e.OutputCostM <= 0 {
			t.Errorf("%s/%s (tier %s) has input=%v output=%v; both must be > 0",
				e.Harness, e.ID, e.Tier, e.InputCostM, e.OutputCostM)
		}
	}
}

// A name that is both an alias of one row and the id of another priced row
// must not carry two different prices, or the same model id is billed two ways.
func TestEmbeddedAliasPriceMatchesTheModelItNames(t *testing.T) {
	entries := embeddedEntries(t)
	for _, e := range entries {
		for _, a := range e.Aliases {
			for _, o := range entries {
				if o.ID == e.ID && o.Harness == e.Harness {
					continue
				}
				if o.Provider != e.Provider || normID(o.ID) != normID(a) {
					continue
				}
				if !near(o.InputCostM, e.InputCostM) || !near(o.OutputCostM, e.OutputCostM) {
					t.Errorf("%s/%s lists alias %q = %s/%s, which is priced %v/%v (alias row %v/%v)",
						e.Harness, e.ID, a, o.Harness, o.ID, o.InputCostM, o.OutputCostM, e.InputCostM, e.OutputCostM)
				}
			}
		}
	}
}

// anthropicCacheReadFactor is the cache-read multiple of base input, from the
// list-price table (https://platform.claude.com/docs/en/about-claude/pricing,
// verified 2026-10-19): 0.1x by default, 0.05x on Opus 5.5 and Sonnet 5.5,
// 0.025x on Fable 5.1.
func anthropicCacheReadFactor(id string) float64 {
	n := normID(id)
	switch {
	case strings.Contains(n, "opus-5-5"), strings.Contains(n, "sonnet-5-5"):
		return 0.05
	case strings.Contains(n, "fable-5-1"):
		return 0.025
	}
	return 0.1
}

func TestEmbeddedAnthropicCachePricesFollowListMultipliers(t *testing.T) {
	for _, e := range embeddedEntries(t) {
		if e.Provider != "anthropic" {
			continue
		}
		if e.CacheReadCostM > 0 {
			if want := e.InputCostM * anthropicCacheReadFactor(e.ID); !near(e.CacheReadCostM, want) {
				t.Errorf("%s/%s cache read = %v, want %v (%vx input %v)",
					e.Harness, e.ID, e.CacheReadCostM, want, anthropicCacheReadFactor(e.ID), e.InputCostM)
			}
		}
		if e.CacheWriteCostM > 0 {
			if want := e.InputCostM * 1.25; !near(e.CacheWriteCostM, want) {
				t.Errorf("%s/%s cache write (5m) = %v, want %v (1.25x input %v)",
					e.Harness, e.ID, e.CacheWriteCostM, want, e.InputCostM)
			}
		}
	}
}

// Older Anthropic models are priced differently from the 5.x rows that share
// their family; they need their own exact rows (they once aliased the 5.x rows).
func TestEmbeddedOlderClaudeModelsKeepTheirOwnPrices(t *testing.T) {
	c := Load()
	for _, tc := range []struct {
		harness, id string
		in, out     float64
	}{
		{"claude-code", "claude-sonnet-4-6", 3, 15},
		{"claude-code", "claude-opus-4-8", 5, 25},
		{"claude-code", "claude-fable-5", 10, 50},
		{"kirocrew", "claude-sonnet-4.6", 3, 15},
		{"kirocrew", "claude-opus-4.8", 5, 25},
	} {
		m, ok := c.LookupByID(tc.harness, tc.id)
		if !ok || !near(m.InputM, tc.in) || !near(m.OutputM, tc.out) {
			t.Errorf("%s/%s = %v/%v ok=%v; want %v/%v", tc.harness, tc.id, m.InputM, m.OutputM, ok, tc.in, tc.out)
		}
	}
}
