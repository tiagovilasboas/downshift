// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

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
	"fmt"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
	"github.com/tiagovilasboas/downshift/internal/quota"
)

const harnessID = "codex"

// Event is the JSON Codex sends on stdin for a PreToolUse hook.
type Event struct {
	HookEventName string `json:"hook_event_name"`
	// Cwd is the working directory, used to find a project policy file.
	Cwd           string `json:"cwd,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	// ToolUseID is the harness id of this tool call (documented in its hook payload).
	ToolUseID string          `json:"tool_use_id,omitempty"`
	SessionID string          `json:"session_id"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	Model     string          `json:"model"`
	// Optional allowlist. Nil means the payload did not include one.
	// Codex PreToolUse does not send this field today.
	ModelMetadata   []core.SessionModel `json:"model_metadata,omitempty"`
	SessionModels   *[]string           `json:"session_models,omitempty"`
	AvailableModels *[]string           `json:"available_models,omitempty"`
	// IncludedModels and UnavailableModels are optional quota marks.
	// Absent marks supply no credit evidence. Present marks apply to this call.
	IncludedModels    *[]string       `json:"included_models,omitempty"`
	UnavailableModels *[]string       `json:"unavailable_models,omitempty"`
	UsageQuota        *quota.Snapshot `json:"usage_quota,omitempty"`
	TranscriptPath    string          `json:"transcript_path,omitempty"`
}

// ToolUseIdentifier returns the harness id of this tool call, if any.
func (ev Event) ToolUseIdentifier() string { return ev.ToolUseID }

// SessionIdentifier exposes Codex's stable session ID to the shared hook
// runner without coupling core routing decisions to harness metadata.
func (ev Event) SessionIdentifier() string { return ev.SessionID }

// ProjectDir returns the working directory the harness reported, if any.
func (ev Event) ProjectDir() string { return ev.Cwd }

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
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ev.Model
	}
	return ev.currentModel(ti)
}

// currentModel is the model the child would run on without a rewrite: the
// spawn "model", the payload "model", then the parent session model.
func (ev Event) currentModel(ti map[string]any) string {
	if model := hookutil.StringField(ti, "model"); model != "" {
		return model
	}
	if ev.Model != "" {
		return ev.Model
	}
	return sessionModelFromTranscript(ev.TranscriptPath)
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
	var ti map[string]any
	if err := json.Unmarshal(ev.ToolInput, &ti); err != nil {
		return allow(), "", core.Decision{}
	}

	// Structural detection: a Codex subagent spawn carries "message" and/or
	// "task_name" in tool_input. Tool name is deliberately not checked.
	subPrompt := hookutil.StringField(ti, "message")
	if tn := hookutil.StringField(ti, "task_name"); tn != "" {
		subPrompt = strings.TrimSpace(subPrompt + " " + tn)
	}
	if subPrompt == "" {
		return allow(), "", core.Decision{}
	}

	currentModel := ev.currentModel(ti)

	var res core.Resolver
	if len(r) > 0 {
		res = r[0]
	}
	session := core.ResolveSessionForID(harnessID, ev.SessionID, ev.SessionModels, ev.AvailableModels)
	session = session.WithHookQuota(ev.IncludedModels, ev.UnavailableModels)
	usage := ev.UsageQuota
	if usage == nil && ev.TranscriptPath != "" {
		usage = &quota.Snapshot{Harness: harnessID}
		if observed, err := quota.CollectCodex(ev.TranscriptPath, 5*time.Minute); err == nil {
			usage = &observed
		}
	}
	session = session.WithUsageQuota(harnessID, usage)
	if ev.ModelMetadata != nil {
		session.Metadata = ev.ModelMetadata
	}
	res = core.ResolveSessionModels(harnessID, session, res)
	decision := core.Route(subPrompt, harnessID, currentModel, res)
	decision.RequestedID = currentModel

	plan := decision.PlanSession(core.CodexCaps, res, session, currentModel)
	if plan.HoldForeign || plan.PreserveExplicit || (!plan.RewriteModel && !plan.ApplyEffort) {
		return allow(), "", decision
	}
	if plan.RewriteModel && !core.CanWriteSessionID(harnessID, plan.Model.ID, plan.Session, res) {
		return allow(), "", decision
	}
	effort, effortOK, effortChanged := core.ConstrainEffort(res, harnessID, plan.Model.ID, decision.Effort)
	effortValue := effort.String()
	if res != nil {
		effortValue = res.EffortFor(harnessID, plan.Model.ID, effort)
	}
	requestedEffort := hookutil.StringField(ti, "reasoning_effort")
	applyEffort := plan.ApplyEffort && effortOK && effortAllowed(decision, requestedEffort, effortValue, plan.RewriteModel)
	if plan.ApplyEffort && effortChanged {
		decision.Corrections = append(decision.Corrections, core.RuleEffortUnsupported)
	}
	if !plan.RewriteModel && (!applyEffort || effortValue == requestedEffort) {
		return allow(), "", decision
	}

	note := ""
	if plan.RewriteModel {
		decision.Model = plan.Model
		ti["model"] = plan.WriteName
		note = decision.Summary()
	} else {
		// Effort-only change: the model stays as requested, and the event
		// must record the model the child actually runs on.
		decision.Model = decision.CurrentModel
		note = fmt.Sprintf("%s task → keep %s, reasoning effort %s → %s",
			decision.Complexity, decision.CurrentModel.ID, effortOrInherit(requestedEffort), effortValue)
	}
	if applyEffort {
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
	return out, note, decision
}

// effortRank orders the cross-harness effort vocabulary. Unknown values
// rank -1.
func effortRank(v string) int {
	for i, e := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max", "ultra"} {
		if v == e {
			return i
		}
	}
	return -1
}

// effortAllowed reports whether the hook may write effort value over the
// requested one. A confident, unheld decision may set any effort. A decision
// the guardrails held (its verdict reduced to a safe one) writes no effort:
// a held change, up or down, leaves the spawn as requested. An unconfident,
// unheld decision writes effort only alongside a model rewrite (a
// deterministic lift such as the risk floor), and never lowers it: on its
// own, raising effort would spend more on doubt (docs/reference/session-models.md).
func effortAllowed(d core.Decision, requested, value string, modelRewritten bool) bool {
	if d.Confident && len(d.Corrections) == 0 {
		return true
	}
	if d.Checked && d.SafeVerdict != d.Verdict {
		return false
	}
	if !modelRewritten {
		return false
	}
	if effortRank(requested) < 0 {
		return true
	}
	return effortRank(value) >= effortRank(requested)
}

func effortOrInherit(v string) string {
	if v == "" {
		return "inherit"
	}
	return v
}

func allow() Output {
	return Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:      "PreToolUse",
			PermissionDecision: "allow",
		},
	}
}
