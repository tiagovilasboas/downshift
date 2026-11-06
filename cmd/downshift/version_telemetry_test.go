// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"runtime/debug"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestTelemetryVersionMatchesReportedVersion(t *testing.T) {
	want, _ := currentBuild()
	if got := telemetryVersion(); got != want {
		t.Errorf("telemetryVersion() = %q, want %q (what downshift version shows)", got, want)
	}
}

// A go install build resolves a module version, so its failures are not
// "dev" and the stale-dev rule of the circuit leaves them alone; a real dev
// build stays "dev" and still expires.
func TestStaleDevFailureByResolvedVersion(t *testing.T) {
	now := time.Now()
	old := now.Add(-2 * circuitDevFailureTTL).UTC().Format(time.RFC3339Nano)
	cases := []struct {
		name, stamped, module string
		wantStale             bool
	}{
		{"go install release", "dev", "v0.8.1", false},
		{"devel build", "dev", "(devel)", true},
		{"ldflags release", "v0.9.0", "v0.8.1", false},
	}
	for _, c := range cases {
		v, _ := resolveBuild(c.stamped, "", &debug.BuildInfo{Main: debug.Module{Version: c.module}})
		ev := telemetry.Event{BinaryVersion: v, Timestamp: old}
		if got := staleDevFailure(ev, now); got != c.wantStale {
			t.Errorf("%s: stale = %v, want %v (version %q)", c.name, got, c.wantStale, v)
		}
	}
}
