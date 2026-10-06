// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// KiroCrew never rewrites: its events are "blocked" (exit 2) or "allow"
// (exit 0), never "rewrite_emitted".
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
	}{
		{"rename the userId variable to userIdentifier", telemetry.OutcomeBlocked, 2},
		{"look at the logs folder and tell me what you see", telemetry.OutcomeAllow, 0},
		{"rearchitect the payment system across services", telemetry.OutcomeAllow, 0},
	}
	for _, tc := range cases {
		stdin := `{"tool_name":"spawn_run","tool_input":{"task":"` + tc.task + `","model":"` + frontierID + `"}}`
		var rc int
		captureStderr(func() { rc = runKiroCrewHook(strings.NewReader(stdin), cmdCat) })
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
