// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import "github.com/tiagovilasboas/downshift/internal/core"

// complexOnSmallNote is the note the orchestrator reads when a decisive COMPLEX
// task was asked to run on the small tier and nothing raised it. It names no
// task text. PreToolUse additionalContext reaches the model in Claude Code
// 2.1.316 (probe, 2026-11-01), with or without a permissionDecision.
const complexOnSmallNote = "downshift: this subagent task reads as complex, and it was requested on the small tier. " +
	"It runs as requested. If the result falls short, spawn it again on the mid tier (model: sonnet)."

// adviseOrAllow is allow, plus the note for the orchestrator when the decision
// carries advice. It sets no permissionDecision and changes no input, so the
// harness applies its normal permission flow to the unchanged call.
func adviseOrAllow(d core.Decision) Output {
	note := ""
	switch d.Advice {
	case core.AdviceComplexOnSmall:
		note = complexOnSmallNote
	case core.AdviceAboveAgentDefault:
		note = core.AgentPinNote(d.AgentPin)
	default:
		return allow()
	}
	return Output{HookSpecificOutput: &HookSpecificOutput{HookEventName: "PreToolUse", AdditionalContext: note}}
}
