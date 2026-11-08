// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

// clearHarnessEnv empties every identification variable any harness declares.
func clearHarnessEnv(t *testing.T) {
	t.Helper()
	for _, n := range core.AllHarnessCaps() {
		for _, name := range []string{n.Caps.SessionEnv, n.Caps.ShellEnv} {
			if name != "" {
				t.Setenv(name, "")
			}
		}
	}
}

func TestEnvHarnessDeclaredEnvMapsToHarness(t *testing.T) {
	for _, n := range core.AllHarnessCaps() {
		for _, name := range []string{n.Caps.SessionEnv, n.Caps.ShellEnv} {
			if name == "" {
				continue
			}
			t.Run(n.Harness+"/"+name, func(t *testing.T) {
				clearHarnessEnv(t)
				t.Setenv(name, "x")
				if got := envHarness(); got != n.Harness {
					t.Fatalf("envHarness with %s set = %q, want %q", name, got, n.Harness)
				}
			})
		}
	}
}

func TestEnvHarnessNothingSetIsUnknown(t *testing.T) {
	clearHarnessEnv(t)
	if got := envHarness(); got != "unknown" {
		t.Fatalf("envHarness with nothing set = %q, want unknown", got)
	}
}
