// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/codeintel"
)

func TestDCIHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runDCI([]string{"help"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDCI(help) = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage: downshift dci") {
		t.Errorf("stdout does not contain usage: %s", stdout.String())
	}
}

func TestDCIStatus(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runDCI([]string{"status"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDCI(status) = %d, want 0. stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Downshift Code Intelligence (DCI) Status:") {
		t.Errorf("unexpected status output: %s", out)
	}
}

func TestDCIEnableAndDisable(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Test enable observe
	code := runDCI([]string{"enable", "observe"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDCI(enable observe) = %d, want 0. stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "enabled in mode \"observe\"") {
		t.Errorf("unexpected enable output: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	// Test disable
	code = runDCI([]string{"disable"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDCI(disable) = %d, want 0. stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "disabled (mode off)") {
		t.Errorf("unexpected disable output: %s", stdout.String())
	}
}

func TestDCIEnableBudgetAndClamping(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tempDir)

	var stdout, stderr bytes.Buffer

	// 1. Valid budget (--budget 5000)
	code := runDCI([]string{"enable", "active", "--budget", "5000"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDCI(enable active --budget 5000) = %d, want 0. stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "budget: 5000 bytes") {
		t.Errorf("expected budget 5000 in stdout, got: %s", stdout.String())
	}
	cfg, err := codeintel.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Mode != codeintel.ModeActive || cfg.MaxContextBytes != 5000 {
		t.Errorf("config = {Mode: %q, MaxContextBytes: %d}, want {active, 5000}", cfg.Mode, cfg.MaxContextBytes)
	}

	// 2. Clamp low out-of-range budget (< 512 -> 512)
	stdout.Reset()
	stderr.Reset()
	code = runDCI([]string{"enable", "active", "--budget", "100"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDCI(enable active --budget 100) = %d, want 0. stderr: %s", code, stderr.String())
	}
	cfg, err = codeintel.LoadConfig(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxContextBytes != 512 {
		t.Errorf("clamped low MaxContextBytes = %d, want 512", cfg.MaxContextBytes)
	}

	// 3. Clamp high out-of-range budget (> 16384 -> 16384) via --budget=N
	stdout.Reset()
	stderr.Reset()
	code = runDCI([]string{"enable", "observe", "--budget=99999"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDCI(enable observe --budget=99999) = %d, want 0. stderr: %s", code, stderr.String())
	}
	cfg, err = codeintel.LoadConfig(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != codeintel.ModeObserve || cfg.MaxContextBytes != 16384 {
		t.Errorf("clamped high config = {Mode: %q, MaxContextBytes: %d}, want {observe, 16384}", cfg.Mode, cfg.MaxContextBytes)
	}

	// 4. Invalid non-integer budget returns exit code 2
	stdout.Reset()
	stderr.Reset()
	code = runDCI([]string{"enable", "active", "--budget", "notanumber"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("runDCI(enable active --budget notanumber) = %d, want 2", code)
	}
}

func TestDCIHookMultiHarnessAndPayloadFields(t *testing.T) {
	tempDir := t.TempDir()

	srcFile := filepath.Join(tempDir, "dispatcher.go")
	srcContent := `package dispatch

// DispatchOrderEvent routes order events to downstream consumers
func DispatchOrderEvent(orderID string) error {
	return nil
}
`
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := codeintel.SetMode(tempDir, codeintel.ModeActive); err != nil {
		t.Fatal(err)
	}

	harnesses := []struct {
		harness   string
		fieldKey  string
		wantTopLv bool
	}{
		{harness: "cursor", fieldKey: "prompt", wantTopLv: true},
		{harness: "opencode", fieldKey: "user_message", wantTopLv: true},
		{harness: "codex", fieldKey: "task", wantTopLv: true},
		{harness: "generic", fieldKey: "user_message", wantTopLv: true},
		{harness: "claude-code", fieldKey: "task", wantTopLv: false},
	}

	for _, tc := range harnesses {
		payload := map[string]string{
			"cwd":       tempDir,
			tc.fieldKey: "How does DispatchOrderEvent route order events?",
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		code := runDCI([]string{"hook", tc.harness}, bytes.NewReader(payloadBytes), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("runDCI(hook %s) with %s = %d, want 0. stderr: %s", tc.harness, tc.fieldKey, code, stderr.String())
		}
		if stdout.Len() == 0 {
			t.Fatalf("expected non-empty hook output for harness=%s field=%s", tc.harness, tc.fieldKey)
		}

		var resp userPromptSubmitOutput
		if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal hook output for harness=%s failed: %v. Raw: %s", tc.harness, err, stdout.String())
		}
		if !strings.Contains(resp.HookSpecificOutput.AdditionalContext, "DispatchOrderEvent") {
			t.Errorf("harness=%s field=%s missing DispatchOrderEvent in HookSpecificOutput.AdditionalContext: %s",
				tc.harness, tc.fieldKey, resp.HookSpecificOutput.AdditionalContext)
		}
		if tc.wantTopLv && !strings.Contains(resp.AdditionalContext, "DispatchOrderEvent") {
			t.Errorf("harness=%s field=%s expected top-level additionalContext, got %q",
				tc.harness, tc.fieldKey, resp.AdditionalContext)
		}
	}

	// Verify unsupported harness still fails open with exit 0 and empty output
	var stdout, stderr bytes.Buffer
	code := runDCI([]string{"hook", "unsupported-harness"}, strings.NewReader(`{"prompt":"DispatchOrderEvent"}`), &stdout, &stderr)
	if code != 0 || stdout.Len() != 0 {
		t.Errorf("unsupported harness: got code=%d out=%q, want 0 and empty", code, stdout.String())
	}
}

func TestDCIDoctor(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runDCI([]string{"doctor"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDCI(doctor) = %d, want 0. stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Checking Downshift Code Intelligence (DCI) Health:") {
		t.Errorf("unexpected doctor output: %s", out)
	}
}

func TestDCIInspect(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runDCI([]string{"inspect", "tokenization", "identifiers"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDCI(inspect) = %d, want 0. stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "=== DCI Query Inspection ===") {
		t.Errorf("unexpected inspect output: %s", out)
	}
	if !strings.Contains(out, "Observed Telemetry") {
		t.Errorf("missing telemetry in inspect output: %s", out)
	}
}

func TestDCIBenchmark(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runDCI([]string{"benchmark"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDCI(benchmark) = %d, want 0. stderr: %s\nstdout:\n%s", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Running Downshift Code Intelligence (DCI) Benchmarks") {
		t.Errorf("unexpected benchmark output: %s", out)
	}
	if !strings.Contains(out, "Warm Path p95 Latency") {
		t.Errorf("missing warm path p95 latency in benchmark output: %s", out)
	}
}
