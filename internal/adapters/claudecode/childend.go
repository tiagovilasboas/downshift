// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// ParseChildEnd is Claude Code's side of the shared child-end contract. It
// reads a PostToolUse or PostToolUseFailure payload (a spawn launch or end)
// or a SubagentStop payload (the end: inline usage from the mod's
// turn.complete path, or the subagent's own transcript). Parsing only:
// records, pricing and the child-end line are the shared recorder's job
// (internal/childend).
func ParseChildEnd(raw []byte) (hookport.ChildEnd, bool) {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return hookport.ChildEnd{}, false
	}
	if stringFieldAny(payload, "hook_event_name") == "SubagentStop" {
		return parseSubagentStop(payload)
	}
	return parsePostToolUse(payload)
}

func isAgentTool(payload map[string]any) bool {
	tool := stringFieldAny(payload, "tool_name")
	return tool == "Agent" || tool == "Task"
}

// parsePostToolUse reads the documented PostToolUse fields: the model the
// harness chose for the child (tool_response.resolvedModel), how the call
// ended (event name, tool_response.status, is_interrupt; the error text is
// never read) and any usage the payload itself reports. ok is false when the
// payload is neither a subagent call nor carries a model or usage.
func parsePostToolUse(payload map[string]any) (hookport.ChildEnd, bool) {
	agent := isAgentTool(payload)
	resolved := resolvedModel(payload)
	model := stringFieldAny(payload, "model", "model_id")
	usage := telemetry.TokenUsageFromPayload(payload)
	if !agent && resolved == "" && usage == (telemetry.TokenUsage{}) {
		return hookport.ChildEnd{}, false
	}
	ce := hookport.ChildEnd{
		Harness:       harnessID,
		AgentKey:      agentID(payload),
		ToolUseID:     stringFieldAny(payload, "tool_use_id", "toolUseId"),
		SessionID:     stringFieldAny(payload, "session_id", "sessionId", "sessionID"),
		Model:         model,
		ResolvedModel: resolved,
		ServedModel:   firstAssistantModel(stringFieldAny(payload, "agent_transcript_path")),
		Source:        hookport.SourcePayload,
		IsChild:       agent,
	}
	if usage != (telemetry.TokenUsage{}) {
		ce.Usage = &usage
	}
	if agent {
		toolStatus := ""
		if resp, ok := payload["tool_response"].(map[string]any); ok {
			toolStatus, _ = resp["status"].(string)
		}
		isInterrupt, _ := payload["is_interrupt"].(bool)
		ce.Status = telemetry.ClassifyChildEnd(stringFieldAny(payload, "hook_event_name"), toolStatus, isInterrupt)
	}
	return ce, true
}
