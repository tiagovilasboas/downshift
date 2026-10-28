// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunLoop_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--help"}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runLoop(--help) = %d, want 0", rc)
	}
	out := stdout.String()
	if !strings.Contains(out, "downshift loop") || !strings.Contains(out, "Usage:") {
		t.Errorf("expected usage info in stdout, got %q", out)
	}
}

func TestRunLoop_Success(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", tempDir)

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--task=Unit test", "--session=sess_loop_test", "--json", "--", "true"}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runLoop() = %d, want 0; stderr: %s", rc, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"success": true`) {
		t.Errorf("stdout = %q; want success: true", out)
	}
}

func TestRunLoop_TripwireFailure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", tempDir)

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--task=Broken task", "--iterations=1", "--sensor=false"}, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("runLoop() = %d, want 1 for failed sensor; stdout: %s", rc, stdout.String())
	}
	errStr := stderr.String()
	if !strings.Contains(errStr, "Autonomous loop stopped") {
		t.Errorf("stderr = %q; want 'Autonomous loop stopped'", errStr)
	}
}
