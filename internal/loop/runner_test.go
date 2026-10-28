// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/cascade"
)

func TestRunner_SuccessOnPass(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", tempDir)

	cfg := Config{
		Task:          "Add healthcheck endpoint",
		SensorCmd:     "true",
		SessionID:     "sess_loop_pass",
		MaxIterations: 2,
		Timeout:       10 * time.Second,
	}

	runner := NewRunner(cfg)
	res, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected res.Success = true, got false")
	}
	if !res.SensorPassed {
		t.Errorf("expected res.SensorPassed = true")
	}
	if !res.Accepted {
		t.Errorf("expected res.Accepted = true")
	}
	if res.FinalStep != StepComplete {
		t.Errorf("FinalStep = %s, want %s", res.FinalStep, StepComplete)
	}
}

func TestRunner_TripwireOnRepeatedFailure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", tempDir)

	sessionID := "sess_loop_fail"
	cfg := Config{
		Task:          "Fix race condition in db",
		SensorCmd:     "false", // exits with code 1
		SessionID:     sessionID,
		MaxIterations: 2,
		Timeout:       10 * time.Second,
	}

	runner := NewRunner(cfg)
	res, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success {
		t.Errorf("expected res.Success = false on repeated failure")
	}
	if res.FinalStep != StepFailed {
		t.Errorf("FinalStep = %s, want %s", res.FinalStep, StepFailed)
	}

	// Verify that cascade recorded the dead-end failure
	p := filepath.Join(tempDir, "dead_ends.json")
	_ = p
	if _, found := cascade.HasRecentFailure(sessionID, cfg.Task, 5*time.Minute); !found {
		t.Errorf("expected dead-end recorded in cascade after sensor failure")
	}
}

func TestRunner_AllowlistedSensorPass(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", tempDir)

	cfg := Config{
		Task:          "Run paths unit test",
		SensorCmd:     "go test github.com/tiagovilasboas/downshift/internal/paths",
		SessionID:     "sess_loop_paths",
		MaxIterations: 1,
		Timeout:       10 * time.Second,
	}

	runner := NewRunner(cfg)
	res, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected res.Success = true")
	}
	if !res.SensorPassed {
		t.Errorf("expected res.SensorPassed = true")
	}
}

func TestRunnerPassKeepsSiblingTaskFailure(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	cascade.RecordFailure("sess-batch", "task a", "m", "failed a")

	res, err := NewRunner(Config{Task: "task b", SensorCmd: "true", SessionID: "sess-batch", MaxIterations: 1}).Run(context.Background())
	if err != nil || !res.Success {
		t.Fatalf("run = %+v, %v", res, err)
	}
	if _, found := cascade.HasRecentFailure("sess-batch", "task a", time.Minute); !found {
		t.Error("a passing sibling task must not erase the failure record of task a")
	}
}
