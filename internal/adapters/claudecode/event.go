// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"encoding/json"
	"os"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
	"github.com/tiagovilasboas/downshift/internal/quota"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

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
	// ParentModel is the parent's effective model when a caller already knows
	// it (the Claude Code mod's agent.spawn event). Used as the baseline only
	// for inheriting agent types, before the transcript tail; never a guess.
	ParentModel string `json:"parent_model,omitempty"`
	// Entry names which spawn-time path built this payload ("hook" or "mod").
	// Empty means the settings/plugin PreToolUse hook; the runner records "hook".
	Entry string `json:"entry,omitempty"`
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

// EntryIdentifier returns the spawn-time entry point named in the payload.
// Empty means the settings or plugin PreToolUse hook.
func (ev Event) EntryIdentifier() string { return ev.Entry }

// PermissionModeIdentifier returns the session's permission mode name as sent.
// The runner keeps only a known label (telemetry.PermissionModeLabel).
func (ev Event) PermissionModeIdentifier() string { return ev.PermissionMode }

// clientEntrypointEnv is the variable Claude Code sets in the environment of
// the processes it starts, naming which client launched it (for example "cli"
// or "claude-desktop"). Claude Desktop's Code tab runs an embedded Claude Code.
const clientEntrypointEnv = "CLAUDE_CODE_ENTRYPOINT"

// ClientIdentifier returns the Claude Code client that fired the hook, read
// from CLAUDE_CODE_ENTRYPOINT in the environment. This is a Claude Code harness
// fact, so it lives in the adapter; the runner stores only the sanitized label
// (telemetry.ClientOrDrop). The value is opaque and never used for policy.
func (ev Event) ClientIdentifier() string { return os.Getenv(clientEntrypointEnv) }

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

// RequestedReasoningEffort is the effort the child would run at without a
// rewrite, in Claude Code's order: the Agent call's `effort`, the agent
// definition's `effort:`, then the session's. The Downshift mod's
// `reasoning_effort` counts as a per-call value. With neither, the child
// inherits the session effort, which the hook does not see: "inherit".
func (ev Event) RequestedReasoningEffort() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	if e := hookutil.StringField(ti, "effort"); e != "" {
		return e
	}
	if e := hookutil.StringField(ti, "reasoning_effort"); e != "" {
		return e
	}
	if def := lookupAgentDefinition(ev.Cwd, hookutil.StringField(ti, "subagent_type")); def.Effort != "" {
		return def.Effort
	}
	return telemetry.EffortInherit
}

// RequestedModel returns the model received before Downshift emitted a rewrite.
func (ev Event) RequestedModel() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ev.Model
	}
	return ev.currentModel(ti)
}

// ParentModelIdentifier is the model the parent session ran on at this spawn,
// whether or not the call names its own model, and where it came from: "mod"
// (the mod's agent.spawn event) or "transcript" (the assistant line that issued
// this call, a bounded read). Empty when neither knows it. Data for telemetry
// only; the routing baseline is resolveBaseline.
func (ev Event) ParentModelIdentifier() (string, string) {
	if ev.ParentModel != "" {
		return ev.ParentModel, "mod"
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
	return hookutil.TaskText(ti, "prompt", "description")
}
