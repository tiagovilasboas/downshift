// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

func TestEffortValue_KnownScale(t *testing.T) {
	c := Load()
	// codex uses reasoning_effort; low should map to "low"
	entry, ok := c.EntryFor("codex", c.ModelFor("codex", core.TierSmall).ID)
	if !ok {
		t.Fatal("codex small entry not found")
	}
	v := EffortValue(entry, core.EffortLow)
	if v == "" {
		t.Error("EffortValue returned empty string")
	}
}

// --- Catalog.EffortFor (method) ---

func TestCatalogEffortFor_KnownModel(t *testing.T) {
	c := Load()
	// gpt-5.6-sol is the codex frontier; its effort_map maps "low" → "low"
	frontierID := c.ModelFor("codex", core.TierFrontier).ID
	got := c.EffortFor("codex", frontierID, core.EffortLow)
	if got == "" {
		t.Error("EffortFor must return a non-empty effort string for a known model")
	}
}

func TestCatalogEffortFor_UnknownModel_Fallback(t *testing.T) {
	c := Load()
	// Unknown model → falls back to effort.String()
	got := c.EffortFor("codex", "completely-unknown-xyz", core.EffortHigh)
	if got != "high" {
		t.Errorf("EffortFor unknown model = %q, want 'high' (effort.String() fallback)", got)
	}
}

// --- EffortValue: entry with no effort_map key for the requested level ---

func TestEffortValue_MissingKey_FallsBackToEffortString(t *testing.T) {
	// An entry with an empty effort_map has no keys at all.
	e := Entry{EffortMap: map[string]string{}}
	got := EffortValue(e, core.EffortMid)
	if got != "medium" {
		t.Errorf("EffortValue with empty map = %q, want 'medium' (effort.String())", got)
	}
}

func TestEffortValue_NilEffortMap_FallsBackToEffortString(t *testing.T) {
	e := Entry{} // EffortMap is nil
	got := EffortValue(e, core.EffortHigh)
	if got != "high" {
		t.Errorf("EffortValue with nil map = %q, want 'high'", got)
	}
}

// --- Option A: family + effort routing ---

func TestFamilyModelFor_OpusHighEffort(t *testing.T) {
	c := Load()
	m, ok := c.FamilyModelFor("cursor", "claude-opus", core.EffortHigh)
	if !ok {
		t.Fatal("FamilyModelFor(cursor, claude-opus, high) should hit")
	}
	if m.ID != "claude-opus-5-thinking-high" {
		t.Errorf("got %q, want claude-opus-5-thinking-high", m.ID)
	}
}

func TestFamilyModelFor_OpusMediumEffort(t *testing.T) {
	c := Load()
	m, ok := c.FamilyModelFor("cursor", "claude-opus", core.EffortMid)
	if !ok {
		t.Fatal("FamilyModelFor(cursor, claude-opus, medium) should hit the 5.5 variant")
	}
	if m.ID != "claude-opus-5-5-medium" {
		t.Errorf("got %q, want claude-opus-5-5-medium", m.ID)
	}
}

func TestFamilyModelFor_GeminiHigh(t *testing.T) {
	c := Load()
	m, ok := c.FamilyModelFor("cursor", "gemini", core.EffortHigh)
	if !ok {
		t.Fatal("FamilyModelFor(cursor, gemini, high) should hit")
	}
	if m.ID != "gemini-3.8-flash-high" {
		t.Errorf("got %q, want gemini-3.8-flash-high", m.ID)
	}
	if m.Tier != core.TierSmall {
		t.Errorf("gemini tier = %s, want small", m.Tier)
	}
}

func TestFamilyModelFor_UnknownFamilyReturnsFalse(t *testing.T) {
	c := Load()
	if _, ok := c.FamilyModelFor("cursor", "no-such-family", core.EffortLow); ok {
		t.Error("unknown family should return false (caller falls back to ModelFor)")
	}
}

func TestFamilyModelFor_FamilyWithoutEffortVariantReturnsFalse(t *testing.T) {
	c := Load()
	// gemini only has a high variant — low must miss, not silently substitute.
	if _, ok := c.FamilyModelFor("cursor", "gemini", core.EffortLow); ok {
		t.Error("gemini+low should return false")
	}
}

func TestFamilyModelFor_ExplicitOnlyNeverReturned(t *testing.T) {
	c := Load()
	// claude-fable is explicit_only: valid when selected, never an auto target.
	if _, ok := c.FamilyModelFor("cursor", "claude-fable", core.EffortHigh); ok {
		t.Error("explicit_only fable must never be returned by FamilyModelFor")
	}
}

