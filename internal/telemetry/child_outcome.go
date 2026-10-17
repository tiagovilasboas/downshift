// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "strings"

// OutcomeChildEnded records how a spawned subagent ended. It is an observation
// about an earlier decision: it never counts as a decision, cost or savings.
const OutcomeChildEnded = "child_ended"

// ChildStatus is a closed, prompt-free vocabulary for how a child ended. Only
// values the harness documents are produced; everything else is unknown.
type ChildStatus string

const (
	ChildCompleted   ChildStatus = "completed"   // foreground tool finished
	ChildLaunched    ChildStatus = "launched"    // background launch; NOT a success
	ChildInterrupted ChildStatus = "interrupted" // aborted before finishing
	ChildError       ChildStatus = "error"       // failed while running
	ChildStopped     ChildStatus = "stopped"     // ended; no outcome claim
	ChildUnknown     ChildStatus = "unknown"
)

// ClassifyChildEnd maps the documented hook fields to a ChildStatus. event is
// hook_event_name. toolStatus is tool_response.status for PostToolUse. isInterrupt
// is the PostToolUseFailure is_interrupt flag. The error text is never read, so
// no tool output reaches telemetry.
func ClassifyChildEnd(event, toolStatus string, isInterrupt bool) ChildStatus {
	switch event {
	case "PostToolUse":
		switch toolStatus {
		case "completed":
			return ChildCompleted
		case "async_launched":
			return ChildLaunched
		}
	case "PostToolUseFailure":
		if isInterrupt {
			return ChildInterrupted
		}
		return ChildError
	case "SubagentStop":
		return ChildStopped
	}
	return ChildUnknown
}

// childStatusSynonyms is the vocabulary harnesses use for a child's status in
// a payload field (Cursor subagentStop.status, Codex tool_response.status).
// Both payload shapes are unverified, so the table is the union of the names
// read so far; matching is case-insensitive.
var childStatusSynonyms = map[string]ChildStatus{
	"completed": ChildCompleted, "complete": ChildCompleted, "succeeded": ChildCompleted, "success": ChildCompleted, "done": ChildCompleted,
	"error": ChildError, "errored": ChildError, "failed": ChildError, "failure": ChildError,
	"interrupted": ChildInterrupted, "aborted": ChildInterrupted, "cancelled": ChildInterrupted, "canceled": ChildInterrupted,
}

// ChildStatusFromText maps a status string a harness payload reports to a
// ChildStatus through childStatusSynonyms. A name not in the table, including
// an empty one, is fallback: what an unqualified event means is a harness fact
// (a Cursor subagentStop is a stop with no outcome claim, ChildStopped; a
// Codex spawn_agent call that returned without a status is a launch,
// ChildLaunched), so the adapter passes it in.
func ChildStatusFromText(status string, fallback ChildStatus) ChildStatus {
	if st, ok := childStatusSynonyms[strings.ToLower(status)]; ok {
		return st
	}
	return fallback
}
