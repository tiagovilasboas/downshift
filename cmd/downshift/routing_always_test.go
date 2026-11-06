// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// Every spawn is routed through the shared runner, whatever the environment
// says, and `downshift report` reads it.

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"github.com/tiagovilasboas/downshift/internal/testenv"
)

func runCursorSpawn(t *testing.T, eventLog string) (updated map[string]any, events []telemetry.Event) {
	t.Helper()
	smallID := cmdCat.ModelFor("cursor", core.TierSmall).ID
	midID := cmdCat.ModelFor("cursor", core.TierMid).ID
	frontierID := cmdCat.ModelFor("cursor", core.TierFrontier).ID
	setSessionAllowlist(t, "cursor", []string{smallID, midID, frontierID})
	stdin := []byte(`{"hook_event_name":"preToolUse","tool_name":"Task","model_id":"` +
		frontierID + `","tool_input":{"task":"rename the userId variable","model":"` + frontierID + `"}}`)
	out := captureStdout(func() {
		rc := runHookAdapter(
			"cursor",
			bytes.NewReader(stdin),
			func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
			func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, cmdCat) },
			printCursorAllow,
		)
		if rc != 0 {
			t.Errorf("runHookAdapter() rc = %d, want 0", rc)
		}
	})
	var resp struct {
		UpdatedInput map[string]any `json:"updated_input"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("hook stdout is not valid JSON: %v\n%s", err, out)
	}
	events, err := telemetry.ReadEventsFrom(eventLog)
	if err != nil {
		t.Fatal(err)
	}
	return resp.UpdatedInput, events
}

// Every spawn is routed. DOWNSHIFT_CONTROL_SHARE and DOWNSHIFT_NO_ROUTE were
// removed, so setting either (or both) changes nothing: the spawn is rewritten
// and logged as a routed decision, never as an unrouted baseline.
func TestEverySpawnIsRouted_RemovedVariablesHaveNoEffect(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"unset":              {},
		"control_share_1":    {"DOWNSHIFT_CONTROL_SHARE": "1"},
		"no_route_1":         {"DOWNSHIFT_NO_ROUTE": "1"},
		"both":               {"DOWNSHIFT_CONTROL_SHARE": "1", "DOWNSHIFT_NO_ROUTE": "1"},
		"control_share_half": {"DOWNSHIFT_CONTROL_SHARE": "0.5"},
	} {
		t.Run(name, func(t *testing.T) {
			decidertest.Use(t, decidertest.Tier(core.TierSmall))
			testenv.SetHome(t, t.TempDir())
			eventLog := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
			for k, v := range env {
				t.Setenv(k, v)
			}
			updated, events := runCursorSpawn(t, eventLog)
			if updated == nil {
				t.Fatal("the spawn must be rewritten: it is routed whatever the removed variables say")
			}
			if len(events) != 1 || events[0].Outcome != telemetry.OutcomeRewriteEmitted {
				t.Fatalf("want one routed (rewrite_emitted) event, got %+v", events)
			}
			for _, c := range events[0].ReasonCodes {
				if c == "CONTROL" {
					t.Errorf("no spawn carries the retired CONTROL code: %v", events[0].ReasonCodes)
				}
			}
		})
	}
}

func TestRunReport_JSONAndFlags(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	var out, errOut bytes.Buffer
	if rc := runReport([]string{"--days=7", "--json"}, &out, &errOut); rc != 0 {
		t.Fatalf("rc = %d, stderr %s", rc, errOut.String())
	}
	var rep telemetry.Report
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("report --json is not JSON: %v\n%s", err, out.String())
	}
	if rep.Routed.Spawns != 0 {
		t.Errorf("empty log has no routed spawn: %+v", rep)
	}
	out.Reset()
	if rc := runReport(nil, &out, &errOut); rc != 0 || !strings.Contains(out.String(), "No routed spawn in this window.") {
		t.Errorf("text report rc=%d out=%q", rc, out.String())
	}
	if rc := runReport([]string{"--bogus"}, &out, &errOut); rc != 2 {
		t.Errorf("unknown flag rc = %d, want 2", rc)
	}
	if rc := runReport([]string{"--days=x"}, &out, &errOut); rc != 2 {
		t.Errorf("bad days rc = %d, want 2", rc)
	}
}

// A routed spawn ends up in `downshift report`: the PreToolUse decision through
// the shared runner, then the PostToolUse launch and the SubagentStop usage
// through the shared end runner, joined by the tool-call id and the agent id.
func TestRoutedSpawn_EndRecordLinksThroughSharedRunners(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	t.Setenv("DOWNSHIFT_QUIET", "1")
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	midID := cmdCat.ModelFor("claude-code", core.TierMid).ID
	smallID := cmdCat.ModelFor("claude-code", core.TierSmall).ID
	setSessionAllowlist(t, "claude-code", []string{smallID, midID})

	pre := `{"hook_event_name":"PreToolUse","tool_name":"Agent","session_id":"s-ctl","tool_use_id":"toolu_ctl",` +
		`"tool_input":{"prompt":"rename the userId variable to userIdentifier","subagent_type":"general-purpose","model":"` + midID + `"}}`
	captureStdout(func() {
		captureStderr(func() {
			if rc := harnessRegistry["claude-code"].Hook(strings.NewReader(pre), cmdCat); rc != 0 {
				t.Errorf("hook rc = %d", rc)
			}
		})
	})
	post := `{"hook_event_name":"PostToolUse","tool_name":"Agent","session_id":"s-ctl","tool_use_id":"toolu_ctl",` +
		`"tool_response":{"agentId":"ag-ctl","status":"completed","resolvedModel":"` + midID + `"}}`
	stop := `{"hook_event_name":"SubagentStop","session_id":"s-ctl","agent_id":"ag-ctl","model":"` + midID + `",` +
		`"usage":{"input_tokens":1200,"output_tokens":300}}`
	for _, payload := range []string{post, stop} {
		if rc, _, _ := runEndFor("claude-code", []byte(payload)); rc != 0 {
			t.Fatalf("end rc = %d", rc)
		}
	}

	events, err := telemetry.ReadEventsFrom(eventLog)
	if err != nil {
		t.Fatal(err)
	}
	tasks := telemetry.BuildTasks(events)
	if len(tasks) != 1 || tasks[0].Group != telemetry.GroupRouted || !tasks[0].Completed() || !tasks[0].Observed {
		t.Fatalf("tasks = %+v, want one completed, priced routed task", tasks)
	}
	rep := telemetry.BuildReport(events, 1)
	if rep.Routed.Spawns != 1 || rep.Routed.Completed != 1 {
		t.Fatalf("report routed = %+v, want 1 spawn and 1 completed task", rep.Routed)
	}
}
