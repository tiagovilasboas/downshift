// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
)

// TestSharedRunner_StubDecisionReachesEveryHarness pins the Decider seam at
// the shared layer: whatever the installed Decider says reaches each
// registered hook's wire output through the shared runner, with no adapter
// change. "keep" allows the spawn on its model; "small" moves a frontier spawn
// to the session's small model (a rewrite, or a block that names it).
func TestSharedRunner_StubDecisionReachesEveryHarness(t *testing.T) {
	for _, id := range harnessIDs() {
		entry := harnessRegistry[id]
		driver, ok := conformanceDrivers[id]
		if entry.Hook == nil || !ok {
			continue
		}
		t.Run(id, func(t *testing.T) {
			h := conformanceHarness{
				id:       id,
				caps:     entry.Caps,
				driver:   driver,
				small:    cmdCat.ModelFor(id, core.TierSmall),
				mid:      cmdCat.ModelFor(id, core.TierMid),
				frontier: cmdCat.ModelFor(id, core.TierFrontier),
			}
			decidertest.Each(t, func(t *testing.T, s decidertest.Stub) {
				h.eventLog = conformanceEnv(t)
				h.session(t, h.small, h.mid, h.frontier)
				// The task text says nothing a classifier could use: only
				// the stub decides.
				res := h.run(t, spawnSpec{task: "stub-decided task", model: h.frontier.ID})
				ev := h.lastEvent(t)
				if s.Keeps {
					if res.changed || res.rc != 0 {
						t.Fatalf("keep: want the spawn allowed on its model, got %+v", res)
					}
					// An effort-capable harness may still write the effort
					// (PlanForSession ApplyEffort); the model never moves.
					if res.fields != nil && (!h.caps.CanApplyEffort || res.model != h.frontier.WriteName()) {
						t.Fatalf("keep: rewritten input must keep the requested model, got %+v", res)
					}
					if ev.ToModel != h.frontier.ID {
						t.Errorf("keep: telemetry final_model=%q, want %s", ev.ToModel, h.frontier.ID)
					}
					return
				}
				if !res.changed || res.model != h.small.WriteName() {
					t.Fatalf("small: got changed=%v model=%q rc=%d, want %q", res.changed, res.model, res.rc, h.small.WriteName())
				}
				if ev.Verdict != "DOWNSHIFT" || ev.ToModel != h.small.ID {
					t.Errorf("small: telemetry verdict=%q final_model=%q, want DOWNSHIFT to %s", ev.Verdict, ev.ToModel, h.small.ID)
				}
			})
		})
	}
}
