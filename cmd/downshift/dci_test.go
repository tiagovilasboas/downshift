// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
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
