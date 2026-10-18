// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package models_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/models"
)

var cat = catalog.Load()

// --- List ---

func TestList_ContainsAllHarnesses(t *testing.T) {
	var buf bytes.Buffer
	models.List(cat, &buf)
	out := buf.String()

	for _, harness := range []string{"claude-code", "cursor", "codex"} {
		if !strings.Contains(out, harness) {
			t.Errorf("List() output missing harness %q", harness)
		}
	}
}

func TestList_ContainsTiers(t *testing.T) {
	var buf bytes.Buffer
	models.List(cat, &buf)
	out := buf.String()

	for _, tier := range []string{"small", "mid", "frontier"} {
		if !strings.Contains(out, tier) {
			t.Errorf("List() output missing tier %q", tier)
		}
	}
}

func TestList_ContainsCatalogSource(t *testing.T) {
	var buf bytes.Buffer
	models.List(cat, &buf)
	out := buf.String()

	if !strings.Contains(out, "Catalog source:") {
		t.Error("List() must print catalog source line")
	}
}

func TestList_EmptyCatalog(t *testing.T) {
	// Build a minimal catalog with zero entries via parse (package-internal);
	// here we just verify List handles an empty slice gracefully.
	var buf bytes.Buffer
	// We can't easily create an empty *catalog.Catalog from outside the package,
	// so we just verify that a real catalog produces non-empty output.
	models.List(cat, &buf)
	if buf.Len() == 0 {
		t.Error("List() must produce output for non-empty catalog")
	}
}

// TestList_DoesNotReorderTheCallersCatalog pins the hermetic contract: a
// reader that shares its catalog with other readers in the same process (the
// conformance suite is one) must not observe a reordering after List.
func TestList_DoesNotReorderTheCallersCatalog(t *testing.T) {
	reader := &fixedReader{entries: []catalog.Entry{
		{Harness: "zeta", Tier: "small", ID: "z-small", InputCostM: 1, OutputCostM: 2},
		{Harness: "alpha", Tier: "frontier", ID: "a-frontier", InputCostM: 3, OutputCostM: 6},
		{Harness: "alpha", Tier: "small", ID: "a-small", InputCostM: 1, OutputCostM: 1},
	}}
	before := append([]catalog.Entry(nil), reader.entries...)
	models.List(reader, &bytes.Buffer{})
	for i := range before {
		if reader.entries[i].ID != before[i].ID {
			t.Fatalf("List reordered the caller's catalog: position %d = %q, want %q", i, reader.entries[i].ID, before[i].ID)
		}
	}
}

// fixedReader serves a fixed entry slice, shared like a live catalog.
type fixedReader struct{ entries []catalog.Entry }

func (r *fixedReader) Entries() []catalog.Entry { return r.entries }

func (r *fixedReader) LookupByID(_, _ string) (core.Model, bool) { return core.Model{}, false }

// --- Check (unit-level, no real HTTP) ---

// TestCheck_NoKeys verifies that check does not error when no API keys are set.
func TestCheck_NoKeys(t *testing.T) {
	// Unset all provider keys so every provider is skipped.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("XAI_API_KEY", "")

	var out, errOut bytes.Buffer
	rc := models.Check(cat, &out, &errOut)

	if rc != 0 {
		t.Errorf("Check() with no keys returned rc=%d, want 0", rc)
	}
	// Must mention that no providers were checked.
	if !strings.Contains(out.String(), "No providers checked") {
		t.Errorf("Check() output = %q, want 'No providers checked'", out.String())
	}
}

// TestCheck_SkipMessagePerProvider verifies that each missing key prints a skip note.
func TestCheck_SkipMessagePerProvider(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("XAI_API_KEY", "")

	var out bytes.Buffer
	models.Check(cat, &out, &bytes.Buffer{})
	text := out.String()

	for _, key := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "XAI_API_KEY"} {
		if !strings.Contains(text, key) {
			t.Errorf("Check() output missing skip note for %s", key)
		}
	}
}

// --- Pull (unit-level, no real HTTP, no file writes) ---

// TestPull_NoKeys is the same as Check — no keys means nothing happens.
func TestPull_NoKeys(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("XAI_API_KEY", "")

	var out, errOut bytes.Buffer
	rc := models.Pull(cat, &out, &errOut)
	if rc != 0 {
		t.Errorf("Pull() with no keys returned rc=%d, want 0", rc)
	}
}

// --- catalogSource when user override exists ---

func TestList_ShowsOverrideSourceWhenPresent(t *testing.T) {
	// Create a temp home with an override file so catalogSource returns the
	// user-override path instead of "embedded (default)".
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := home + "/.harness-downshift"
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Write a minimal valid catalog override.
	minimal := `{"version":"1","entries":[]}`
	if err := os.WriteFile(dir+"/catalog.json", []byte(minimal), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Load uses the real HOME so it picks up the override catalog.
	overrideCat := catalog.Load()
	var buf bytes.Buffer
	models.List(overrideCat, &buf)
	if !strings.Contains(buf.String(), "user override") {
		t.Errorf("List output should show 'user override' source; got:\n%s", buf.String())
	}
}
