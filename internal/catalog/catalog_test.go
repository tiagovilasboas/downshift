// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

func TestLoad_ReturnsEmbedded(t *testing.T) {
	c := Load()
	if c == nil {
		t.Fatal("Load() returned nil")
	}
}

func TestCodexGPT6ModelsAreRoutable(t *testing.T) {
	c := Load()
	for _, tc := range []struct {
		id       string
		wantTier core.Tier
	}{
		{id: "gpt-6-luna", wantTier: core.TierSmall},
		{id: "gpt-6-sol", wantTier: core.TierFrontier},
		{id: "gpt-6.1-sol", wantTier: core.TierFrontier},
	} {
		m, ok := c.LookupByID("codex", tc.id)
		if !ok {
			t.Fatalf("LookupByID(codex, %q) not found", tc.id)
		}
		if m.Tier != tc.wantTier {
			t.Errorf("%s = tier %s; want tier %s", tc.id, m.Tier, tc.wantTier)
		}
	}
}

func TestModelFor_KnownHarness(t *testing.T) {
	c := Load()
	harnesses := []string{"claude-code", "cursor", "codex"}
	tiers := []core.Tier{core.TierSmall, core.TierMid, core.TierFrontier}

	for _, h := range harnesses {
		for _, tier := range tiers {
			m := c.ModelFor(h, tier)
			if m.ID == "" {
				t.Errorf("ModelFor(%q, %s) returned empty ID", h, tier)
			}
			if m.Tier != tier {
				t.Errorf("ModelFor(%q, %s): model tier = %s, want %s", h, tier, m.Tier, tier)
			}
		}
	}
}

func TestModelFor_UnknownHarness_FallsBack(t *testing.T) {
	c := Load()
	m := c.ModelFor("unknown-harness", core.TierSmall)
	if m.ID == "" {
		t.Error("ModelFor(unknown) must return a fallback, not empty ID")
	}
}

func TestModelFor_KnownIncompleteHarnessDoesNotBorrowClaudeModel(t *testing.T) {
	c, err := parse([]byte(`{
		"entries": [
			{"id":"claude-haiku","harness":"claude-code","tier":"small"},
			{"id":"codex-terra","harness":"codex","tier":"mid"}
		]
	}`))
	if err != nil {
		t.Fatalf("parse catalog: %v", err)
	}

	m := c.ModelFor("codex", core.TierSmall)
	if m.ID != "" {
		t.Fatalf("known incomplete harness returned model %q; must not borrow another harness model", m.ID)
	}
	if m.Harness != "codex" {
		t.Errorf("zero model harness = %q, want codex", m.Harness)
	}
	if m.Tier != core.TierSmall {
		t.Errorf("zero model tier = %s, want small", m.Tier)
	}
}

func TestModelFor_DeclaredHarnessWithoutRoutableModelsDoesNotFallback(t *testing.T) {
	c, err := parse([]byte(`{
		"entries": [
			{"id":"claude-haiku","harness":"claude-code","tier":"small"},
			{"id":"pending-model","harness":"codex","tier":"unknown"}
		]
	}`))
	if err != nil {
		t.Fatalf("parse catalog: %v", err)
	}

	m := c.ModelFor("codex", core.TierSmall)
	if m.ID != "" {
		t.Fatalf("declared harness without routable tiers returned model %q; must not fallback", m.ID)
	}
	if m.Harness != "codex" {
		t.Errorf("zero model harness = %q, want codex", m.Harness)
	}
}

func TestClaudeCodeFableIsOptInNotTheDefaultFrontier(t *testing.T) {
	c := Load()
	m, ok := c.LookupByID("claude-code", "fable")
	if !ok {
		t.Fatal("native name fable must resolve on claude-code")
	}
	if m.ID != "claude-fable-5-1" || m.Native != "fable" {
		t.Fatalf("fable lookup = %+v, want claude-fable-5-1 native fable", m)
	}
	if c.IsExplicitOnly("claude-code", "claude-fable-5-1") {
		t.Error("claude-code fable is an optional session upshift, not explicit_only")
	}
	frontier := c.ModelFor("claude-code", core.TierFrontier)
	if frontier.ID != "claude-opus-5-5" {
		t.Errorf("default frontier = %q, want claude-opus-5-5 (fable stays opt-in)", frontier.ID)
	}
}

