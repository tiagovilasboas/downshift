// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func childEndEvents(t *testing.T) []telemetry.Event {
	t.Helper()
	all, _ := telemetry.ReadEvents()
	var out []telemetry.Event
	for _, ev := range all {
		if ev.Outcome == telemetry.OutcomeChildEnded {
			out = append(out, ev)
		}
	}
	return out
}

func TestChildEnd_StatusesFromDocumentedFields(t *testing.T) {
	cases := map[string]map[string]any{
		"error": {"hook_event_name": "PostToolUseFailure", "tool_name": "Agent", "session_id": "sess-stop",
			"error": "Exit code 1\nboom: a secret token appears here", "is_interrupt": false},
		"interrupted": {"hook_event_name": "PostToolUseFailure", "tool_name": "Agent", "session_id": "sess-stop",
			"error": "aborted", "is_interrupt": true},
		"completed": {"hook_event_name": "PostToolUse", "tool_name": "Agent", "session_id": "sess-stop",
			"tool_response": map[string]any{"status": "completed"}},
	}
	for want, payload := range cases {
		log, small, frontier, res := stopSetup(t)
		seedLaunch(t, log, small, frontier, res)
		raw, _ := json.Marshal(payload)
		claudecode.HandlePostToolUse(raw, "test", res)
		ev := childEndEvents(t)
		if len(ev) != 1 || string(ev[0].ChildStatus) != want {
			t.Fatalf("%s: got %+v", want, ev)
		}
		b, _ := json.Marshal(ev[0])
		if strings.Contains(string(b), "secret token") || strings.Contains(string(b), "boom") {
			t.Fatalf("%s: error text leaked into telemetry: %s", want, b)
		}
	}
}

func TestChildEnd_IgnoresNonAgentToolsAndDoesNotCountAsDecision(t *testing.T) {
	log, small, frontier, res := stopSetup(t)
	seedLaunch(t, log, small, frontier, res)
	bash, _ := json.Marshal(map[string]any{"hook_event_name": "PostToolUseFailure", "tool_name": "Bash", "session_id": "sess-stop"})
	claudecode.HandlePostToolUse(bash, "test", res)
	if n := len(childEndEvents(t)); n != 0 {
		t.Fatalf("non-Agent tool recorded %d child events", n)
	}
	fail, _ := json.Marshal(map[string]any{"hook_event_name": "PostToolUseFailure", "tool_name": "Agent", "session_id": "sess-stop", "error": "x"})
	claudecode.HandlePostToolUse(fail, "test", res)
	all, _ := telemetry.ReadEvents()
	stats := telemetry.Aggregate(all)
	if stats.Total != 1 {
		t.Fatalf("child_ended moved the decision count: Total=%d", stats.Total)
	}
	if stats.ChildStatus[telemetry.ChildError] != 1 {
		t.Fatalf("child status not aggregated: %+v", stats.ChildStatus)
	}
}
