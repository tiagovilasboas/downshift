// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor

import (
	"encoding/json"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// ParseChildEnd is Cursor's side of the shared child-end contract. It reads a
// subagentStop payload: the status Cursor reports for the subagent and, when
// present, the id of the spawn tool call.
//
// Unverified: needs a real payload. subagentStop is a hook event in Cursor's
// hooks reference, but no payload was captured here (docs/reference/harness-matrix.md).
// The names read are status, tool_use_id/tool_call_id, subagent_id and
// session_id. The model is not read: no field is known to carry the executed
// model, and a guess would fake an honored rewrite. Cursor events carry no session id today and no
// token usage this adapter knows how to read, so a record is linked only when
// the payload names the spawn tool call, and usage stays unmeasured.
// Parsing only: records and the child-end line are the shared recorder's job.
// A stop event that names no known status is a stop with no outcome claim
// (telemetry.ChildStopped).
func ParseChildEnd(raw []byte) (hookport.ChildEnd, bool) {
	var p map[string]any
	if json.Unmarshal(raw, &p) != nil {
		return hookport.ChildEnd{}, false
	}
	if ev := hookutil.StringField(p, "hook_event_name"); ev != "" && !strings.EqualFold(ev, "subagentStop") {
		return hookport.ChildEnd{}, false
	}
	call := hookutil.StringField(p, "tool_use_id")
	if call == "" {
		call = hookutil.StringField(p, "tool_call_id")
	}
	// Cursor 3.27 subagentStop carries subagent_id only; it matches the Task
	// tool_use_id from preToolUse on the same spawn (probe 2026-10-24).
	if call == "" {
		call = hookutil.StringField(p, "subagent_id")
	}
	agent := hookutil.StringField(p, "subagent_id")
	if agent == "" {
		agent = call
	}
	sid := hookutil.StringField(p, "session_id")
	TouchFromStop(sid, hookutil.StringField(p, "model_id"), hookutil.StringField(p, "model"))
	return hookport.ChildEnd{
		Harness:   harnessID,
		AgentKey:  agent,
		ToolUseID: call,
		SessionID: sid,
		Status:    telemetry.ChildStatusFromText(hookutil.StringField(p, "status"), telemetry.ChildStopped),
		Source:    hookport.SourcePayload,
		IsChild:   true,
	}, true
}
