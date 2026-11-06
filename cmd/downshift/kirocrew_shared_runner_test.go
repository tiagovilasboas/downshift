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
	"github.com/tiagovilasboas/downshift/internal/testenv"
)

// kirocrewSpawn runs the registered KiroCrew hook (the shared runner in
// policy mode) on one spawn and returns the exit code, stdout, stderr and the
// events it logged.
func kirocrewSpawn(t *testing.T, task string, extraEnv map[string]string) (rc int, stdout, stderr string, events []telemetry.Event) {
	t.Helper()
	testenv.SetHome(t, t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	for k, v := range extraEnv {
		t.Setenv(k, v)
	}
	frontierID := cmdCat.ModelFor("kirocrew", core.TierFrontier).ID
	setSessionAllowlist(t, "kirocrew", []string{
		cmdCat.ModelFor("kirocrew", core.TierSmall).ID,
		cmdCat.ModelFor("kirocrew", core.TierMid).ID,
		frontierID,
	})
	stdin := `{"tool_name":"spawn_run","tool_input":{"task":"` + task + `","model":"` + frontierID + `"}}`
	stdout = captureStdout(func() {
		stderr = captureStderr(func() {
			rc = harnessRegistry["kirocrew"].Hook(strings.NewReader(stdin), cmdCat)
		})
	})
	var err error
	events, err = telemetry.ReadEventsFrom(eventLog)
	if err != nil {
		t.Fatal(err)
	}
	return rc, stdout, stderr, events
}

const (
	kirocrewTrivialTask = "rename the userId variable to userIdentifier"
	kirocrewReadTask    = "look at the logs folder and tell me what you see"
)

func hasReasonCode(ev telemetry.Event, code string) bool {
	for _, c := range ev.ReasonCodes {
		if c == code {
			return true
		}
	}
	return false
}

// A confident mismatch with an in-session target: exit 2, the block message
// on stderr under the correlation id, nothing on stdout, outcome "blocked".
func TestKiroCrewSharedRunner_BlockExits2(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	rc, stdout, stderr, events := kirocrewSpawn(t, kirocrewTrivialTask, nil)
	if rc != 2 {
		t.Fatalf("rc = %d, want 2 (block)\nstderr:\n%s", rc, stderr)
	}
	if stdout != "" {
		t.Errorf("policy mode must print nothing on stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "downshift: correlation_id=") || !strings.Contains(stderr, "Respawn this subagent with model=") {
		t.Errorf("stderr must carry the block message under the correlation id:\n%s", stderr)
	}
	if len(events) != 1 || events[0].Outcome != telemetry.OutcomeBlocked {
		t.Fatalf("want one blocked event, got %+v", events)
	}
	if events[0].Harness != "kirocrew" {
		t.Errorf("harness = %q, want kirocrew", events[0].Harness)
	}
	if strings.Count(stderr, "Respawn this subagent") != 1 {
		t.Errorf("block message must appear exactly once (note is not printed again):\n%s", stderr)
	}
}

// The right gear already: exit 0, silence on stdout, outcome "allow" with the
// unchanged model recorded.
func TestKiroCrewSharedRunner_AllowExits0(t *testing.T) {
	decidertest.Use(t, decidertest.Keep())
	rc, stdout, _, events := kirocrewSpawn(t, kirocrewReadTask, nil)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if stdout != "" {
		t.Errorf("policy mode must print nothing on stdout, got %q", stdout)
	}
	if len(events) != 1 || events[0].Outcome != telemetry.OutcomeAllow {
		t.Fatalf("want one allow event, got %+v", events)
	}
	if want := cmdCat.ModelFor("kirocrew", core.TierFrontier).ID; events[0].ToModel != want || events[0].FromModel != want {
		t.Errorf("allow must record the unchanged model: from=%q to=%q want %q", events[0].FromModel, events[0].ToModel, want)
	}
}

// DOWNSHIFT_NO_ROUTE and DOWNSHIFT_CONTROL_SHARE are gone: with either set the
// would-be block is still decided and enforced (exit 2, a blocked event), not
// allowed unchanged as a baseline.
func TestKiroCrewSharedRunner_RemovedVariablesDoNotSkipRouting(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	for name, env := range map[string]map[string]string{
		"no_route":      {"DOWNSHIFT_NO_ROUTE": "1"},
		"control_share": {"DOWNSHIFT_CONTROL_SHARE": "1"},
		"both":          {"DOWNSHIFT_NO_ROUTE": "1", "DOWNSHIFT_CONTROL_SHARE": "1"},
	} {
		t.Run(name, func(t *testing.T) {
			rc, _, stderr, events := kirocrewSpawn(t, kirocrewTrivialTask, env)
			if rc != 2 {
				t.Fatalf("rc = %d, want 2 (still routed and blocked)\nstderr:\n%s", rc, stderr)
			}
			if !strings.Contains(stderr, "Respawn this subagent with model=") {
				t.Errorf("stderr must carry the block message:\n%s", stderr)
			}
			if len(events) != 1 || events[0].Outcome != telemetry.OutcomeBlocked {
				t.Fatalf("want one blocked event, got %+v", events)
			}
			if hasReasonCode(events[0], "CONTROL") {
				t.Errorf("no spawn carries the retired CONTROL code: %v", events[0].ReasonCodes)
			}
		})
	}
}

// Session pause is not covered here: the KiroCrew payload carries no session
// id (no SessionIdentifier on kirocrew.Event), so telemetry.SessionPaused sees
// an empty hash and the spawn is never paused. See docs/reference/harness-matrix.md.
