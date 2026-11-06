// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestAdoptionLine(t *testing.T) {
	scope := Scope{Kind: ScopeAll}
	if got := AdoptionLine(BuildAdoption(nil, scope)); got != "Planner and workers: no calls yet" {
		t.Errorf("zero: %q", got)
	}
	var evs []telemetry.Event
	for i := 0; i < 6; i++ {
		evs = append(evs, telemetry.MCPTool("plan", "", telemetry.ToolCounts{Steps: 2, SpawnSteps: 1, DelegateSteps: 1}))
	}
	evs = append(evs,
		telemetry.MCPTool("delegate", "", telemetry.ToolCounts{Tasks: 4}),
		telemetry.MCPTool("delegate", "workers_off", telemetry.ToolCounts{Tasks: 1}),
		telemetry.WorkerEnded("codex", "done", nil),
	)
	want := "Planner: 6 plans, 12 steps (6 spawn · 6 delegate) · Workers: 4 delegated, 1 done, 1 refused (workers_off) · gateway 0/4"
	if got := AdoptionLine(BuildAdoption(evs, scope)); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
