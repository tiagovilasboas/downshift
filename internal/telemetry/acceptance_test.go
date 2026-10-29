// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"path/filepath"
	"testing"
)

func TestRecordAcceptance_RoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)

	sessionID := "test_session_git_accept"
	err := RecordAcceptance(sessionID, AcceptanceUsed, "claude-code")
	if err != nil {
		t.Fatalf("RecordAcceptance failed: %v", err)
	}

	events, err := ReadEventsFrom(logPath)
	if err != nil {
		t.Fatalf("ReadEventsFrom failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	ev := events[0]
	if ev.Outcome != OutcomeAcceptance {
		t.Errorf("got Outcome = %q, want %q", ev.Outcome, OutcomeAcceptance)
	}
	if ev.Acceptance != string(AcceptanceUsed) {
		t.Errorf("got Acceptance = %q, want %q", ev.Acceptance, AcceptanceUsed)
	}
	if ev.SessionID != sessionID {
		t.Errorf("got SessionID = %q, want %q", ev.SessionID, sessionID)
	}
}
