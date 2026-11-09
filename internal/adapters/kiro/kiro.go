// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package kiro adapts core routing decisions to Kiro IDE's PreToolUse hook.
//
// Kiro IDE (VS Code extension) spawns subagents through invoke_sub_agent. Its
// hook protocol supports JSON rewrite via hookSpecificOutput.updatedInput,
// like Codex. This is distinct from KiroCrew (daemon), which is policy-only.
package kiro

import (
	"encoding/json"
	"fmt"

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
type Output struct {
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput carries the rewrite decision.
type HookSpecificOutput struct {
	PermissionDecision       string          `json:"permissionDecision"`
	PermissionDecisionReason string          `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             json.RawMessage `json:"updatedInput,omitempty"`
}

// Handle routes a PreToolUse event. Only invoke_sub_agent with a prompt is routed.
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

	currentModel := hookutil.StringField(ti, "model")
	if currentModel == "" {
		currentModel = ev.Model
	}

	var res core.Resolver
	if len(r) > 0 {
		res = r[0]
	}

	session := core.ResolveSession(harnessID, ev.SessionModels, ev.AvailableModels)
	res = core.ResolveSessionModels(harnessID, session, res)

	decision := core.RouteSpawn(core.Request{
		Prompt:         taskText,
		Harness:        harnessID,
		AgentType:      hookutil.StringField(ti, "name"),
		ModelInherited: hookutil.StringField(ti, "model") == "",
		CurrentModelID: currentModel,
	}, res)
	decision.Baseline.RequestedID = currentModel

	plan := decision.PlanSession(core.KiroCaps, res, session, currentModel)
	if plan.HoldForeign || plan.PreserveExplicit || !plan.RewriteModel {
		return allow(), "", decision
	}
	if !core.CanWriteSessionID(harnessID, plan.Model.ID, plan.Session, res) {
		return allow(), "", decision
	}

	decision.Model = plan.Model
	ti["model"] = plan.WriteName
	note := decision.Summary()

	updated, err := json.Marshal(ti)
	if err != nil {
		return allow(), "", decision
	}
	return Output{HookSpecificOutput: &HookSpecificOutput{
		PermissionDecision:       "allow",
		PermissionDecisionReason: fmt.Sprintf("downshift: %s", note),
		UpdatedInput:             updated,
	}}, note, decision
}

func allow() Output { return Output{} }
