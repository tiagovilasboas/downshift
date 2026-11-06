// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package antigravity

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
	"github.com/tiagovilasboas/downshift/internal/quota"
)

const harnessID = "antigravity"

type ToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type Event struct {
	ToolCall ToolCall `json:"toolCall"`
	// Cwd is the working directory, used to find a project policy file.
	Cwd string `json:"cwd,omitempty"`
	// WorkspacePaths is the IDE hook field for the open workspaces; fallback
	// source for the project directory when cwd is absent.
	WorkspacePaths []string `json:"workspacePaths,omitempty"`
	// Optional allowlist. Nil means the payload did not include one.
	// ResolveSession falls back to the user file when both are nil.
	ModelMetadata   []core.SessionModel `json:"model_metadata,omitempty"`
	SessionModels   *[]string           `json:"session_models,omitempty"`
	AvailableModels *[]string           `json:"available_models,omitempty"`
	// IncludedModels and UnavailableModels are optional quota marks.
	// Absent marks supply no credit evidence. Present marks apply to this call.
	IncludedModels    *[]string       `json:"included_models,omitempty"`
	UnavailableModels *[]string       `json:"unavailable_models,omitempty"`
	UsageQuota        *quota.Snapshot `json:"usage_quota,omitempty"`
}

type Output struct {
	Decision  string          `json:"decision"`
	Overwrite json.RawMessage `json:"overwrite,omitempty"`
}

// ProjectDir returns the working directory the harness reported, if any.
func (ev Event) ProjectDir() string {
	if ev.Cwd != "" {
		return ev.Cwd
	}
	if len(ev.WorkspacePaths) > 0 {
		return ev.WorkspacePaths[0]
	}
	return ""
}

// decidedSubagent returns the child a routing row is about: the last subagent
// that has a Prompt, the same one Handle ends up returning a decision for. Task
// text and requested model both read it, so a row never pairs one child's task
// with another child's model. Nil when no child has a Prompt.
func decidedSubagent(ti map[string]any) map[string]any {
	subagents, _ := ti["Subagents"].([]any)
	var decided map[string]any
	for _, raw := range subagents {
		if subagent, ok := raw.(map[string]any); ok && hookutil.StringField(subagent, "Prompt") != "" {
			decided = subagent
		}
	}
	return decided
}

func (ev Event) TaskText() string {
	var ti map[string]any
	if err := json.Unmarshal(ev.ToolCall.Args, &ti); err != nil {
		return ""
	}
	if subagent := decidedSubagent(ti); subagent != nil {
		return hookutil.TaskText(subagent, "Prompt", "prompt", "task", "Task")
	}
	return hookutil.TaskText(ti, "Prompt", "prompt", "task", "Task")
}

// RequestedModel returns the Model of the subagent whose decision Handle
// returns: the last one with a prompt. That keeps the telemetry row's
// requested model and its decision about the same child.
func (ev Event) RequestedModel() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolCall.Args, &ti) != nil {
		return ""
	}
	if subagent := decidedSubagent(ti); subagent != nil {
		return hookutil.StringField(subagent, "Model")
	}
	return ""
}

func Handle(ev Event, r ...core.Resolver) (Output, string, core.Decision) {
	var ti map[string]any
	if err := json.Unmarshal(ev.ToolCall.Args, &ti); err != nil {
		return allow(), "", core.Decision{}
	}

	// Structural detection: Antigravity subagent spawns carry a non-empty
	// "Subagents" array in the tool args. Tool name is not checked so the
	// adapter stays agnostic to harness-specific naming.
	subagentsRaw, ok := ti["Subagents"].([]any)
	if !ok || len(subagentsRaw) == 0 {
		return allow(), "", core.Decision{}
	}

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

	changed := false
	var lastDecision core.Decision
	lastWritten := false

	for _, raw := range subagentsRaw {
		subagent, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		prompt := hookutil.StringField(subagent, "Prompt")
		if prompt == "" {
			continue
		}

		currentModel := hookutil.StringField(subagent, "Model")
		decision := core.Route(prompt, harnessID, currentModel, res)
		decision.Baseline.RequestedID = currentModel
		// Gate the rewrite on the session plan: unknown session, empty
		// target, explicit-only current model, or a rewrite the plan
		// forbids all fail open (allow, no write).
		plan := decision.PlanSession(core.AntigravityCaps, res, session, currentModel)
		lastDecision = decision
		lastWritten = false
		if plan.HoldForeign || plan.PreserveExplicit || !plan.RewriteModel || !session.Contains(plan.Model.ID) {
			continue
		}
		// A decision held by a guardrail (self-correction recorded a
		// violation) must never be applied, even when the plan names a
		// session target. The embedded catalog carries native flash_lite /
		// flash / pro entries, so R4 (foreign model) no longer fires for
		// Antigravity; R1/R2/R5/R6 still hold decisions here.
		if len(decision.Corrections) > 0 {
			continue
		}

		mappedModel := plan.Model.ID

		// The string written must be one of this session's selectable model IDs.
		if !core.CanWriteSessionID(harnessID, mappedModel, plan.Session, res) {
			continue
		}

		subagent["Model"] = plan.WriteName
		changed = true
		lastDecision.Model = plan.Model
		lastWritten = true
	}

	if !changed {
		return allow(), "", lastDecision
	}

	updatedArgs, err := json.Marshal(ti)
	if err != nil {
		return allow(), "", lastDecision
	}

	// The note marks the telemetry row as a rewrite, and the row describes the
	// last child: an earlier child's rewrite must not make a held or unchanged
	// last child look applied.
	note := ""
	if lastWritten {
		note = "downshift: " + lastDecision.Summary()
	}
	return Output{
		Decision:  "allow",
		Overwrite: updatedArgs,
	}, note, lastDecision
}

func allow() Output {
	return Output{Decision: "allow"}
}
