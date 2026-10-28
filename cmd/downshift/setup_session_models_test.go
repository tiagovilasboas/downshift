// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/discovery"
)

func TestSetupEnsureSessionModels_SeedsMissingHarnesses(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", tempDir)

	var stdout, stderr bytes.Buffer
	opts := setupOptions{
		Out:  &stdout,
		Err:  &stderr,
		Home: tempDir,
	}

	setupEnsureSessionModels(opts, []setupTarget{{Name: "claude-code"}})

	p := filepath.Join(tempDir, "session-models.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("session-models.json was not created: %v", err)
	}

	var data map[string][]string
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("failed to parse session-models.json: %v", err)
	}

	claudeModels, ok := data["claude-code"]
	if !ok || len(claudeModels) != 3 {
		t.Fatalf("expected claude-code with 3 relative tiers, got %v", claudeModels)
	}

	if claudeModels[0] != "claude-haiku-5-5" {
		t.Errorf("expected small tier claude-haiku-5-5, got %q", claudeModels[0])
	}
	if claudeModels[1] != "claude-sonnet-5-5" {
		t.Errorf("expected mid tier claude-sonnet-5-5, got %q", claudeModels[1])
	}
	if claudeModels[2] != "claude-opus-5-5" {
		t.Errorf("expected frontier tier claude-opus-5-5, got %q", claudeModels[2])
	}
}

func TestSetupEnsureSessionModels_PreservesExistingUserConfig(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", tempDir)

	p := filepath.Join(tempDir, "session-models.json")
	initial := map[string][]string{
		"claude-code": {"custom-small", "custom-frontier"},
	}
	raw, _ := json.Marshal(initial)
	_ = os.WriteFile(p, raw, 0600)

	var stdout, stderr bytes.Buffer
	opts := setupOptions{
		Out:  &stdout,
		Err:  &stderr,
		Home: tempDir,
	}

	setupEnsureSessionModels(opts, []setupTarget{{Name: "claude-code"}})

	readBack, _ := os.ReadFile(p)
	var data map[string][]string
	_ = json.Unmarshal(readBack, &data)

	if data["claude-code"][0] != "custom-small" {
		t.Errorf("user custom model was overwritten: got %v", data["claude-code"])
	}
}

func TestSeedListPrefersDiscoveryCache(t *testing.T) {
	cache := discovery.File{Harnesses: map[string]discovery.Entry{
		"opencode": {Ranked: discovery.Ranked{IDs: []string{"opencode/beam-free", "opencode/big-pickle"}}},
	}}
	got, ok := seedList("opencode", cache)
	if !ok || len(got) != 2 || got[0] != "opencode/beam-free" {
		t.Fatalf("seedList(opencode) = %v, %v; want the discovered ids", got, ok)
	}
	if _, ok := seedList("opencode", discovery.File{}); ok {
		t.Error("opencode has no fixed fallback: without discovery it must not be seeded")
	}
	if got, ok := seedList("claude-code", discovery.File{}); !ok || len(got) != 3 {
		t.Errorf("claude-code without cache = %v, %v; want the fixed tiers", got, ok)
	}
}
