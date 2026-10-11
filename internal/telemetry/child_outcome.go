// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

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
