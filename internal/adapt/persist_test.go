// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package adapt

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Memory is advice, never a requirement: every unreadable state loads as
// "no memory" or an error the caller treats as fail-open.
func TestLoadMissingEmptyAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	if mem, err := Load(filepath.Join(dir, "absent.json")); mem != nil || err != nil {
		t.Errorf("missing file = %v, %v; want nil, nil", mem, err)
	}
	empty := filepath.Join(dir, "empty.json")
	writeFile(t, empty, "")
	if mem, err := Load(empty); mem != nil || err != nil {
		t.Errorf("empty file = %v, %v; want nil, nil", mem, err)
	}
	noShapes := filepath.Join(dir, "noshapes.json")
	writeFile(t, noShapes, `{"version":1,"shapes":{}}`)
	if mem, err := Load(noShapes); mem != nil || err != nil {
		t.Errorf("no shapes = %v, %v; want nil, nil", mem, err)
	}
	bad := filepath.Join(dir, "bad.json")
	writeFile(t, bad, `{"version":`)
	if mem, err := Load(bad); mem != nil || err == nil {
		t.Errorf("corrupt file = %v, %v; want nil and an error", mem, err)
	}
	if _, err := Load(dir); err == nil {
		t.Error("a directory must be an error, not silently empty")
	}
}

func TestLoadDefaultsTheVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.json")
	writeFile(t, path, `{"shapes":{"s":{"hits":["small"]}}}`)
	mem, err := Load(path)
	if err != nil || mem == nil {
		t.Fatalf("Load = %v, %v", mem, err)
	}
	if mem.Version != fileVersion {
		t.Errorf("version = %d, want %d", mem.Version, fileVersion)
	}
}

func TestSaveThenLoadRoundTripsAndKeepsPrivateMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state", "adapt-memory.json")
	mem := BuildMemory([]FeedbackEvent{
		{Shape: "trivial:rename", SelectedTier: TierSmall, Success: true},
		{Shape: "trivial:rename", SelectedTier: TierMid, Failed: true},
	})
	if err := Save(path, mem); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Error("temporary file left behind after the atomic rename")
	}
	got, err := Load(path)
	if err != nil || got == nil {
		t.Fatalf("Load = %v, %v", got, err)
	}
	// The saved memory must steer exactly like the in-memory one.
	if a, b := AdjustTier(TierMid, "trivial:rename", &mem), AdjustTier(TierMid, "trivial:rename", got); a != b {
		t.Errorf("AdjustTier differs after a round trip: %v vs %v", a, b)
	}
}

func TestSaveFillsDefaultsAndFailsWhereItCannotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.json")
	if err := Save(path, Memory{}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != `{"version":1,"shapes":{}}` {
		t.Errorf("zero memory saved as %s", raw)
	}
	// A regular file where the directory should be: Save reports it.
	blocker := filepath.Join(dir, "blocker")
	writeFile(t, blocker, "x")
	if err := Save(filepath.Join(blocker, "m.json"), Memory{}); err == nil {
		t.Error("Save under a file must fail")
	}
}

func TestDefaultPathFollowsTheStateDir(t *testing.T) {
	state := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", state)
	p, err := DefaultMemoryPath()
	if err != nil || p != filepath.Join(state, "adapt-memory.json") {
		t.Fatalf("DefaultMemoryPath = %q, %v", p, err)
	}
	if mem, err := LoadDefault(); mem != nil || err != nil {
		t.Fatalf("LoadDefault on a fresh dir = %v, %v", mem, err)
	}
	mem := BuildMemory([]FeedbackEvent{{Shape: "s", SelectedTier: TierSmall, Success: true}})
	if err := SaveDefault(mem); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDefault()
	if err != nil || got == nil || len(got.Shapes) != 1 {
		t.Fatalf("LoadDefault = %v, %v", got, err)
	}
}

func TestBuildMemoryFreezesOnlyCleanShapes(t *testing.T) {
	mem := BuildMemory([]FeedbackEvent{
		{Shape: "clean", SelectedTier: TierSmall, Success: true},
		{Shape: "mixed", SelectedTier: TierSmall, Success: true},
		{Shape: "mixed", SelectedTier: TierSmall, Retry: true},
		{Shape: "", SelectedTier: TierSmall, Success: true}, // no shape: ignored
		{Shape: "unknown", SelectedTier: TierSmall},         // unreviewed outcome: ignored
	})
	if len(mem.Shapes) != 2 {
		t.Fatalf("shapes = %v, want clean and mixed only", mem.Shapes)
	}
	if !mem.Shapes["clean"].frozen() {
		t.Error("a shape with a hit and no miss is frozen")
	}
	if mem.Shapes["mixed"].frozen() {
		t.Error("a shape with a miss must not be frozen")
	}
	// Duplicates of the same tier are kept once.
	again := BuildMemory([]FeedbackEvent{
		{Shape: "s", SelectedTier: TierSmall, Success: true},
		{Shape: "s", SelectedTier: TierSmall, Success: true},
	})
	if n := len(again.Shapes["s"].Hits); n != 1 {
		t.Errorf("hits = %d, want 1", n)
	}
}
