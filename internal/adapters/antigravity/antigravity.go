// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package antigravity

import (
	"encoding/json"
	"strings"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/hookutil"
)

const harnessID = "antigravity"

type ToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type Event struct {
	ToolCall ToolCall `json:"toolCall"`
	// Optional allowlist. Nil means the payload did not include one.
	// ResolveSession falls back to the user file when both are nil.
	SessionModels   *[]string `json:"session_models,omitempty"`
	AvailableModels *[]string `json:"available_models,omitempty"`
	// IncludedModels and UnavailableModels are optional quota marks.
	// Nil leaves the user file. A non-nil slice replaces it for this call.
	IncludedModels    *[]string `json:"included_models,omitempty"`
	UnavailableModels *[]string `json:"unavailable_models,omitempty"`
}

type Output struct {
	Decision  string          `json:"decision"`
	Overwrite json.RawMessage `json:"overwrite,omitempty"`
}

func (ev Event) TaskText() string {
	var ti map[string]any
	if err := json.Unmarshal(ev.ToolCall.Args, &ti); err != nil {
		return ""
	}
	if subagentsRaw, ok := ti["Subagents"].([]any); ok && len(subagentsRaw) > 0 {
		if subagent, ok := subagentsRaw[0].(map[string]any); ok {
			return hookutil.TaskText(subagent, "Prompt", "prompt", "task", "Task")
		}
	}
	return hookutil.TaskText(ti, "Prompt", "prompt", "task", "Task")
}

func Handle(ev Event, r ...core.Resolver) (Output, string, core.Decision) {
	if !strings.EqualFold(ev.ToolCall.Name, "invoke_subagent") && !strings.EqualFold(ev.ToolCall.Name, "spawn_agent") {
		return allow(), "", core.Decision{}
	}

	var ti map[string]any
	if err := json.Unmarshal(ev.ToolCall.Args, &ti); err != nil {
		return allow(), "", core.Decision{}
	}

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

	changed := false
	var lastDecision core.Decision

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
		decision.RequestedID = currentModel
		decision.SessionUnknown = !session.Known
		lastDecision = decision

		// Gate the rewrite on the session plan: unknown session, empty
		// target, explicit-only current model, or a rewrite the plan
		// forbids all fail open (allow, no write).
		plan := decision.PlanForSession(core.AntigravityCaps, res, session)
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
		if !core.CanWriteSessionID(harnessID, mappedModel, session, res) {
			continue
		}

		subagent["Model"] = plan.WriteName
		changed = true
	}

	if !changed {
		return allow(), "", lastDecision
	}

	updatedArgs, err := json.Marshal(ti)
	if err != nil {
		return allow(), "", lastDecision
	}

	return Output{
		Decision:  "allow",
		Overwrite: updatedArgs,
	}, "downshift: mapped to antigravity tier " + lastDecision.Tier.String(), lastDecision
}

func allow() Output {
	return Output{Decision: "allow"}
}
