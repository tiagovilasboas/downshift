// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

func TestLookupByID_Found(t *testing.T) {
	c := Load()
	// The small claude-code model must be findable by ID.
	small := c.ModelFor("claude-code", core.TierSmall)
	m, ok := c.LookupByID("claude-code", small.ID)
	if !ok {
		t.Fatalf("LookupByID(%q) = not found", small.ID)
	}
	if m.ID != small.ID {
		t.Errorf("LookupByID returned %q, want %q", m.ID, small.ID)
	}
}

func TestLookupByID_NotFound(t *testing.T) {
	c := Load()
	_, ok := c.LookupByID("claude-code", "nonexistent-xyz-12345")
	if ok {
		t.Error("LookupByID(nonexistent) should return false")
	}
}

func TestLookupByID_EmptyID(t *testing.T) {
	c := Load()
	_, ok := c.LookupByID("claude-code", "")
	if ok {
		t.Error("LookupByID('') should return false")
	}
}

// --- Family prefix (version-agnostic) matching ---

func TestLookupByID_FamilyMatchVersionBump(t *testing.T) {
	c := Load()
	// claude-opus-4-9 doesn't exist in the catalog, but starts with family
	// "claude-opus" → should match the frontier tier entry.
	m, ok := c.LookupByID("claude-code", "claude-opus-4-9")
	if !ok {
		t.Fatal("version-bumped opus ID should match via family prefix")
	}
	if m.Tier != core.TierFrontier {
		t.Errorf("family match tier = %s, want frontier", m.Tier)
	}
}

func TestLookupByID_FamilyMatchHaiku(t *testing.T) {
	c := Load()
	m, ok := c.LookupByID("claude-code", "claude-haiku-5")
	if !ok {
		t.Fatal("claude-haiku-5 should match via family 'claude-haiku'")
	}
	if m.Tier != core.TierSmall {
		t.Errorf("family match tier = %s, want small", m.Tier)
	}
}

func TestLookupByID_FamilyMatchCodexFrontier(t *testing.T) {
	c := Load()
	// gpt-5.6-sol-v2 starts with family "gpt-5.6-sol" → frontier
	m, ok := c.LookupByID("codex", "gpt-5.6-sol-v2")
	if !ok {
		t.Fatal("gpt-5.6-sol-v2 should match via family")
	}
	if m.Tier != core.TierFrontier {
		t.Errorf("family match tier = %s, want frontier", m.Tier)
	}
}

func TestLookupByID_FamilyMatchCaseInsensitive(t *testing.T) {
	c := Load()
	m, ok := c.LookupByID("claude-code", "Claude-Opus-5")
	if !ok {
		t.Fatal("family match should be case-insensitive")
	}
	if m.Tier != core.TierFrontier {
		t.Errorf("tier = %s, want frontier", m.Tier)
	}
}

func TestLookupByID_ExactWinsOverFamily(t *testing.T) {
	c := Load()
	// Exact ID match must take precedence over family prefix.
	exact := c.ModelFor("claude-code", core.TierMid)
	m, ok := c.LookupByID("claude-code", exact.ID)
	if !ok {
		t.Fatal("exact ID not found")
	}
	if m.ID != exact.ID {
		t.Errorf("exact match returned wrong ID %q, want %q", m.ID, exact.ID)
	}
}

// --- OpenRouter "provider/model-id" normalisation ---

func TestLookupByID_OpenRouterExactMatch(t *testing.T) {
	c := Load()
	// "anthropic/claude-opus-5-5" → strip prefix → exact match on "claude-opus-5-5"
	m, ok := c.LookupByID("claude-code", "anthropic/claude-opus-5-5")
	if !ok {
		t.Fatal("OpenRouter-prefixed exact ID should match after normalisation")
	}
	if m.Tier != core.TierFrontier {
		t.Errorf("tier = %s, want frontier", m.Tier)
	}
}

func TestLookupByID_OpenRouterFamilyMatch(t *testing.T) {
	c := Load()
	// "anthropic/claude-opus-4-9" → strip → "claude-opus-4-9" → family "claude-opus"
	m, ok := c.LookupByID("claude-code", "anthropic/claude-opus-4-9")
	if !ok {
		t.Fatal("OpenRouter-prefixed version-bumped ID should match via family")
	}
	if m.Tier != core.TierFrontier {
		t.Errorf("tier = %s, want frontier", m.Tier)
	}
}

