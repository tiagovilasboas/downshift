// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsConfigured_FalseOnEmpty(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("DOWNSHIFT_STATE_DIR", filepath.Join(tempHome, ".downshift"))

	if isConfigured(tempHome) {
		t.Errorf("isConfigured(%s) = true; want false for clean empty home", tempHome)
	}
}

func TestIsConfigured_TrueWhenHookExists(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("DOWNSHIFT_STATE_DIR", filepath.Join(tempHome, ".downshift"))

	// Create a dummy hook file for claude-code
	claudeDir := filepath.Join(tempHome, ".claude")
	_ = os.MkdirAll(claudeDir, 0o755)
	_ = os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"hooks":{"PreToolUse":[{"command":"downshift claude-code"}]}}`), 0o644)

	if !isConfigured(tempHome) {
		t.Errorf("isConfigured(%s) = false; want true when downshift hook is present in .claude/settings.json", tempHome)
	}
}

func TestRunDefaultEntry_ConfiguredOutputsDashboard(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("DOWNSHIFT_STATE_DIR", filepath.Join(tempHome, ".downshift"))

	// Mark as configured by creating a hook
	claudeDir := filepath.Join(tempHome, ".claude")
	_ = os.MkdirAll(claudeDir, 0o755)
	_ = os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"hooks":[{"command":"downshift claude-code"}]}`), 0o644)

	var stdout, stderr bytes.Buffer
	rc := runDefaultEntry(strings.NewReader(""), &stdout, &stderr, cmdCat)
	if rc != 0 {
		t.Fatalf("runDefaultEntry() = %d, want 0", rc)
	}
	out := stdout.String()
	if !strings.Contains(out, "Downshift") {
		t.Errorf("runDefaultEntry() output = %q; want header containing Downshift", out)
	}
}
