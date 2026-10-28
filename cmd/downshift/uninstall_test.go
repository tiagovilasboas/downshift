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

func TestUninstall_HelpFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runUninstall([]string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runUninstall(--help) = %d, want 0", rc)
	}
	if !strings.Contains(stdout.String(), "usage: downshift uninstall") {
		t.Errorf("expected usage message, got %q", stdout.String())
	}
}

func TestUninstall_RestoresBackupWhenPresent(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	stateDir := filepath.Join(tempHome, ".downshift")
	t.Setenv("DOWNSHIFT_STATE_DIR", stateDir)
	_ = os.MkdirAll(filepath.Join(stateDir, "bin"), 0o755)

	claudeDir := filepath.Join(tempHome, ".claude")
	_ = os.MkdirAll(claudeDir, 0o755)
	settingsFile := filepath.Join(claudeDir, "settings.json")
	_ = os.WriteFile(settingsFile, []byte(`{"hooks":{"PreToolUse":[{"command":"downshift claude-code"}]}}`), 0o644)
	backupFile := settingsFile + setupBackupInfix + "20261028T000000.000000"
	_ = os.WriteFile(backupFile, []byte(`{"original":true}`), 0o644)

	var stdout, stderr bytes.Buffer
	rc := runUninstall([]string{"--yes"}, strings.NewReader(""), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runUninstall(--yes) = %d; stderr: %s", rc, stderr.String())
	}

	restored, err := os.ReadFile(settingsFile)
	if err != nil {
		t.Fatalf("cannot read settings.json: %v", err)
	}
	if !strings.Contains(string(restored), `"original":true`) {
		t.Errorf("settings.json content = %q; want original backup content", string(restored))
	}

	// Verify generated bin dir in stateDir was removed
	if _, err := os.Stat(filepath.Join(stateDir, "bin")); !os.IsNotExist(err) {
		t.Errorf("stateDir/bin was not cleaned up during uninstall")
	}
}

func TestUninstall_PurgeCleansEverything(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	stateDir := filepath.Join(tempHome, ".downshift")
	t.Setenv("DOWNSHIFT_STATE_DIR", stateDir)
	_ = os.MkdirAll(stateDir, 0o755)
	_ = os.WriteFile(filepath.Join(stateDir, "session-models.json"), []byte("{}"), 0o644)

	claudeDir := filepath.Join(tempHome, ".claude")
	_ = os.MkdirAll(claudeDir, 0o755)
	settingsFile := filepath.Join(claudeDir, "settings.json")
	_ = os.WriteFile(settingsFile, []byte(`{"hooks":{}}`), 0o644)
	backupFile := settingsFile + setupBackupInfix + "20261028T000000.000000"
	_ = os.WriteFile(backupFile, []byte(`{"clean":true}`), 0o644)

	var stdout, stderr bytes.Buffer
	rc := runUninstall([]string{"--yes", "--purge"}, strings.NewReader(""), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runUninstall(--yes, --purge) = %d; stderr: %s", rc, stderr.String())
	}

	// 1. Verify stateDir was completely removed
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Errorf("expected stateDir ~/.downshift to be completely purged, but it still exists")
	}

	// 2. Verify backup file was removed
	if _, err := os.Stat(backupFile); !os.IsNotExist(err) {
		t.Errorf("expected backup file %s to be purged, but it still exists", backupFile)
	}
}