func TestEmbeddedCatalogNeverEmitsNonexistentHaikuID(t *testing.T) {
	c, err := parse(embeddedJSON)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range c.Entries() {
		if e.ID == "claude-haiku-4" {
			t.Errorf("harness %q emits nonexistent model id %q; use claude-haiku-4-5", e.Harness, e.ID)
		}
	}
	if small := c.ModelFor("claude-code", core.TierSmall); small.ID != "claude-haiku-5-5" {
		t.Errorf("claude-code small = %q, want claude-haiku-5-5 (the newest haiku)", small.ID)
	}
}

// The native 'haiku' alias runs the newest haiku (Haiku 5.5, observed
// 2026-10-28; it ran Haiku 4.5 on 2026-10-15). Haiku 5.5 is the small target
// and owns the 'claude-haiku' family; Haiku 4.5 keeps its own exact id and
// price and is not written by name.
func TestEmbeddedClaudeCodeHaiku55IsTheNativeTarget(t *testing.T) {
	c, err := parse(embeddedJSON)
	if err != nil {
		t.Fatal(err)
	}
	small := c.ModelFor("claude-code", core.TierSmall)
	if small.Native != "haiku" || small.ID != "claude-haiku-5-5" {
		t.Fatalf("claude-code small = %+v, want native haiku on claude-haiku-5-5", small)
	}
	m45, ok := c.LookupByID("claude-code", "claude-haiku-4-5")
	if !ok || m45.ID != "claude-haiku-4-5" || m45.Native != "" {
		t.Errorf("claude-haiku-4-5 = %+v (ok %v), want its own entry with no native name", m45, ok)
	}
	for _, id := range []string{"claude-haiku", "claude-haiku-5-5"} {
		if m, ok := c.LookupByID("claude-code", id); !ok || m.ID != "claude-haiku-5-5" {
			t.Errorf("LookupByID(%q) = %s, want claude-haiku-5-5", id, m.ID)
		}
	}
}

// --- explicit_only routing policy ---

func TestIsExplicitOnly_AstraIsExplicitOnly(t *testing.T) {
	c := Load()
	if !c.IsExplicitOnly("codex", "gpt-6-astra") {
		t.Error("gpt-6-astra should be explicit_only")
	}
}

func TestIsExplicitOnly_NormalModelIsNotExplicit(t *testing.T) {
	c := Load()
	frontier := c.ModelFor("codex", core.TierFrontier)
	if c.IsExplicitOnly("codex", frontier.ID) {
		t.Errorf("%s should not be explicit_only", frontier.ID)
	}
}

func TestIsExplicitOnly_EmptyIDReturnsFalse(t *testing.T) {
	c := Load()
	if c.IsExplicitOnly("codex", "") {
		t.Error("empty model ID should return false")
	}
}

func TestIsExplicitOnly_UnknownIDReturnsFalse(t *testing.T) {
	c := Load()
	if c.IsExplicitOnly("codex", "completely-unknown-model") {
		t.Error("unknown model should return false (fail-open)")
	}
}

func TestAstraNotChosenAsAutomaticTarget(t *testing.T) {
	c := Load()
	// gpt-6-astra is explicit_only — ModelFor must NEVER return it as the
	// automatic target for any tier, even if it is the only frontier model
	// registered for that harness.
	for _, tier := range []core.Tier{core.TierSmall, core.TierMid, core.TierFrontier} {
		m := c.ModelFor("codex", tier)
		if m.ID == "gpt-6-astra" {
			t.Errorf("ModelFor(codex, %s) returned explicit_only model gpt-6-astra", tier)
		}
	}
}

func TestEntries_NotEmpty(t *testing.T) {
	c := Load()
	if len(c.Entries()) == 0 {
		t.Error("Entries() returned empty slice")
	}
}
