// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

// Package codex adapts core routing decisions to OpenAI Codex CLI's hook
// protocol under multi_agent_v2.
//
// Codex spawns subagents through a reserved spawn_agent tool. A PreToolUse
// hook can inject the model and reasoning_effort into the tool input via
// hookSpecificOutput.updatedInput before the local spawn handler creates
// the child.
//
// reasoning_effort is translated from core.Effort via the catalog's
// effort_map for the target model — no hardcoded switch in this adapter.
package codex

import (
	"encoding/json"
	"strings"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/hookutil"
)

const harnessID = "codex"

// Event is the JSON Codex sends on stdin for a PreToolUse hook.
type Event struct {
	HookEventName string          `json:"hook_event_name"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	SessionID     string          `json:"session_id"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
	Model         string          `json:"model"`
	// Optional allowlist. Nil means the payload did not include one.
	// Codex PreToolUse does not send this field today.
	SessionModels   *[]string `json:"session_models,omitempty"`
	AvailableModels *[]string `json:"available_models,omitempty"`
	// IncludedModels and UnavailableModels are optional quota marks.
	// Nil leaves the user file. A non-nil slice replaces it for this call.
	IncludedModels    *[]string `json:"included_models,omitempty"`
	UnavailableModels *[]string `json:"unavailable_models,omitempty"`
}

// SessionIdentifier exposes Codex's stable session ID to the shared hook
// runner without coupling core routing decisions to harness metadata.
func (ev Event) SessionIdentifier() string { return ev.SessionID }

// CorrelationIdentifier returns the optional opaque ID supplied by a caller.
func (ev Event) CorrelationIdentifier() string { return ev.CorrelationID }

// RequestedReasoningEffort reads an optional hook input field for telemetry.
func (ev Event) RequestedReasoningEffort() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	return hookutil.StringField(ti, "reasoning_effort")
}

// RequestedModel returns the original model field, never a routing
// recommendation. Tool input takes precedence because it is what the child
// request actually carried before this hook rewrote it.
func (ev Event) RequestedModel() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) == nil {
		if model := hookutil.StringField(ti, "model"); model != "" {
			return model
		}
	}
	return ev.Model
}

// TaskText returns task content for local feature extraction. Raw text is
// never returned from the shared loop's persistence layer.
func (ev Event) TaskText() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	message := hookutil.StringField(ti, "message")
	if taskName := hookutil.StringField(ti, "task_name"); taskName != "" {
		message = strings.TrimSpace(message + " " + taskName)
	}
	return message
}

// Output is the JSON we print on stdout to steer Codex's PreToolUse.
type Output struct {
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput carries the PreToolUse decision.
type HookSpecificOutput struct {
	HookEventName            string          `json:"hookEventName"`
	PermissionDecision       string          `json:"permissionDecision"`
	PermissionDecisionReason string          `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             json.RawMessage `json:"updatedInput,omitempty"`
}

// Handle processes a PreToolUse event. Accepts an optional catalog.Resolver
// for model lookup and effort translation. Falls back to legacy core.Catalog
// when nil (tests that don't inject a resolver).
// Returns the hook output, a human-readable note, and the full routing Decision
// so callers can record telemetry without re-classifying the prompt.
func Handle(ev Event, r ...core.Resolver) (Output, string, core.Decision) {
	if !isSpawnTool(ev.ToolName) {
		return allow(), "", core.Decision{}
	}

	var ti map[string]any
	if err := json.Unmarshal(ev.ToolInput, &ti); err != nil {
		return allow(), "", core.Decision{}
	}

	subPrompt := hookutil.StringField(ti, "message")
	if tn := hookutil.StringField(ti, "task_name"); tn != "" {
		subPrompt = strings.TrimSpace(subPrompt + " " + tn)
	}
	if subPrompt == "" {
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
	decision := core.Route(subPrompt, harnessID, currentModel, res)
	decision.RequestedID = currentModel

	session := core.ResolveSessionForID(harnessID, ev.SessionID, ev.SessionModels, ev.AvailableModels)
	session = session.WithHookQuota(ev.IncludedModels, ev.UnavailableModels)
	decision.SessionUnknown = !session.Known
	plan := decision.PlanForSession(core.CodexCaps, res, session)
	if plan.HoldForeign || plan.PreserveExplicit || (!plan.RewriteModel && !plan.ApplyEffort) {
		return allow(), "", decision
	}
	if plan.RewriteModel && !core.CanWriteCatalogID(harnessID, plan.Model.ID, session, res) {
		return allow(), "", decision
	}
	if plan.RewriteModel {
		decision.Model = plan.Model
	}

	effortValue := decision.Effort.String()
	if res != nil {
		effortValue = res.EffortFor(harnessID, plan.Model.ID, decision.Effort)
	}

	if plan.RewriteModel {
		ti["model"] = plan.Model.ID
	}
	if plan.ApplyEffort {
		ti["reasoning_effort"] = effortValue
	}
	updated, err := json.Marshal(ti)
	if err != nil {
		return allow(), "", decision
	}

	out := Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:      "PreToolUse",
			PermissionDecision: "allow",
			UpdatedInput:       updated,
		},
	}
	return out, decision.Summary(), decision
}

func isSpawnTool(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return false
	}
	return n == "agent" || n == "spawn_agent" || strings.HasSuffix(n, "spawn_agent")
}

func allow() Output {
	return Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:      "PreToolUse",
			PermissionDecision: "allow",
		},
	}
}
