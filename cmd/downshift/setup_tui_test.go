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
