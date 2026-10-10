// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package cursor adapts core routing decisions to Cursor's hook protocol.
// Cursor exposes a preToolUse hook whose output supports updated_input —
// the same interception point Claude Code uses. When the agent is about to
// spawn a subagent via the Task tool, we rewrite the tool input's model
// before the child starts.
package cursor

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
	"github.com/tiagovilasboas/downshift/internal/quota"
)

const harnessID = "cursor"

// Event is the JSON Cursor sends on stdin for a preToolUse hook.
type Event struct {
	HookEventName string          `json:"hook_event_name"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
	Model         string          `json:"model"`
	ModelID       string          `json:"model_id"`
	// SessionModels and AvailableModels are optional hook allowlists.
	// Nil means the payload did not include a list. A non-nil slice is
	// the session and skips the user file. Cursor's preToolUse payload
	// does not send either field today.
	SessionModels   *[]string `json:"session_models,omitempty"`
	AvailableModels *[]string `json:"available_models,omitempty"`
	// IncludedModels and UnavailableModels are optional quota marks.
	// Absent marks supply no credit evidence. Present marks apply to this call.
	IncludedModels    *[]string       `json:"included_models,omitempty"`
	UnavailableModels *[]string       `json:"unavailable_models,omitempty"`
	UsageQuota        *quota.Snapshot `json:"usage_quota,omitempty"`
}

// CorrelationIdentifier returns the optional opaque ID supplied by a caller.
func (ev Event) CorrelationIdentifier() string { return ev.CorrelationID }

// RequestedReasoningEffort reads an optional input field for future-compatible telemetry.
func (ev Event) RequestedReasoningEffort() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	return hookutil.StringField(ti, "reasoning_effort")
}

// RequestedModel returns the model received before Downshift emitted a rewrite.
func (ev Event) RequestedModel() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) == nil {
		if model := hookutil.StringField(ti, "model"); model != "" {
			return model
		}
	}
	if ev.ModelID != "" {
		return ev.ModelID
	}
	return ev.Model
}

// TaskText returns the task content used for local, prompt-free feature
// extraction by the shared engineering loop.
func (ev Event) TaskText() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	return hookutil.TaskText(ti, "task", "prompt", "description")
}

// Output is the JSON we print on stdout to steer Cursor's preToolUse.
type Output struct {
	Permission   string          `json:"permission"`
	UpdatedInput json.RawMessage `json:"updated_input,omitempty"`
	AgentMessage string          `json:"agent_message,omitempty"`
}

// Handle processes a preToolUse event using the catalog.Resolver injected by
// main. Falls back to the legacy core.Catalog when r is nil (tests).
// Returns the hook output, a human-readable note, and the full routing Decision
// so callers can record telemetry without re-classifying the prompt.
func Handle(ev Event, r ...core.Resolver) (Output, string, core.Decision) {
	var ti map[string]any
	if err := json.Unmarshal(ev.ToolInput, &ti); err != nil {
		return allow(), "", core.Decision{}
	}

	// Structural detection: a subagent spawn carries task text in tool_input.
	// Tool name is deliberately not checked.
	subPrompt := hookutil.TaskText(ti, "task", "prompt", "description")
	if subPrompt == "" {
		return allow(), "", core.Decision{}
	}

	currentModel := hookutil.StringField(ti, "model")
	if currentModel == "" {
		currentModel = ev.ModelID
	}
	if currentModel == "" {
		currentModel = ev.Model
	}

	var res core.Resolver
	if len(r) > 0 {
		res = r[0]
	}
	decision := core.Route(subPrompt, harnessID, currentModel, res)
	decision.RequestedID = currentModel

	session := core.ResolveSession(harnessID, ev.SessionModels, ev.AvailableModels)
	session = session.WithHookQuota(ev.IncludedModels, ev.UnavailableModels)
	session = session.WithUsageQuota(harnessID, ev.UsageQuota)
	decision.SessionUnknown = !session.Known
	decision.QuotaStatus = string(session.QuotaStatus(harnessID, currentModel))
	if session.Usage != nil {
		decision.QuotaSource = quota.SourceName(session.Usage.Source)
	}
	plan := decision.PlanForSession(core.CursorCaps, res, session)
	decision.CreditHeld = plan.CreditHeld
	if plan.RewriteModel {
		decision.QuotaStatus = string(session.QuotaStatus(harnessID, plan.Model.ID))
	}
	if plan.HoldForeign || plan.PreserveExplicit {
		return allow(), "", decision
	}

	target := plan.Model
	note := decision.Summary()
	if target.ID == "" || target.ID == decision.CurrentModel.ID {
		return allow(), "", decision
	}
	if !plan.RewriteModel {
		return allow(), "", decision
	}
	if !core.CanWriteSessionID(harnessID, target.ID, session, res) {
		return allow(), "", decision
	}
	writeName, ok := cursorTaskModel(plan.WriteName, res)
	if !ok {
		// The selected id is not a slug this Cursor build accepts, and the
		// catalog has no replacement. Leave the spawn unchanged.
		return allow(), "", decision
	}
	if writeName != plan.WriteName {
		target.ID = writeName
		target.Native = ""
	}
	decision.Model = target
	note = decision.Summary()

	ti["model"] = writeName
	updated, err := json.Marshal(ti)
	if err != nil {
		return allow(), "", decision
	}

	return Output{
		Permission:   "allow",
		UpdatedInput: updated,
		AgentMessage: "downshift: " + note,
	}, note, decision
}

func allow() Output {
	return Output{Permission: "allow"}
}

// rejectedCursorSubagentModel is a catalog alias Cursor's Task tool rejects
// on build cd6d2a1f2e56e9841f0ed9c7c24542087b4e69b0. The accepted slug is the
// catalog id of the Cursor muse-spark entry.
const rejectedCursorSubagentModel = "muse-spark-1.3-high"

// cursorTaskModel is the string written into tool_input.model. A session may
// still carry the rejected alias; the hook writes the catalog id instead.
// When that id is itself rejected, the rewrite is held.
func cursorTaskModel(writeName string, res core.Resolver) (string, bool) {
	if writeName == "" {
		return "", false
	}
	if res == nil {
		if writeName == rejectedCursorSubagentModel {
			return "", false
		}
		return writeName, true
	}
	m, ok := res.LookupByID(harnessID, writeName)
	if !ok || m.WriteName() == "" {
		if writeName == rejectedCursorSubagentModel {
			return "", false
		}
		return writeName, true
	}
	canonical := m.WriteName()
	if canonical == rejectedCursorSubagentModel {
		return "", false
	}
	if writeName == rejectedCursorSubagentModel {
		return canonical, true
	}
	return writeName, true
}
