// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package cursor adapts core routing decisions to Cursor's hook protocol.
// Cursor exposes a preToolUse hook whose output supports updated_input —
// the same interception point Claude Code uses. When the agent is about to
// spawn a subagent via the Task tool, we rewrite the tool input's model
// before the child starts.
package cursor

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookutil"
	"github.com/tiagovilasboas/downshift-ai/internal/quota"
)

const harnessID = "cursor"

// Event is the JSON Cursor sends on stdin for a preToolUse hook.
type Event struct {
	HookEventName string `json:"hook_event_name"`
	// Cwd is the working directory, used to find a project policy file.
	Cwd string `json:"cwd,omitempty"`
	// WorkspaceRoots is reported by Cursor hooks; cwd is documented but reported
	// empty in places, so it is the fallback source for the project directory.
	WorkspaceRoots []string `json:"workspace_roots,omitempty"`
	CorrelationID  string   `json:"correlation_id,omitempty"`
	// SessionID is Cursor's conversation/session id on preToolUse (3.27+).
	SessionID      string `json:"session_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	// ToolUseID is the harness id of this tool call (documented in its hook payload).
	ToolUseID string          `json:"tool_use_id,omitempty"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	Model     string          `json:"model"`
	ModelID   string          `json:"model_id"`
	// SessionModels and AvailableModels are optional hook allowlists.
	// Nil means the payload did not include a list. A non-nil slice is
	// the session and skips the user file. Cursor's preToolUse payload
	// does not send either field today.
	ModelMetadata   []core.SessionModel `json:"model_metadata,omitempty"`
	SessionModels   *[]string           `json:"session_models,omitempty"`
	AvailableModels *[]string           `json:"available_models,omitempty"`
	// IncludedModels and UnavailableModels are optional quota marks.
	// Absent marks supply no credit evidence. Present marks apply to this call.
	IncludedModels    *[]string       `json:"included_models,omitempty"`
	UnavailableModels *[]string       `json:"unavailable_models,omitempty"`
	UsageQuota        *quota.Snapshot `json:"usage_quota,omitempty"`
	// TranscriptPath is the parent conversation JSONL (Claude-shaped). Used when
	// model_id and model are absent on Task preToolUse (live 3.27.12).
	TranscriptPath string `json:"transcript_path,omitempty"`
}

// ToolUseIdentifier returns the harness id of this tool call, if any.
func (ev Event) ToolUseIdentifier() string { return ev.ToolUseID }

// SessionIdentifier returns the session id for telemetry correlation when
// Cursor sends session_id on preToolUse (live probe 3.27.12).
func (ev Event) SessionIdentifier() string { return ev.SessionID }

// ProjectDir returns the working directory the harness reported, if any.
func (ev Event) ProjectDir() string {
	if ev.Cwd != "" {
		return ev.Cwd
	}
	if len(ev.WorkspaceRoots) > 0 {
		return ev.WorkspaceRoots[0]
	}
	return ""
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
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return baselineCurrentModel(ev, nil)
	}
	return baselineCurrentModel(ev, ti)
}

// ParentModelIdentifier is the session model the payload reports (model_id,
// then model), else the model of the transcript line that issued this call.
func (ev Event) ParentModelIdentifier() (string, string) {
	for _, m := range []string{ev.ModelID, ev.Model} {
		if m != "" {
			return m, "payload"
		}
	}
	if m := parentModelAtSpawn(ev.TranscriptPath, ev.ToolUseID); m != "" {
		return m, "transcript"
	}
	return "", ""
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
	touchComposer(ev)
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

	tiModel := hookutil.StringField(ti, "model")
	// fromParent: baseline from inherit, empty tool_input.model, model_id, or transcript.
	fromParent := tiModel == "" || tiModel == "inherit"
	currentModel := baselineCurrentModel(ev, ti)

	var res core.Resolver
	if len(r) > 0 {
		res = r[0]
	}
	session := core.ResolveSession(harnessID, ev.SessionModels, ev.AvailableModels)
	session = session.WithHookQuota(ev.IncludedModels, ev.UnavailableModels)
	session = session.WithUsageQuota(harnessID, ev.UsageQuota)
	if ev.ModelMetadata != nil {
		session.Metadata = ev.ModelMetadata
	}
	res = core.ResolveSessionModels(harnessID, session, res)
	decision := core.RouteSpawn(core.Request{
		Prompt: subPrompt, Harness: harnessID, AgentType: hookutil.StringField(ti, "subagent_type"),
		ModelInherited: fromParent, CurrentModelID: currentModel,
	}, res)
	decision.Baseline.RequestedID = currentModel
	decision.Baseline.Guessed = fromParent

	plan := decision.PlanSession(core.CursorCaps, res, session, currentModel)
	if plan.HoldForeign || plan.PreserveExplicit {
		return allow(), "", decision
	}
	target := plan.Model
	if target.ID == "" || target.ID == decision.Baseline.CurrentModel.ID {
		return allow(), "", decision
	}
	if !plan.RewriteModel {
		return allow(), "", decision
	}
	if decision.HoldGuessedDownshift(target) {
		return allow(), "", decision
	}
	if !core.CanWriteSessionID(harnessID, target.ID, plan.Session, res) {
		return allow(), "", decision
	}
	writeName, ok := core.SpawnWriteName(harnessID, plan.WriteName, res)
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
	note := decision.Summary()

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
