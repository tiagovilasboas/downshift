// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package sensor_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/sensor"
)

func TestSensor_RecordToolOutput_AggregatesWithoutRawText(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(paths.EnvStateDir, tmp)

	store, err := sensor.DefaultStore()
	if err != nil {
		t.Fatalf("failed to create default store: %v", err)
	}

	testOutput := []byte("ok  github.com/foo/bar  0.1s  coverage: 80%\nok  github.com/foo/baz  0.2s  coverage: 90%\n")
	err = store.RecordToolOutput("test-session-123", "claude-code", "Bash", testOutput, false)
	if err != nil {
		t.Fatalf("record failed: %v", err)
	}

	summary, err := store.GetSummary()
	if err != nil {
		t.Fatalf("get summary failed: %v", err)
	}

	obs, ok := summary["test-session-123"]
	if !ok {
		t.Fatalf("expected observation for test-session-123")
	}

	if obs.ToolExecutions != 1 {
		t.Errorf("expected 1 execution, got %d", obs.ToolExecutions)
	}
	if obs.CompressionOpportunities != 1 {
		t.Errorf("expected 1 compression opportunity, got %d", obs.CompressionOpportunities)
	}
	if obs.FormatBreakdown[compressor.FormatGoTest] != 1 {
		t.Errorf("expected 1 go_test format counted")
	}
	if obs.PotentialReducedBytes <= 0 {
		t.Errorf("expected positive potential reduced bytes")
	}

	// Verify states
	if obs.InputTokens.State != sensor.StateUnavailable {
		t.Errorf("expected InputTokens to be StateUnavailable")
	}
}
