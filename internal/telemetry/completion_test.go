// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestCompletionByHarness(t *testing.T) {
	events := []telemetry.Event{
		{Harness: "claude-code", Outcome: telemetry.OutcomeUsage},
		{Harness: "claude-code", Outcome: telemetry.OutcomeChildEnded, ChildStatus: telemetry.ChildLaunched},
		{Harness: "codex", Outcome: telemetry.OutcomeChildEnded, ChildStatus: telemetry.ChildLaunched}, // a launch is not an end
		{Harness: "cursor", Outcome: telemetry.OutcomeChildEnded, ChildStatus: telemetry.ChildCompleted},
		{Harness: "antigravity", Outcome: telemetry.OutcomeChildEnded, ChildStatus: telemetry.ChildCompleted}, // no end event declared
		{Harness: "codex", Outcome: telemetry.OutcomeAllow},
	}
	got := map[string]telemetry.HarnessCompletion{}
	var order []string
	for _, c := range telemetry.CompletionByHarness(events) {
		got[c.Harness] = c
		order = append(order, c.Harness)
	}
	for i, n := range core.AllHarnessCaps() {
		if order[i] != n.Harness {
			t.Fatalf("order = %v, want the core.AllHarnessCaps order", order)
		}
	}
	want := map[string]struct {
		measurable bool
		ends       int
	}{"claude-code": {true, 1}, "codex": {false, 0}, "cursor": {true, 1}, "antigravity": {false, 1}, "kirocrew": {false, 0}}
	for h, w := range want {
		if got[h].Measurable != w.measurable || got[h].EndRecords != w.ends {
			t.Errorf("%s = %+v, want measurable=%v ends=%d", h, got[h], w.measurable, w.ends)
		}
	}
	if d := got["antigravity"].Describe(); !strings.Contains(d, "no end event known") {
		t.Errorf("antigravity: %q", d)
	}
	if d := got["codex"].Describe(); !strings.Contains(d, "not yet measurable") || !strings.Contains(d, "PostToolUse") {
		t.Errorf("codex: %q", d)
	}
}
