// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kiro

import (
	"encoding/json"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/hookutil"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// ParseChildEnd is Kiro IDE's side of the shared child-end contract. It reads
// the PostToolUse payload of an invoke_sub_agent call: the tool_response status
// and child model.
//
// Kiro hook docs (https://kiro.dev/docs/hooks/types/): PostToolUse "triggers
// after the agent has invoked a tool, with access to tool results." The
// documented input fields for PreToolUse are hook_event_name, cwd, session_id,
// tool_name, tool_input. PostToolUse adds tool_response but the exact schema
// (including nested fields) is NOT confirmed in vendor docs.
//
// Unverified (N5): no real Kiro IDE PostToolUse payload for invoke_sub_agent
// was captured (docs/reference/harness-matrix.md). The assumed fields follow
// the PreToolUse pattern plus tool_response. A spawn call that reports no
// status, or one outside the shared vocabulary, is a launch
// (telemetry.ChildLaunched), not an end.
//
// Parsing only: records, pricing and the child-end line are the shared
// recorder's job (internal/childend).
func ParseChildEnd(raw []byte) (hookport.ChildEnd, bool) {
	var p struct {
		HookEventName string          `json:"hook_event_name"`
		SessionID     string          `json:"session_id,omitempty"`
		Cwd           string          `json:"cwd,omitempty"`
		ToolName      string          `json:"tool_name"`
		ToolUseID     string          `json:"tool_use_id,omitempty"`
		ToolInput     json.RawMessage `json:"tool_input"`
		ToolResponse  json.RawMessage `json:"tool_response,omitempty"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return hookport.ChildEnd{}, false
	}
	// Accept PascalCase event name (Kiro uses PascalCase).
	if p.HookEventName != "" && p.HookEventName != "PostToolUse" {
		return hookport.ChildEnd{}, false
	}
	// Match invoke_sub_agent (the Kiro IDE subagent spawn tool).
	if !strings.Contains(strings.ToLower(p.ToolName), "invoke_sub_agent") && !strings.Contains(strings.ToLower(p.ToolName), "sub_agent") {
		return hookport.ChildEnd{}, false
	}

	// Read model from tool_input (the requested model for the spawn).
	var input map[string]any
	_ = json.Unmarshal(p.ToolInput, &input)
	requestedModel := hookutil.StringField(input, "model")

	// Read status and usage from tool_response.
	var resp map[string]any
	_ = json.Unmarshal(p.ToolResponse, &resp)

	status := telemetry.ChildStatusFromText(hookutil.StringField(resp, "status"), telemetry.ChildLaunched)
	// Also check for outcome field (alternative name used by some tools).
	if status == telemetry.ChildLaunched {
		status = telemetry.ChildStatusFromText(hookutil.StringField(resp, "outcome"), telemetry.ChildLaunched)
	}

	// ResolvedModel stays empty: no Kiro payload field names the model that
	// actually ran (tool_input.model is the request, not evidence of the served
	// model). Copying it here would record a served model without evidence.
	ce := hookport.ChildEnd{
		Harness:   harnessID,
		AgentKey:  p.ToolUseID,
		ToolUseID: p.ToolUseID,
		SessionID: p.SessionID,
		Model:     requestedModel,
		Status:    status,
		Source:    hookport.SourcePayload,
		IsChild:   true,
	}

	// Try to read usage from tool_response if present.
	// Format is assumed: {"usage": {"input_tokens": N, "output_tokens": N}}
	if usageRaw, ok := resp["usage"].(map[string]any); ok {
		in, _ := usageRaw["input_tokens"].(float64)
		out, _ := usageRaw["output_tokens"].(float64)
		cached, _ := usageRaw["cached_tokens"].(float64)
		if in > 0 || out > 0 {
			ce.Usage = &telemetry.TokenUsage{
				InputTokens:  int64(in),
				OutputTokens: int64(out),
				CachedTokens: int64(cached),
			}
		}
	}

	return ce, true
}
