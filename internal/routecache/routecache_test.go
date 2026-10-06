// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package routecache_test

import (
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/routecache"
)

func testKey(prompt string) routecache.Key {
	return routecache.Key{Prompt: prompt, Harness: "claude-code", CurrentModel: "frontier"}
}

func TestCache_MissThenHit(t *testing.T) {
	c := routecache.New(16)
	if _, ok := c.Get(testKey("a")); ok {
		t.Fatal("empty cache must miss")
	}
	want := core.Decision{Verdict: core.VerdictDownshift}
	c.Put(testKey("a"), want)
	got, ok := c.Get(testKey("a"))
	if !ok {
		t.Fatal("stored decision must hit")
	}
	if got.Verdict != want.Verdict {
		t.Errorf("Verdict = %s, want %s", got.Verdict, want.Verdict)
	}
}

func TestCache_ScopeIsTriple(t *testing.T) {
	c := routecache.New(16)
	c.Put(testKey("a"), core.Decision{})
	other := testKey("a")
	other.Harness = "cursor"
	if _, ok := c.Get(other); ok {
		t.Error("different harness must not hit")
	}
}

func TestCache_EvictsOldestAtBound(t *testing.T) {
	c := routecache.New(2)
	c.Put(testKey("a"), core.Decision{})
	c.Put(testKey("b"), core.Decision{})
	c.Put(testKey("c"), core.Decision{})
	if _, ok := c.Get(testKey("a")); ok {
		t.Error("oldest entry must be evicted at bound")
	}
	if c.Len() != 2 {
		t.Errorf("Len = %d, want 2", c.Len())
	}
}

func TestCache_NilIsDisabled(t *testing.T) {
	var c *routecache.Cache
	if _, ok := c.Get(testKey("a")); ok {
		t.Error("nil cache must miss")
	}
	c.Put(testKey("a"), core.Decision{})
	if c.Len() != 0 {
		t.Error("nil cache must discard writes")
	}
}
