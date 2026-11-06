// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// Harness conformance suite. One table of harness-free routing scenarios runs
// against every hook in harnessRegistry through the shared runner, so a
// behaviour that holds for one harness is proved for all of them in one place
// (AGENTS.md layering rule 4: test at the shared layer). The only per-harness
// code is data: how to build that harness's spawn payload and how to read its
// wire response back into a hookResult. A scenario is skipped only on a
// declared capability or on catalog data, never on a harness name.

import (
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
	"github.com/tiagovilasboas/downshift/internal/testenv"
)

// conformanceEnv isolates one scenario: fresh HOME and state dir, a private
// event log, and no session file until the scenario installs one.
func conformanceEnv(t *testing.T) string {
	t.Helper()
	testenv.SetHome(t, t.TempDir())
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	t.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(t.TempDir(), "absent-session-models.json"))
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	return eventLog
}

// TestHarnessConformance runs every scenario against every hook in the
// registry. Harness ids come from the registry and core.AllHarnessCaps, never
// from a literal in a loop.
func TestHarnessConformance(t *testing.T) {
	runConformance(t)
}

func runConformance(t *testing.T) {
	t.Helper()
	// Every declared harness must be routable through the registry, and every
	// registered hook must have conformance data.
	for _, n := range core.AllHarnessCaps() {
		if e, ok := harnessRegistry[n.Harness]; !ok || e.Hook == nil {
			t.Fatalf("harness %q is declared in core.AllHarnessCaps but has no registry hook", n.Harness)
		}
	}
	for _, id := range harnessIDs() {
		entry := harnessRegistry[id]
		if entry.Hook == nil {
			continue // try-only harness: nothing to conform
		}
		driver, ok := conformanceDrivers[id]
		if !ok {
			t.Fatalf("registry harness %q has no conformance driver; add its payload builder and decoder", id)
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
			if h.small.ID == "" || h.mid.ID == "" || h.frontier.ID == "" {
				t.Fatalf("catalog lacks a tier model for %s (small=%q mid=%q frontier=%q)", id, h.small.ID, h.mid.ID, h.frontier.ID)
			}
			for _, sc := range conformanceScenarios {
				t.Run(sc.name, func(t *testing.T) {
					if sc.decider != nil {
						decidertest.Use(t, sc.decider)
					}
					if sc.skip != nil {
						if reason := sc.skip(h.caps); reason != "" {
							t.Skip(reason)
						}
					}
					h.eventLog = conformanceEnv(t)
					sc.run(t, h)
				})
			}
		})
	}
}
