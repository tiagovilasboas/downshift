// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Unverified: needs a real payload. These fixtures only pin the mapping from
// the names the parser reads to the shared contract.
func TestParseChildEnd(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		ok      bool
		status  telemetry.ChildStatus
		call    string
		agent   string
	}{
		{"completed", `{"hook_event_name":"subagentStop","status":"completed","tool_call_id":"tc1","subagent_id":"sa1"}`, true, telemetry.ChildCompleted, "tc1", "sa1"},
		{"error", `{"hook_event_name":"subagentStop","status":"error","tool_use_id":"tc2"}`, true, telemetry.ChildError, "tc2", "tc2"},
		{"aborted", `{"status":"aborted","tool_call_id":"tc3"}`, true, telemetry.ChildInterrupted, "tc3", "tc3"},
		{"unknown status is a stop with no claim", `{"hook_event_name":"subagentStop","status":"weird"}`, true, telemetry.ChildStopped, "", ""},
		{"other event", `{"hook_event_name":"subagentStart","status":"completed"}`, false, "", "", ""},
		{"not json", `x`, false, "", "", ""},
	}
	for _, c := range cases {
		ce, ok := cursor.ParseChildEnd([]byte(c.payload))
		if ok != c.ok {
			t.Errorf("%s: ok = %v", c.name, ok)
			continue
		}
		if !ok {
			continue
		}
		if ce.Harness != "cursor" || ce.Status != c.status || ce.ToolUseID != c.call || ce.AgentKey != c.agent ||
			ce.Source != hookport.SourcePayload || ce.Usage != nil || ce.Model != "" || ce.ResolvedModel != "" {
			t.Errorf("%s: %+v", c.name, ce)
		}
	}
}
