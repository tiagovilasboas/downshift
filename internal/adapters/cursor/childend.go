// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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
	agent := hookutil.StringField(p, "subagent_id")
	if agent == "" {
		agent = call
	}
	return hookport.ChildEnd{
		Harness:   harnessID,
		AgentKey:  agent,
		ToolUseID: call,
		SessionID: hookutil.StringField(p, "session_id"),
		Status:    childStatus(hookutil.StringField(p, "status")),
		Source:    hookport.SourcePayload,
		IsChild:   true,
	}, true
}

// childStatus maps Cursor's subagent status to a ChildStatus. A stop event
// that names no known status is a stop with no outcome claim.
func childStatus(s string) telemetry.ChildStatus {
	switch strings.ToLower(s) {
	case "completed", "complete", "success":
		return telemetry.ChildCompleted
	case "error", "failed":
		return telemetry.ChildError
	case "aborted", "cancelled", "canceled", "interrupted":
		return telemetry.ChildInterrupted
	}
	return telemetry.ChildStopped
}
