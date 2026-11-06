// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestRunAccept_WithSession(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)

	var stdout, stderr bytes.Buffer
	rc := runAccept([]string{"--session=sess_abc", "--used"}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runAccept() = %d, want 0; stderr: %s", rc, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "sess_abc marked as used") {
		t.Errorf("stdout = %q; want 'sess_abc marked as used'", out)
	}

	events, err := telemetry.ReadEventsFrom(logPath)
	if err != nil || len(events) != 1 {
		t.Fatalf("expected 1 event, got %d (%v)", len(events), err)
	}
	if events[0].Acceptance != "used" {
		t.Errorf("Acceptance = %q, want 'used'", events[0].Acceptance)
	}
}

func TestRunAccept_Discarded(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)

	var stdout, stderr bytes.Buffer
	rc := runAccept([]string{"--session=sess_xyz", "--discarded"}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runAccept() = %d, want 0; stderr: %s", rc, stderr.String())
	}

	events, err := telemetry.ReadEventsFrom(logPath)
	if err != nil || len(events) != 1 {
		t.Fatalf("expected 1 event, got %d (%v)", len(events), err)
	}
	if events[0].Acceptance != "discarded" {
		t.Errorf("Acceptance = %q, want 'discarded'", events[0].Acceptance)
	}
}
