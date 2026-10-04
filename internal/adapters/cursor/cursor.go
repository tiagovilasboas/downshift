// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

// Package cursor adapts core routing decisions to Cursor's hook protocol.
// Cursor exposes a preToolUse hook whose output supports updated_input —
// the same interception point Claude Code uses. When the agent is about to
// spawn a subagent via the Task tool, we rewrite the tool input's model
// before the child starts.
package cursor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/hookutil"
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
	// Nil leaves the user file. A non-nil slice replaces it for this call.
	IncludedModels    *[]string `json:"included_models,omitempty"`
	UnavailableModels *[]string `json:"unavailable_models,omitempty"`
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
	if !isTaskTool(ev.ToolName) {
		return allow(), "", core.Decision{}
	}

	var ti map[string]any
	if err := json.Unmarshal(ev.ToolInput, &ti); err != nil {
		return allow(), "", core.Decision{}
	}

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
	plan := decision.PlanForSession(core.CursorCaps, res, session)
	if plan.HoldForeign || plan.PreserveExplicit {
		return allow(), "", decision
	}

	// Option A: stay in the current model's family, switch only the effort.
	// The family comes from the catalog (exact, alias, or prefix match), so
	// this works even for version-bumped IDs the catalog never listed. When
	// the family has no variant at the needed effort, fall back to the tier
	// default below. Other harnesses keep their existing path until they
	// opt into FamilyEffortResolver.
	//
	// Safety rule: the family variant must meet or exceed the needed tier.
	// An in-family move that preserves capability needs no confidence gate;
	// anything weaker (or no variant at all) goes through the normal rewrite
	// gate, which already blocks downgrades on uncertain classifications.
	// A decision held by a guardrail (Corrections non-empty) never takes
	// this shortcut: it falls through to the normal gate below so SafeVerdict
	// holds and session-target selection still apply.
	target := plan.Model
	note := decision.Summary()
	familyHit := false
	if res != nil && decision.CurrentModel.Family != "" && len(decision.Corrections) == 0 {
		if fer, ok := res.(core.FamilyEffortResolver); ok {
			if fm, ok := fer.FamilyModelFor(harnessID, decision.CurrentModel.Family, decision.Effort); ok &&
				fm.ID != "" && session.Contains(fm.ID) && !session.Blocks(fm.ID) && fm.ID != decision.CurrentModel.ID && fm.Tier >= decision.Tier {
				target = fm
				note = familyNote(decision, fm, res)
				familyHit = true
			}
		}
	}

	if target.ID == "" || target.ID == decision.CurrentModel.ID {
		return allow(), "", decision
	}
	if !familyHit && !plan.RewriteModel {
		return allow(), "", decision
	}
	if !core.CanWriteCatalogID(harnessID, target.ID, session, res) {
		return allow(), "", decision
	}
	decision.Model = target
	if !familyHit {
		note = decision.Summary()
	}

	ti["model"] = target.ID
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

// familyNote renders the one-line readout for an in-family effort switch.
// It mirrors Decision.Summary: cheaper targets show the savings ratio,
// pricier ones are framed as upshifts needing more torque.
func familyNote(d core.Decision, fm core.Model, res core.Resolver) string {
	if res != nil {
		if s := res.SavingsRatio(d.CurrentModel, fm); s > 0 {
			return fmt.Sprintf("%s task → stay on %s family at %s effort: %s (~%.0f%% cheaper)",
				d.Complexity, d.CurrentModel.Family, d.Effort, fm.ID, s*100)
		}
	}
	return fmt.Sprintf("%s task → stay on %s family at %s effort: %s (needs more torque)",
		d.Complexity, d.CurrentModel.Family, d.Effort, fm.ID)
}

func isTaskTool(name string) bool {
	return strings.EqualFold(name, "task")
}

func allow() Output {
	return Output{Permission: "allow"}
}