func TestLookupByID_OpenRouterHaikuSmall(t *testing.T) {
	c := Load()
	m, ok := c.LookupByID("claude-code", "anthropic/claude-haiku-5")
	if !ok {
		t.Fatal("anthropic/claude-haiku-5 should match via family")
	}
	if m.Tier != core.TierSmall {
		t.Errorf("tier = %s, want small", m.Tier)
	}
}

func TestLookupByID_OpenRouterCodex(t *testing.T) {
	c := Load()
	// "openai/gpt-5.6-sol" → strip → exact match
	m, ok := c.LookupByID("codex", "openai/gpt-5.6-sol")
	if !ok {
		t.Fatal("openai/gpt-5.6-sol should match after normalisation")
	}
	if m.Tier != core.TierFrontier {
		t.Errorf("tier = %s, want frontier", m.Tier)
	}
}

func TestLookupByID_OpenRouterUnknownProvider(t *testing.T) {
	c := Load()
	// "somevendor/claude-haiku-5" → strip → "claude-haiku-5" → family match
	m, ok := c.LookupByID("claude-code", "somevendor/claude-haiku-5")
	if !ok {
		t.Fatal("any-provider/claude-haiku-5 should match via family after normalisation")
	}
	if m.Tier != core.TierSmall {
		t.Errorf("tier = %s, want small", m.Tier)
	}
}

func TestLookupByID_OpenRouterTrulyUnknown(t *testing.T) {
	c := Load()
	_, ok := c.LookupByID("claude-code", "somevendor/total-unknown-xyz-12345")
	if ok {
		t.Error("genuinely unknown model should return false even with provider prefix")
	}
}

// --- Grok installed in non-native harness ---

func TestLookupByID_GrokInCursor(t *testing.T) {
	c := Load()
	m, ok := c.LookupByID("cursor", "grok-4.7-high-fast")
	if !ok {
		t.Fatal("grok-4.7-high-fast should be found in cursor catalog")
	}
	if m.Tier != core.TierFrontier {
		t.Errorf("grok in cursor tier = %s, want frontier", m.Tier)
	}
}

func TestLookupByID_GrokVersionBumpInCursor(t *testing.T) {
	c := Load()
	// grok-4.7 is not an exact id → family "grok" → frontier
	m, ok := c.LookupByID("cursor", "grok-4.7")
	if !ok {
		t.Fatal("grok-4.7 should match via family 'grok' in cursor")
	}
	if m.Tier != core.TierFrontier {
		t.Errorf("tier = %s, want frontier", m.Tier)
	}
}

func TestLookupByID_GrokInClaudeCode(t *testing.T) {
	c := Load()
	m, ok := c.LookupByID("claude-code", "grok-4.6")
	if !ok {
		t.Fatal("grok-4.6 should be found in claude-code catalog")
	}
	if m.Tier != core.TierFrontier {
		t.Errorf("grok in claude-code tier = %s, want frontier", m.Tier)
	}
}

func TestLookupByID_ReturnsFamily(t *testing.T) {
	c := Load()
	m, ok := c.LookupByID("cursor", "grok-4.7-high")
	if !ok {
		t.Fatal("grok-4.7-high should be found")
	}
	if m.Family != "grok" {
		t.Errorf("Family = %q, want grok", m.Family)
	}
	// The Fast slug is its own exact-id row (twice the rate), with no family.
	if fast, ok := c.LookupByID("cursor", "grok-4.7-high-fast"); !ok || fast.Family != "" || fast.ID != "grok-4.7-high-fast" {
		t.Errorf("grok-4.7-high-fast = %q family %q (ok=%v), want its own row without a family", fast.ID, fast.Family, ok)
	}
}

func TestLookupByID_FamilyFallbackCarriesFamily(t *testing.T) {
	c := Load()
	// Version-bumped ID: family prefix match must still report the family so
	// adapters can do in-family effort routing on unknown versions.
	m, ok := c.LookupByID("cursor", "gemini-3.9-flash-high")
	if !ok {
		t.Fatal("gemini-3.9-flash-high should match via family 'gemini'")
	}
	if m.Family != "gemini" {
		t.Errorf("Family = %q, want gemini", m.Family)
	}
}
