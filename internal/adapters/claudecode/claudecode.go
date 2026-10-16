// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package claudecode adapts core routing decisions to Claude Code's hook
// protocol. Claude Code spawns subagents via the Task tool; a PreToolUse hook
// can rewrite the tool input before the subagent starts — including its model.
// This is the one place a subagent's model can be set programmatically, before
// the child process loads it.
package claudecode

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
	"github.com/tiagovilasboas/downshift/internal/quota"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

const harnessID = "claude-code"

// Event is the JSON Claude Code sends on stdin for a PreToolUse hook.
type Event struct {
	HookEventName string          `json:"hook_event_name"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
	Model         string          `json:"model"`
	Prompt        string          `json:"prompt"`
	// Optional opaque session id. Persisted only as a hash by runHookAdapter
	// (SessionIdentifier); empty when the harness omits it, keeping old logs
	// byte-identical. Enables PostToolUse cost linkage by session.
	SessionID string `json:"session_id,omitempty"`
	// Cwd is the hook's working directory, used to find a project policy file.
	Cwd string `json:"cwd,omitempty"`
	// TranscriptPath is the parent session transcript. Read only to learn the
	// session model a Task call without "model" inherits.
	TranscriptPath string `json:"transcript_path,omitempty"`
	// PermissionMode is the session's permission mode; it decides which model
	// the "opusplan" setting means (see configuredModel).
	PermissionMode string `json:"permission_mode,omitempty"`
	// ToolUseID is the harness id of this Task call; PostToolUse repeats it.
	ToolUseID string `json:"tool_use_id,omitempty"`
	// Optional allowlist. Nil means the payload did not include one.
	// Claude Code PreToolUse does not send this field today.
	ModelMetadata   []core.SessionModel `json:"model_metadata,omitempty"`
	SessionModels   *[]string           `json:"session_models,omitempty"`
	AvailableModels *[]string           `json:"available_models,omitempty"`
	// IncludedModels and UnavailableModels are optional quota marks.
	// Absent marks supply no credit evidence. Present marks apply to this call.
	IncludedModels    *[]string       `json:"included_models,omitempty"`
	UnavailableModels *[]string       `json:"unavailable_models,omitempty"`
	UsageQuota        *quota.Snapshot `json:"usage_quota,omitempty"`
}

// ProjectDir returns the working directory the harness reported, if any.
func (ev Event) ProjectDir() string { return ev.Cwd }

// ToolUseIdentifier returns the harness id of this tool call, if any.
func (ev Event) ToolUseIdentifier() string { return ev.ToolUseID }

// CorrelationIdentifier returns the optional opaque ID supplied by a caller.
func (ev Event) CorrelationIdentifier() string { return ev.CorrelationID }

// AgentTypeIdentifier returns the Task call's subagent_type exactly as sent,
// empty when the call named none. The runner turns it into a storable label
// (telemetry.AgentTypeLabel); the raw value never reaches the event log.
func (ev Event) AgentTypeIdentifier() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	return hookutil.StringField(ti, "subagent_type")
}

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
		return ev.Model
	}
	return ev.currentModel(ti)
}

// currentModel is the model the subagent would run on without a rewrite: the
// Task "model", the payload "model", then, for agent types that inherit (see
// inheritingAgentTypes), the harness override env
// (core.ClaudeCodeCaps.ModelOverrideEnv; Explore and Plan only when forced) and
// finally the parent session model from the transcript. It never reads the
// settings fallback, which needs a catalog (see resolveModel).
func (ev Event) currentModel(ti map[string]any) string {
	m, _ := ev.resolveModel(ti, nil)
	return m
}

// resolveModel is currentModel plus a last resort for an inheriting agent type
// whose transcript holds no model: the model the session is configured to use
// (configuredModel), which needs res to turn an alias into a catalog id. The
// bool reports that the model came from that fallback, so the decision can say
// the baseline is inferred. A model in the transcript always wins.
func (ev Event) resolveModel(ti map[string]any, res core.Resolver) (string, bool) {
	if model := hookutil.StringField(ti, "model"); model != "" {
		return model, false
	}
	if ev.Model != "" {
		return ev.Model, false
	}
	if !inheritsSessionModel(ti) {
		return "", false
	}
	typ := hookutil.StringField(ti, "subagent_type")
	if m := subagentModelFromEnv(typ); m != "" {
		return m, false
	}
	parent := sessionModelFromTranscript(ev.TranscriptPath)
	inferred := false
	if parent == "" && res != nil {
		parent = configuredModel(ev.Cwd, ev.PermissionMode, res)
		inferred = parent != ""
	}
	if exploreMayLeaveTheMainModel(typ, parent) {
		return "", false
	}
	return parent, inferred
}

// TaskText returns the task content used for local, prompt-free feature
// extraction by the shared engineering loop.
func (ev Event) TaskText() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	return hookutil.TaskText(ti, "prompt", "description")
}

// Output is the JSON we print on stdout to steer Claude Code.
type Output struct {
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
	SystemMessage      string              `json:"systemMessage,omitempty"`
}

// HookSpecificOutput carries the PreToolUse decision.
type HookSpecificOutput struct {
	HookEventName      string          `json:"hookEventName"`
	PermissionDecision string          `json:"permissionDecision"`
	UpdatedInput       json.RawMessage `json:"updatedInput,omitempty"`
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
	currentModel, fromSettings := ev.resolveModel(ti, res)

	session := core.ResolveSessionForID(harnessID, ev.SessionID, ev.SessionModels, ev.AvailableModels)
	session = session.WithHookQuota(ev.IncludedModels, ev.UnavailableModels)
	session = session.WithUsageQuota(harnessID, ev.UsageQuota)
	if ev.ModelMetadata != nil {
		session.Metadata = ev.ModelMetadata
	}
	res = core.ResolveSessionModels(harnessID, session, res)
	decision := core.Route(subPrompt, harnessID, currentModel, res)
	decision.RequestedID = currentModel
	decision.BaselineFromSettings = fromSettings
	decision.BaselineGuessed = fromSettings

	plan := decision.PlanSession(core.ClaudeCodeCaps, res, session, currentModel)
	if plan.HoldForeign || plan.PreserveExplicit || !plan.RewriteModel || !core.CanWriteSessionID(harnessID, plan.Model.ID, plan.Session, res) {
		return allow(), "", decision
	}
	// A baseline read from settings is a guess: the session may run another
	// model (--model, /model). Only the small tier is cheaper than anything the
	// session can run, so a downshift on a guessed baseline is written only to
	// it; any other target could be an upshift nobody sees.
	if fromSettings && decision.Verdict == core.VerdictDownshift && plan.Model.Tier != core.TierSmall {
		decision.GuessHeld = true
		return allow(), "", decision
	}
	decision.Model = plan.Model

	// plan.WriteName is the catalog's native name: Claude Code validates
	// "model" against family names, and the core plan already declined to
	// rewrite when the entry has none.
	ti["model"] = plan.WriteName
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
	if !telemetry.Quiet() {
		out.SystemMessage = "downshift: " + decision.Summary()
	}
	return out, decision.Summary(), decision
}

func allow() Output {
	return Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:      "PreToolUse",
			PermissionDecision: "allow",
		},
	}
}
