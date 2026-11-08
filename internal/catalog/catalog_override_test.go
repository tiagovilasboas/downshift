// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"bytes"
	"encoding/json"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeEntries_PreservesDefaultsAndOverridesTarget(t *testing.T) {
	base := []Entry{{ID: "luna", Harness: "codex", Tier: "small"}, {ID: "terra", Harness: "codex", Tier: "mid"}}
	override := []Entry{{ID: "terra", Harness: "codex", Tier: "frontier"}, {ID: "custom", Harness: "codex", Tier: "frontier"}}
	merged := mergeEntries(base, override)
	encoded, err := json.Marshal(merged)
	if err != nil {
		t.Fatalf("marshal merged: %v", err)
	}
	if string(encoded) == "" || len(merged) != 3 || merged[0].ID != "luna" || merged[1].Tier != "frontier" {
		t.Fatalf("merge did not preserve and override entries: %#v", merged)
	}
}

func TestMergeEntries_KeepsRoutingFlagsWhenOverrideLeavesThemUnset(t *testing.T) {
	base := []Entry{{ID: "m", Harness: "h", Tier: "small", PoolGated: true, RejectedName: "m-alias"}}
	merged := mergeEntries(base, []Entry{{ID: "m", Harness: "h", Tier: "mid"}})
	if len(merged) != 1 || merged[0].Tier != "mid" || !merged[0].PoolGated || merged[0].RejectedName != "m-alias" {
		t.Fatalf("override dropped the routing flags: %#v", merged)
	}
	merged = mergeEntries(base, []Entry{{ID: "m", Harness: "h", Tier: "mid", RejectedName: "other"}})
	if merged[0].RejectedName != "other" {
		t.Fatalf("explicit override value lost: %#v", merged)
	}
}

func TestLoad_WarnsAndFallsBackWhenOverrideCannotBeRead(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	dir := filepath.Join(home, ".harness-downshift", "catalog.json")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll override directory: %v", err)
	}

	var warnings bytes.Buffer
	c := load(&warnings)
	if c == nil || len(c.Entries()) == 0 {
		t.Fatal("load() did not fall back to the embedded catalog")
	}
	if !strings.Contains(warnings.String(), "could not read") {
		t.Errorf("warnings = %q, want override read warning", warnings.String())
	}
}

// --- load: path where override parses but fails validateEntries (could not merge) ---

func TestLoad_WarnsAndFallsBackWhenMergeFails(t *testing.T) {
	// Write a catalog where an override entry adds a new model whose family
	// prefix overlaps with an existing family in the embedded base, causing
	// validateEntries(merged) to fail with "overlapping model families".
	// The base has "claude-haiku" as a family for claude-code.
	// If we add a new entry with family "claude" (a prefix of "claude-haiku"),
	// the merged validation rejects it.
	home := t.TempDir()
	testenv.SetHome(t, home)
	dir := filepath.Join(home, ".harness-downshift")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// A new ID that won't collide in the override alone, but whose family "claude"
	// is a prefix of "claude-haiku" in the base → merged validation fails.
	overlap := `{"version":"1","entries":[
		{"id":"claude-new-model","family":"claude","harness":"claude-code","tier":"small"}
	]}`
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(overlap), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	var warnings bytes.Buffer
	c := load(&warnings)
	if c == nil || len(c.Entries()) == 0 {
		t.Fatal("load() must fall back to embedded catalog on merge failure")
	}
	w := warnings.String()
	// Either "could not merge" (from validateEntries) or "could not parse" (if
	// decode rejects the overlap earlier). Either path means fall-back worked.
	if !strings.Contains(w, "could not merge") && !strings.Contains(w, "could not parse") {
		t.Errorf("warnings = %q, want warning about merge or parse failure", w)
	}
}
