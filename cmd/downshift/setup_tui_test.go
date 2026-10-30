// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetupTUI_NoHarnessesDetected(t *testing.T) {
	tempHome := t.TempDir()
	var stdout, stderr bytes.Buffer

	opts := setupOptions{
		Home: tempHome,
		In:   strings.NewReader(""),
		Out:  &stdout,
		Err:  &stderr,
		Now:  time.Now,
		TUI:  true,
	}

	rc := setupTUI(opts)
	if rc != 1 {
		t.Fatalf("setupTUI() = %d, want 1 when no harnesses are found", rc)
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "no supported harness found") {
		t.Errorf("setupTUI() stderr = %q; want 'no supported harness found'", errOut)
	}
}

func TestSetupTUI_DetectedHarnessAndAbort(t *testing.T) {
	tempHome := t.TempDir()
	claudeDir := filepath.Join(tempHome, ".claude")
	_ = os.MkdirAll(claudeDir, 0o755)

	var stdout, stderr bytes.Buffer
	opts := setupOptions{
		Home:   tempHome,
		In:     strings.NewReader("n\n"),
		Out:    &stdout,
		Err:    &stderr,
		Now:    time.Now,
		TUI:    true,
		DryRun: true,
	}

	rc := setupTUI(opts)
	if rc != 0 {
		t.Fatalf("setupTUI() with 'n' = %d, want 0", rc)
	}
	out := stdout.String()
	if !strings.Contains(out, "Setup aborted by user") {
		t.Errorf("setupTUI() output = %q; want 'Setup aborted by user'", out)
	}
}

func TestSetupTUI_CustomizePresetAndAbortAtConfirm(t *testing.T) {
	tempHome := t.TempDir()
	claudeDir := filepath.Join(tempHome, ".claude")
	_ = os.MkdirAll(claudeDir, 0o755)

	var stdout, stderr bytes.Buffer
	// Input: 'c' to customize, '2' for Aggressive, 'n' to abort at final confirmation
	opts := setupOptions{
		Home:   tempHome,
		In:     strings.NewReader("c\n2\nn\n"),
		Out:    &stdout,
		Err:    &stderr,
		Now:    time.Now,
		TUI:    true,
		DryRun: true,
	}

	rc := setupTUI(opts)
	if rc != 0 {
		t.Fatalf("setupTUI() customize with 'n' = %d, want 0", rc)
	}
	out := stdout.String()
	if !strings.Contains(out, "Preset selecionado: Economia Máxima (Aggressive)") {
		t.Errorf("setupTUI() did not record aggressive preset:\n%s", out)
	}
	if !strings.Contains(out, "Setup aborted by user") {
		t.Errorf("setupTUI() output = %q; want 'Setup aborted by user'", out)
	}
}

func TestSetupTUI_ConfirmAndExecuteDryRun(t *testing.T) {
	tempHome := t.TempDir()
	claudeDir := filepath.Join(tempHome, ".claude")
	_ = os.MkdirAll(claudeDir, 0o755)

	var stdout, stderr bytes.Buffer
	// Input: 's' for standard config, 's' for apply
	opts := setupOptions{
		Home:   tempHome,
		In:     strings.NewReader("s\ns\n"),
		Out:    &stdout,
		Err:    &stderr,
		Now:    time.Now,
		TUI:    true,
		DryRun: true,
	}

	rc := setupTUI(opts)
	if rc != 0 {
		t.Fatalf("setupTUI() dry-run confirmed = %d, want 0 (stderr: %s)", rc, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "⚡ Downshift Setup Wizard") {
		t.Errorf("setupTUI() missing header:\n%s", out)
	}
}

func TestIsInteractiveTerminal(t *testing.T) {
	if isInteractiveTerminal(nil) {
		t.Error("nil reader reported as interactive terminal")
	}
	if isInteractiveTerminal(strings.NewReader("not a terminal")) {
		t.Error("strings.Reader reported as interactive terminal")
	}
	if isInteractiveTerminal(&bytes.Buffer{}) {
		t.Error("bytes.Buffer reported as interactive terminal")
	}
}

func TestSmokeCheckAllHarnesses(t *testing.T) {
	harnesses := []string{"claude-code", "cursor", "codex", "antigravity", "kirocrew"}
	for _, h := range harnesses {
		t.Run(h, func(t *testing.T) {
			dur, ok := runSmokeCheck(h, cmdCat)
			if !ok {
				t.Fatalf("runSmokeCheck(%s) failed", h)
			}
			if dur > 500*time.Millisecond {
				t.Errorf("runSmokeCheck(%s) took %v, want < 500ms", h, dur)
			}
		})
	}
}
