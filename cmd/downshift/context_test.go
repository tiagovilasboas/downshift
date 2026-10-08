// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

func TestRunContext_ProvidersList(t *testing.T) {
	var out, errOut bytes.Buffer
	rc := runContext([]string{"providers"}, &out, &errOut)
	if rc != 0 {
		t.Fatalf("expected rc 0, got %d, stderr: %s", rc, errOut.String())
	}
	s := out.String()
	if !strings.Contains(s, "Downshift Native Context Compressor") {
		t.Errorf("expected Native compressor listed in output:\n%s", s)
	}
	if !strings.Contains(s, "command_output_optimization") {
		t.Errorf("expected capability listed in output:\n%s", s)
	}
}

func TestRunContext_StatusEnabledByDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(paths.EnvStateDir, tmp)

	var out, errOut bytes.Buffer
	rc := runContext([]string{"status"}, &out, &errOut)
	if rc != 0 {
		t.Fatalf("expected rc 0, got %d", rc)
	}
	if !strings.Contains(out.String(), "Status: Enabled") {
		t.Errorf("expected enabled status in output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Active Provider: native") {
		t.Errorf("expected active provider native in output:\n%s", out.String())
	}

	// Now disable
	out.Reset()
	rc = runContext([]string{"disable"}, &out, &errOut)
	if rc != 0 {
		t.Fatalf("expected rc 0 on disable, got %d", rc)
	}
	out.Reset()
	rc = runContext([]string{"status"}, &out, &errOut)
	if !strings.Contains(out.String(), "Status: Disabled") {
		t.Errorf("expected disabled status in output:\n%s", out.String())
	}

	// Now re-enable
	out.Reset()
	rc = runContext([]string{"enable"}, &out, &errOut)
	if rc != 0 {
		t.Fatalf("expected rc 0 on enable, got %d", rc)
	}
	out.Reset()
	rc = runContext([]string{"status"}, &out, &errOut)
	if !strings.Contains(out.String(), "Status: Enabled") {
		t.Errorf("expected enabled status in output:\n%s", out.String())
	}
}

func TestRunContext_Doctor(t *testing.T) {
	var out, errOut bytes.Buffer
	rc := runContext([]string{"doctor"}, &out, &errOut)
	if rc != 0 {
		t.Fatalf("expected rc 0, got %d", rc)
	}
	if !strings.Contains(out.String(), "Provider: Downshift Native Context Compressor") {
		t.Errorf("expected Native compressor doctor section in output:\n%s", out.String())
	}
}

func TestRunContext_Benchmark(t *testing.T) {
	var out, errOut bytes.Buffer
	rc := runContext([]string{"benchmark"}, &out, &errOut)
	if rc != 0 {
		t.Fatalf("expected rc 0, got %d", rc)
	}
	s := out.String()
	if !strings.Contains(s, "Scenario A") || !strings.Contains(s, "Scenario D") {
		t.Errorf("expected scenario A and D in benchmark output:\n%s", s)
	}
}

func TestRunContext_Compress(t *testing.T) {
	// 1. Observe mode prints metrics to stderr and passes output through to stdout
	in := []byte("ok  github.com/foo/bar  0.1s  coverage: 80%\nok  github.com/foo/baz  0.2s  coverage: 90%\n")
	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		_, _ = w.Write(in)
		_ = w.Close()
	}()

	var out, errOut bytes.Buffer
	rc := runContext([]string{"compress", "observe"}, &out, &errOut)
	os.Stdin = origStdin
	if rc != 0 {
		t.Fatalf("expected rc 0, got %d", rc)
	}
	if !bytes.Equal(out.Bytes(), in) {
		t.Errorf("observe mode must preserve stdout byte-identical")
	}
	if !strings.Contains(errOut.String(), "context observe: format=go_test") {
		t.Errorf("expected format detection in stderr: %s", errOut.String())
	}
}

func TestRunContext_UnknownSubcommand(t *testing.T) {
	var out, errOut bytes.Buffer
	rc := runContext([]string{"unknown-xyz"}, &out, &errOut)
	if rc != 2 {
		t.Errorf("expected rc 2 for unknown subcommand, got %d", rc)
	}
}
