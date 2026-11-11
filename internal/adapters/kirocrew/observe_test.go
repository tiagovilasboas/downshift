// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kirocrew_test

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/kirocrew"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// The kiro-cli 3.x subagent spawn names no model, so it is observed: a
// decision is recorded for telemetry and the spawn is never blocked. Shape from
// docs/evidence/kirocrew-hook-payloads-2026-11-11.md.
func TestHandleObservesModelLessSubagent(t *testing.T) {
	writeSessionFile(t, []string{smallID, midID, frontierID})
	cases := []struct {
		name, tool, input string
		wantDecision      bool
	}{
		{"subagent with prompt", "subagent", `{"subAgentName":"helper","prompt":"rename the variable foo to bar in one file","explanation":"small edit"}`, true},
		{"subagent with only explanation", "subagent", `{"subAgentName":"helper","explanation":"list the files"}`, true},
		{"subagent without text", "subagent", `{"subAgentName":"helper"}`, false},
		{"unrelated tool", "execute_bash", `{"command":"ls"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := kirocrew.Event{HookEventName: "PreToolUse", ToolName: tc.tool, ToolInput: json.RawMessage(tc.input)}
			out, note, d := kirocrew.Handle(ev, cat)
			if out.Block || out.Message != "" || note != "" {
				t.Fatalf("observed spawn must allow with no note: block=%v msg=%q note=%q", out.Block, out.Message, note)
			}
			if got := d.Harness != ""; got != tc.wantDecision {
				t.Fatalf("decision recorded = %v, want %v", got, tc.wantDecision)
			}
			if tc.wantDecision {
				if d.Harness != "kirocrew" || d.Note.Reason != "observation-only" {
					t.Fatalf("decision = harness %q reason %q", d.Harness, d.Note.Reason)
				}
			}
		})
	}
}

func TestParseChildEndModelLessSubagent(t *testing.T) {
	payload := `{"session_id":"s","hook_event_name":"PostToolUse","cwd":"/x","tool_name":"subagent","tool_input":{"subAgentName":"helper","prompt":"p"},"tool_response":"Output:\n\nok"}`
	ce, ok := kirocrew.ParseChildEnd([]byte(payload))
	if !ok {
		t.Fatal("subagent postToolUse must parse")
	}
	if ce.Status != telemetry.ChildLaunched || ce.Model != "" || ce.Harness != "kirocrew" {
		t.Fatalf("child end = status %v model %q harness %q", ce.Status, ce.Model, ce.Harness)
	}
}
