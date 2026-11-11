// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package kiro adapts core routing decisions to Kiro IDE's PreToolUse hook.
//
// Kiro IDE (VS Code extension) spawns subagents through invoke_sub_agent. Unlike
// Claude Code, the Kiro IDE hook protocol does NOT support updatedInput for model
// rewrite (REA 2026-11-10: kiro.dev docs, GitHub issues #6637, #4180, #7500).
//
// This adapter is observation-only: it records decisions for telemetry and can
// advise the model via stderr (exit 2), but cannot rewrite the model in place.
// This is similar to KiroCrew's policy mode.
//
// Known limitations (Kiro IDE 1.x):
//   - invoke_sub_agent payload has no model field (model is decided internally)
//   - custom agent model: field in .kiro/agents/*.md is ignored (issue #6637)
//   - updatedInput is not processed by the IDE (only Claude Code supports it)
//   - no mechanism to define model for hooks/subagents (issue #4180)
package kiro

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookutil"
)

const harnessID = "kiro"

// Event is the PreToolUse payload from Kiro IDE.
type Event struct {
	Cwd       string          `json:"cwd,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	Model     string          `json:"model,omitempty"`
	// Session allowlists (optional, may be absent in early Kiro versions).
	SessionModels   *[]string `json:"session_models,omitempty"`
	AvailableModels *[]string `json:"available_models,omitempty"`
}

// Interface compliance for the shared hook runner.
func (ev Event) ProjectDir() string            { return ev.Cwd }
func (ev Event) SessionIdentifier() string     { return ev.SessionID }
func (ev Event) CorrelationIdentifier() string { return "" }

// RequestedModel returns the spawn's model or the session model.
// Note: Kiro IDE invoke_sub_agent payload has no model field (REA 2026-11-10).
func (ev Event) RequestedModel() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ev.Model
	}
	if m := hookutil.StringField(ti, "model"); m != "" {
		return m
	}
	return ev.Model
}

// TaskText returns task content for classification. Kiro uses "prompt".
func (ev Event) TaskText() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	return hookutil.TaskText(ti, "prompt", "task", "description", "explanation")
}

// Output is the JSON printed to stdout.
// Note: Kiro IDE does NOT process hookSpecificOutput.updatedInput (REA 2026-11-10).
// This struct is kept for API compatibility but rewrite fields are not used.
type Output struct {
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput carries the permission decision.
// Note: updatedInput is NOT supported by Kiro IDE (only permissionDecision works).
type HookSpecificOutput struct {
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	// UpdatedInput is NOT processed by Kiro IDE. Kept for API compatibility.
	// To rewrite models, Kiro would need to support this field (see issue #6637).
	// UpdatedInput json.RawMessage `json:"updatedInput,omitempty"` // REMOVED
}

// Handle routes a PreToolUse event. Only invoke_sub_agent with a prompt is routed.
//
// Kiro IDE is observation-only: the adapter computes the routing decision and
// records it for telemetry, but cannot rewrite the model in place. The spawn
// runs with whatever model Kiro decided internally.
//
// The adapter returns empty output (no JSON) — Kiro IDE does not process
// hookSpecificOutput.updatedInput, and emitting permissionDecision without
// a rewrite would fail harness conformance tests.
func Handle(ev Event, r ...core.Resolver) (Output, string, core.Decision) {
	if ev.ToolName != "invoke_sub_agent" {
		return allow(), "", core.Decision{}
	}
	var ti map[string]any
	if err := json.Unmarshal(ev.ToolInput, &ti); err != nil {
		return allow(), "", core.Decision{}
	}
	taskText := hookutil.TaskText(ti, "prompt", "task", "description", "explanation")
	if taskText == "" {
		return allow(), "", core.Decision{}
	}

	// Read the requested model (usually empty in Kiro IDE).
	currentModel := hookutil.StringField(ti, "model")
	if currentModel == "" {
		currentModel = ev.Model
	}

	var res core.Resolver
	if len(r) > 0 {
		res = r[0]
	}

	session := core.ResolveSession(harnessID, ev.SessionModels, ev.AvailableModels)
	// Observation-only: Kiro cannot apply the decision, so it is computed for
	// telemetry and no note goes back to the runner (outcome allow).
	decision := core.ObserveSpawn(core.Request{
		Prompt:         taskText,
		Harness:        harnessID,
		AgentType:      hookutil.StringField(ti, "name"),
		ModelInherited: hookutil.StringField(ti, "model") == "",
		CurrentModelID: currentModel,
	}, core.KiroCaps, session, res)

	// Return empty output and no note: Kiro IDE is observation-only, so we only
	// record telemetry. The shared runner will record OutcomeAllow because note
	// is empty. No permissionDecision is emitted (harness conformance requires
	// that unchanged spawns emit no permissionDecision).
	return allow(), "", decision
}

func allow() Output { return Output{} }
