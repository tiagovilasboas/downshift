// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/mainwork"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// observeMainWork records one main-session tool call (tool, category code and
// hashed session) with a single append. Nothing is read: the session model's
// tier is resolved at report time. A tool the capabilities do not list, or a
// subagent call, returns first.
func observeMainWork(payload map[string]any) {
	tool := stringFieldAny(payload, "tool_name", "toolName")
	if tool == "" {
		return
	}
	if _, listed := core.ClaudeCodeCaps.MainToolCategories[tool]; !listed {
		return
	}
	transcript := stringFieldAny(payload, "transcript_path", "transcriptPath")
	if isSubagentCall(payload, transcript) {
		return
	}
	session := stringFieldAny(payload, "session_id", "sessionId", "sessionID")
	if session == "" {
		session = sessionFromTranscript(transcript)
	}
	mainwork.Observe(mainwork.Call{Harness: harnessID, Tool: tool, Session: telemetry.HashSessionID(session), ResultBytes: toolResultBytes(payload)}, core.ClaudeCodeCaps)
}

// toolResultBytes is the total length of the string values in tool_response,
// the text Claude Code hands the model; keys and numbers are not counted. 0
// means the response is absent or holds no text. Read's response shape is not
// documented, so any nesting is walked. Only the length leaves this function.
func toolResultBytes(payload map[string]any) int {
	return stringBytes(payload["tool_response"])
}

func stringBytes(v any) int {
	n := 0
	switch t := v.(type) {
	case string:
		return len(t)
	case []any:
		for _, e := range t {
			n += stringBytes(e)
		}
	case map[string]any:
		for _, e := range t {
			n += stringBytes(e)
		}
	}
	return n
}

// isSubagentCall: Claude Code adds a top-level agent_id (and agent_type) to the
// hook input of a call made inside a subagent. A transcript under a
// "subagents" directory is the same call seen through its file.
func isSubagentCall(payload map[string]any, transcript string) bool {
	if stringFieldAny(payload, "agent_id", "agentId") != "" {
		return true
	}
	return strings.Contains(strings.ReplaceAll(transcript, `\`, "/"), "/subagents/")
}
