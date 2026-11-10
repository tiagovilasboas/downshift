// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package claudecode adapts core routing decisions to Claude Code's hook
// protocol. Claude Code spawns subagents via the Task tool; a PreToolUse hook
// can rewrite the tool input before the subagent starts — including its model.
// This is the one place a subagent's model can be set programmatically, before
// the child process loads it.
package claudecode

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookutil"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

const harnessID = "claude-code"

// Output is the JSON we print on stdout to steer Claude Code.
type Output struct {
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
	SystemMessage      string              `json:"systemMessage,omitempty"`
}

// HookSpecificOutput carries the PreToolUse decision.
type HookSpecificOutput struct {
	HookEventName      string          `json:"hookEventName"`
	PermissionDecision string          `json:"permissionDecision,omitempty"`
	UpdatedInput       json.RawMessage `json:"updatedInput,omitempty"`
	AdditionalContext  string          `json:"additionalContext,omitempty"`
	// UpdatedToolOutput replaces a finished tool's result (PostToolUse only).
	// It must keep the tool's own output shape: Claude Code ignores a value
	// that does not match a built-in tool's schema.
	UpdatedToolOutput json.RawMessage `json:"updatedToolOutput,omitempty"`
}

// Handle processes a PreToolUse event using the catalog.Resolver injected by
// main. Falls back to the legacy core.Catalog when r is nil (tests).
// Returns the hook output, a human-readable note, and the full routing Decision
// so callers can record telemetry without re-classifying the prompt.
func Handle(ev Event, r ...core.Resolver) (Output, string, core.Decision) {
	var ti map[string]any
	if err := json.Unmarshal(ev.ToolInput, &ti); err != nil {
		return allow(), "", core.Decision{}
	}

	// Structural detection: a subagent spawn has task text in tool_input.
	// Tool name is deliberately not checked — the hook JSON matcher scopes
	// which tools reach this handler; the adapter stays harness-agnostic.
	subPrompt := hookutil.TaskText(ti, "prompt", "description")
	if subPrompt == "" {
		return allow(), "", core.Decision{}
	}

	var res core.Resolver
	if len(r) > 0 {
		res = r[0]
	}
	base := ev.resolveBaseline(ti, res)
	currentModel, fromSettings := base.Model, base.FromSettings

	session := core.ResolveSessionForID(harnessID, ev.SessionID, ev.SessionModels, ev.AvailableModels)
	session = session.WithHookQuota(ev.IncludedModels, ev.UnavailableModels)
	session = session.WithUsageQuota(harnessID, ev.UsageQuota)
	if ev.ModelMetadata != nil {
		session.Metadata = ev.ModelMetadata
	}
	res = core.ResolveSessionModels(harnessID, session, res)
	decision := core.RouteSpawn(core.Request{
		Prompt: subPrompt, Harness: harnessID, AgentType: ev.AgentTypeIdentifier(),
		ModelInherited: !base.Named, CurrentModelID: currentModel,
	}, res)
	decision.Baseline.RequestedID = currentModel
	decision.Baseline.FromSettings = fromSettings
	decision.Baseline.Guessed = fromSettings
	decision.Baseline.Pinned = base.Pinned

	plan := decision.PlanSession(core.ClaudeCodeCaps, res, session, currentModel)
	modEntry := ev.Entry == "mod"
	holdOrPreserve := plan.HoldForeign || plan.PreserveExplicit
	canRewrite := plan.RewriteModel && !holdOrPreserve && core.CanWriteSessionID(harnessID, plan.Model.ID, plan.Session, res)

	if holdOrPreserve || (!canRewrite && !modEntry) {
		return adviseOrAllow(decision), "", decision
	}

	modelRewritten := false
	if canRewrite {
		if decision.HoldGuessedDownshift(plan.Model) {
			if !modEntry {
				return allow(), "", decision
			}
		} else {
			decision.Model = plan.Model
			ti["model"] = plan.WriteName
			modelRewritten = true
		}
	}

	applyEffort := false
	if modEntry && res != nil {
		targetID := decision.Baseline.CurrentModel.ID
		if modelRewritten {
			targetID = plan.Model.ID
		} else if targetID == "" {
			targetID = currentModel
		}
		effort, effortOK, effortChanged := core.ConstrainEffort(res, harnessID, targetID, decision.Effort)
		if effortChanged {
			decision.Corrections = append(decision.Corrections, core.RuleEffortUnsupported)
		}
		effortValue := effort.String()
		requestedEffort := hookutil.StringField(ti, "reasoning_effort")
		applyEffort = effortOK && hookutil.EffortAllowed(decision, requestedEffort, effortValue, modelRewritten) && effortValue != requestedEffort
		if applyEffort {
			// The settings hook path ignores updatedInput.reasoning_effort; the mod
			// reads it for turn.step. Use the policy vocabulary turn.step accepts.
			ti["reasoning_effort"] = effortValue
		}
	}

	if !modelRewritten && !applyEffort {
		return adviseOrAllow(decision), "", decision
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
	note := ""
	if modelRewritten {
		note = decision.Summary()
	}
	if !telemetry.Quiet() && note != "" {
		out.SystemMessage = "downshift: " + note
	}
	return out, note, decision
}

// allow answers "no opinion": an empty object, no permissionDecision. An explicit
// "allow" would override the user's ask/deny rules for the spawning tool, and Codex
// rejects a bare allow without updatedInput; the harness applies its normal flow.
func allow() Output { return Output{} }
