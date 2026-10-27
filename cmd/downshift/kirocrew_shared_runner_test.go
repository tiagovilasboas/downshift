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

// kirocrewSpawn runs the registered KiroCrew hook (the shared runner in
// policy mode) on one spawn and returns the exit code, stdout, stderr and the
// events it logged.
func kirocrewSpawn(t *testing.T, task string, extraEnv map[string]string) (rc int, stdout, stderr string, events []telemetry.Event) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")
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

// DOWNSHIFT_NO_ROUTE=1 classifies the would-be block but never blocks: exit 0
// and a baseline event without the CONTROL code.
func TestKiroCrewSharedRunner_NoRouteIsBaseline(t *testing.T) {
	rc, stdout, stderr, events := kirocrewSpawn(t, kirocrewTrivialTask, map[string]string{"DOWNSHIFT_NO_ROUTE": "1"})
	if rc != 0 {
		t.Fatalf("rc = %d, want 0 (NO_ROUTE never blocks)\nstderr:\n%s", rc, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout must stay empty, got %q", stdout)
	}
	if strings.Contains(stderr, "Respawn this subagent") {
		t.Errorf("NO_ROUTE must not relay a block message:\n%s", stderr)
	}
	if len(events) != 1 || events[0].Outcome != telemetry.OutcomeBaseline {
		t.Fatalf("want one baseline event, got %+v", events)
	}
	if hasReasonCode(events[0], telemetry.ReasonControl) {
		t.Errorf("NO_ROUTE spawns are not the control group: %v", events[0].ReasonCodes)
	}
}

// The built-in control group allows the would-be block: exit 0, the control
// line on stderr, a baseline event carrying CONTROL.
func TestKiroCrewSharedRunner_ControlGroupIsBaseline(t *testing.T) {
	rc, stdout, stderr, events := kirocrewSpawn(t, kirocrewTrivialTask, map[string]string{"DOWNSHIFT_CONTROL_SHARE": "1"})
	if rc != 0 {
		t.Fatalf("rc = %d, want 0 (control group never blocks)\nstderr:\n%s", rc, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout must stay empty, got %q", stdout)
	}
	if !strings.Contains(stderr, "control group, spawn unchanged") {
		t.Errorf("stderr must say control group:\n%s", stderr)
	}
	if strings.Contains(stderr, "Respawn this subagent") {
		t.Errorf("control group must not relay a block message:\n%s", stderr)
	}
	if len(events) != 1 || events[0].Outcome != telemetry.OutcomeBaseline {
		t.Fatalf("want one baseline event, got %+v", events)
	}
	if !hasReasonCode(events[0], telemetry.ReasonControl) {
		t.Errorf("reason codes %v lack %s", events[0].ReasonCodes, telemetry.ReasonControl)
	}
}

// Session pause is not covered here: the KiroCrew payload carries no session
// id (no SessionIdentifier on kirocrew.Event), so telemetry.SessionPaused sees
// an empty hash and the spawn is never paused. See docs/reference/harness-matrix.md.
