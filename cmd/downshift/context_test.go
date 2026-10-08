// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
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
	if !strings.Contains(s, "RTK (Rust Token Killer)") {
		t.Errorf("expected RTK provider listed in output:\n%s", s)
	}
	if !strings.Contains(s, "command_output_optimization") {
		t.Errorf("expected capability listed in output:\n%s", s)
	}
}

func TestRunContext_StatusDisabledByDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(paths.EnvStateDir, tmp)

	var out, errOut bytes.Buffer
	rc := runContext([]string{"status"}, &out, &errOut)
	if rc != 0 {
		t.Fatalf("expected rc 0, got %d", rc)
	}
	if !strings.Contains(out.String(), "Status: Disabled") {
		t.Errorf("expected disabled status in output:\n%s", out.String())
	}
}

func TestRunContext_Doctor(t *testing.T) {
	var out, errOut bytes.Buffer
	rc := runContext([]string{"doctor"}, &out, &errOut)
	if rc != 0 {
		t.Fatalf("expected rc 0, got %d", rc)
	}
	if !strings.Contains(out.String(), "Provider: RTK (Rust Token Killer)") {
		t.Errorf("expected RTK doctor section in output:\n%s", out.String())
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

func TestRunContext_UnknownSubcommand(t *testing.T) {
	var out, errOut bytes.Buffer
	rc := runContext([]string{"unknown-xyz"}, &out, &errOut)
	if rc != 2 {
		t.Errorf("expected rc 2 for unknown subcommand, got %d", rc)
	}
}
