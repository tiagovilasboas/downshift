// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
