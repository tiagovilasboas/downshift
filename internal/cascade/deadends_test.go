// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cascade

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCascade_RecordAndRetrieveFailure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", tempDir)

	sessionID := "sess_test_123"
	task := "Run unit tests for auth module"
	failedModel := "claude-haiku-4-5"
	reason := "go test failed with exit code 1"

	// Precondition: no failure recorded
	if _, found := HasRecentFailure(sessionID, task, 5*time.Minute); found {
		t.Fatal("unexpected failure found before recording")
	}

	// Record failure
	RecordFailure(sessionID, task, failedModel, reason)

	// Retrieve failure
	deadEnd, found := HasRecentFailure(sessionID, task, 5*time.Minute)
	if !found {
		t.Fatal("expected failure to be found after recording")
	}
	if deadEnd.FailedModel != failedModel {
		t.Errorf("got FailedModel = %q, want %q", deadEnd.FailedModel, failedModel)
	}
	if deadEnd.Reason != reason {
		t.Errorf("got Reason = %q, want %q", deadEnd.Reason, reason)
	}

	// Clear session
	ClearSession(sessionID)
	if _, found := HasRecentFailure(sessionID, task, 5*time.Minute); found {
		t.Fatal("failure should not be found after ClearSession")
	}
}

func TestCascade_WindowExpiration(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", tempDir)

	sessionID := "sess_expired"
	task := "Fix CSS layout bug"
	RecordFailure(sessionID, task, "claude-haiku-4-5", "test failure")

	// Query with 0 duration window should expire
	if _, found := HasRecentFailure(sessionID, task, -1*time.Second); found {
		t.Fatal("failure should be expired for negative/zero window")
	}
}

func TestCascade_StateFileCap(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", tempDir)

	for i := 0; i < 120; i++ {
		RecordFailure("sess", "task_"+string(rune('A'+(i%26)))+string(rune('0'+(i%10)))+string(rune('a'+(i%20))), "haiku", "fail")
	}

	p := filepath.Join(tempDir, "dead_ends.json")
	entries := loadEntries(p)
	if len(entries) > 100 {
		t.Errorf("entries count = %d; want cap of <= 100", len(entries))
	}
}
