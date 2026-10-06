// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package claudecode — lifecycle adapter (DS-04)
//
// ParseLifecycleEvent maps a Claude Code SubagentStart or SubagentStop hook
// payload (stdin JSON) to a lifecycleobserver.Event for the DS-04 POC.
//
// Documented source fields (Claude Code hooks reference, Sep 2026):
//
//	SubagentStart: session_id, turn_id, agent_id, agent_type
//	SubagentStop:  session_id, turn_id, agent_id, agent_type
//
// Fields NOT read here (absent from documented lifecycle contract):
//
//	prompt, messages, transcript, model, reasoning_effort, tool_input,
//	correlation_id — none of these are part of the SubagentStart/Stop schema.
//
// This adapter is disabled-by-default. EnableLifecycleObserver must be true,
// supplied with caller-owned HMAC key material, before any observation is
// emitted. See lifecycleobserver.Config.
package claudecode

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift/internal/lifecycleobserver"
)

// LifecycleEvent is the JSON Claude Code sends on stdin for SubagentStart and
// SubagentStop hooks. Only documented lifecycle-correlation fields are decoded;
// all other fields are intentionally ignored.
type LifecycleEvent struct {
	HookEventName string `json:"hook_event_name"`
	SessionID     string `json:"session_id"`
	TurnID        string `json:"turn_id"`
	AgentID       string `json:"agent_id"`
	AgentType     string `json:"agent_type"`
}

// ParseLifecycleEvent decodes a raw Claude Code SubagentStart or SubagentStop
// payload and returns a lifecycleobserver.Event.
//
// Validation rules (mirrors the DS-04 contract):
//   - hook_event_name must be "SubagentStart" or "SubagentStop".
//   - session_id and turn_id must be non-empty.
//   - agent_id and agent_type must be non-empty for SubagentStart/Stop.
//   - Any missing required field returns (zero-value Event, false).
//
// The function never reads prompt, messages, transcript, model, tool_input, or
// any field outside the documented lifecycle-hook schema.
func ParseLifecycleEvent(raw []byte) (lifecycleobserver.Event, bool) {
	var ev LifecycleEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		return lifecycleobserver.Event{}, false
	}
	switch ev.HookEventName {
	case "SubagentStart", "SubagentStop":
		// require all four correlation fields
		if ev.SessionID == "" || ev.TurnID == "" || ev.AgentID == "" || ev.AgentType == "" {
			return lifecycleobserver.Event{}, false
		}
	default:
		// not a lifecycle event this adapter handles
		return lifecycleobserver.Event{}, false
	}
	return lifecycleobserver.Event{
		HookEventName: ev.HookEventName,
		SessionID:     ev.SessionID,
		TurnID:        ev.TurnID,
		AgentID:       ev.AgentID,
		AgentType:     ev.AgentType,
	}, true
}