func TestFamilyModelFor_CaseInsensitive(t *testing.T) {
	c := Load()
	m, ok := c.FamilyModelFor("cursor", "Claude-Opus", core.EffortHigh)
	if !ok {
		t.Fatal("family lookup should be case-insensitive")
	}
	if m.ID != "claude-opus-5-thinking-high" {
		t.Errorf("got %q, want claude-opus-5-thinking-high", m.ID)
	}
}

func TestFamilyModelFor_EmptyFamilyReturnsFalse(t *testing.T) {
	c := Load()
	if _, ok := c.FamilyModelFor("cursor", "", core.EffortHigh); ok {
		t.Error("empty family should return false")
	}
}

func TestValidate_AllowsSameFamilyDifferentEffort(t *testing.T) {
	data := []byte(`{"entries":[
		{"id":"opus-high","family":"claude-opus","harness":"cursor","tier":"frontier","effort":"high"},
		{"id":"opus-medium","family":"claude-opus","harness":"cursor","tier":"frontier","effort":"medium"}
	]}`)
	if _, err := parse(data); err != nil {
		t.Fatalf("same family with differing efforts should validate: %v", err)
	}
}

func TestValidate_RejectsSameFamilySameEffort(t *testing.T) {
	data := []byte(`{"entries":[
		{"id":"opus-high-a","family":"claude-opus","harness":"cursor","tier":"frontier","effort":"high"},
		{"id":"opus-high-b","family":"claude-opus","harness":"cursor","tier":"frontier","effort":"high"}
	]}`)
	if _, err := parse(data); err == nil {
		t.Fatal("same family with same effort must stay a validation error")
	}
}

func TestValidate_RejectsOverlapWhenOneEntryUntagged(t *testing.T) {
	data := []byte(`{"entries":[
		{"id":"grok-base","family":"grok","harness":"cursor","tier":"frontier"},
		{"id":"grok-fast","family":"grok-4.7","harness":"cursor","tier":"frontier","effort":"high"}
	]}`)
	if _, err := parse(data); err == nil {
		t.Fatal("overlapping families with an untagged entry must stay a validation error")
	}
}

// native_name is a family-level fact: a replaced or newly added version of the
// same family inherits it, so overrides and `models pull` never lose it.
func TestMergeEntries_InheritsNativeNameByFamily(t *testing.T) {
	base := []Entry{{ID: "claude-haiku-4-5", Family: "claude-haiku", NativeName: "haiku", Harness: "claude-code", Tier: "small"}}
	override := []Entry{
		{ID: "claude-haiku-4-5", Family: "claude-haiku", Harness: "claude-code", Tier: "small"},
		{ID: "claude-haiku-9", Family: "claude-haiku", Harness: "claude-code", Tier: "small"},
		{ID: "other-1", Family: "other", Harness: "claude-code", Tier: "small"},
	}
	for _, e := range mergeEntries(base, override) {
		want := "haiku"
		if e.Family == "other" {
			want = ""
		}
		if e.NativeName != want {
			t.Errorf("%s native_name = %q, want %q", e.ID, e.NativeName, want)
		}
	}
}

// The harness echoes its native name back in tool input, so it must resolve to
// the same model as the id and count as an exact identity (not a foreign model).
func TestNativeNameResolvesLikeAnAlias(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	c := Load()
	for _, tier := range []core.Tier{core.TierSmall, core.TierMid, core.TierFrontier} {
		want := c.ModelFor("claude-code", tier)
		if want.Native == "" {
			t.Fatalf("tier %v has no native name in the embedded catalog", tier)
		}
		got, ok := c.LookupByID("claude-code", want.Native)
		if !ok || got.ID != want.ID {
			t.Errorf("LookupByID(%q) = %q ok=%v, want %q", want.Native, got.ID, ok, want.ID)
		}
		if !c.IsExactID("claude-code", want.Native) {
			t.Errorf("IsExactID(%q) = false", want.Native)
		}
	}
}

func TestAllowedEfforts_VerifiedOnlyAndExact(t *testing.T) {
	c := Load()
	if got := c.AllowedEfforts("claude-code", "claude-opus-5-5"); len(got) != 5 {
		t.Errorf("claude-opus-5-5 efforts = %v, want five levels", got)
	}
	if got := c.AllowedEfforts("claude-code", "claude-opus-9-9"); got != nil {
		t.Errorf("family-prefix match must not inherit a list, got %v", got)
	}
	for _, id := range []string{"gpt-6-luna", "gpt-6-sol", "gpt-6-astra", "gpt-5.6-sol"} {
		if got := c.AllowedEfforts("codex", id); got != nil {
			t.Errorf("%s has unverified efforts, want nil, got %v", id, got)
		}
	}
	unknown := strings.Join(c.EffortUnknownModels(), " ")
	if !strings.Contains(unknown, "codex/gpt-6-sol") || strings.Contains(unknown, "claude-code/claude-opus-5-5") {
		t.Errorf("EffortUnknownModels = %s", unknown)
	}
}
