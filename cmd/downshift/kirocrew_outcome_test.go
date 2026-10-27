// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// KiroCrew never rewrites: its events are "blocked" (exit 2) or "allow"
// (exit 0), never "rewrite_emitted". Runs the registered hook, which is the
// shared runner in policy mode.
func TestRunKiroCrewHook_RecordsAllowAndBlocked(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", path)
	frontierID := cmdCat.ModelFor("kirocrew", core.TierFrontier).ID
	setSessionAllowlist(t, "kirocrew", []string{
		cmdCat.ModelFor("kirocrew", core.TierSmall).ID,
		cmdCat.ModelFor("kirocrew", core.TierMid).ID,
		frontierID,
	})
	cases := []struct {
		task, wantOutcome string
		wantRC            int
		decider           core.Decider
	}{
		{"rename the userId variable to userIdentifier", telemetry.OutcomeBlocked, 2, decidertest.Tier(core.TierSmall)},
		{"look at the logs folder and tell me what you see", telemetry.OutcomeAllow, 0, decidertest.Keep()},
		{"rearchitect the payment system across services", telemetry.OutcomeAllow, 0, decidertest.Tier(core.TierFrontier)},
	}
	for _, tc := range cases {
		stdin := `{"tool_name":"spawn_run","tool_input":{"task":"` + tc.task + `","model":"` + frontierID + `"}}`
		var rc int
		restore := core.SetDecider(tc.decider)
		captureStderr(func() { rc = harnessRegistry["kirocrew"].Hook(strings.NewReader(stdin), cmdCat) })
		restore()
		if rc != tc.wantRC {
			t.Fatalf("%q: rc=%d, want %d", tc.task, rc, tc.wantRC)
		}
	}
	events, err := telemetry.ReadEventsFrom(path)
	if err != nil || len(events) != len(cases) {
		t.Fatalf("events=%d err=%v", len(events), err)
	}
	for i, tc := range cases {
		if events[i].Outcome != tc.wantOutcome {
			t.Errorf("%q: outcome=%q, want %q", tc.task, events[i].Outcome, tc.wantOutcome)
		}
		if tc.wantOutcome == telemetry.OutcomeAllow && events[i].ToModel != frontierID {
			t.Errorf("%q: allow must record the unchanged model, got final_model=%q", tc.task, events[i].ToModel)
		}
	}
}
