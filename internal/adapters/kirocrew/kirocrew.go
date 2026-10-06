// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package kirocrew adapts core routing decisions to KiroCrew's preToolUse hook.
//
// KiroCrew is the first supported harness WITHOUT an in-place rewrite path.
// Its preToolUse contract is binary — exit 0 allows the tool, exit 2 blocks it
// and returns stderr to the LLM (the docs say: "write a preToolUse hook as a
// policy"). There is no updated_input channel, so the adapter cannot swap the
// subagent's model before the child starts the way the Claude Code, Cursor and
// Codex adapters do.
//
// Instead this adapter runs in POLICY MODE: it classifies the pending subagent
// spawn and, when the requested model does not match the recommended tier,
// blocks the spawn with an actionable message telling the agent to respawn at
// the right tier. The classification stays deterministic and prompt-free — the
// shared core engine, no LLM in the loop. Fail-open is absolute: an empty task,
// a non-subagent tool, an unknown current model, a low-confidence downshift,
// a target outside the session, a guardrail-held decision, or an
// explicit-only target all allow the spawn. The router must never block a
// spawn on its own doubt, and a block must never name a model id the session
// does not have.
package kirocrew

import (
	"encoding/json"
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
)

const harnessID = "kirocrew"

// subagentTools are the tool names KiroCrew uses to spawn a subagent. The
// preToolUse matcher already scopes the hook, but the adapter re-checks so a
// broad matcher (e.g. "*") never blocks an unrelated tool.
var subagentTools = map[string]bool{
	"subagent":         true,
	"agent_crew":       true,
	"use_subagent":     true,
	"spawn_run":        true,
	"spawn_sub_agents": true,
}

// Event is the JSON KiroCrew sends on stdin for a preToolUse hook.
// Field names follow KiroCrew's documented hook event schema.
type Event struct {
	HookEventName     string          `json:"hook_event_name"`
	CorrelationID     string          `json:"correlation_id,omitempty"`
	ToolName          string          `json:"tool_name"`
	ToolInput         json.RawMessage `json:"tool_input"`
	SessionModels     *[]string       `json:"session_models,omitempty"`
	AvailableModels   *[]string       `json:"available_models,omitempty"`
	IncludedModels    *[]string       `json:"included_models,omitempty"`
	UnavailableModels *[]string       `json:"unavailable_models,omitempty"`
}

// CorrelationIdentifier returns the optional opaque ID supplied by a caller.
func (ev Event) CorrelationIdentifier() string { return ev.CorrelationID }

// TaskText returns the subagent task content used for prompt-free feature
// extraction. spawn_run uses "task"; spawn_sub_agents nests it under "prompt".
func (ev Event) TaskText() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	return hookutil.TaskText(ti, "task", "prompt", "description")
}

// Output is what the runner needs to pick an exit code. Unlike the rewrite
// adapters, KiroCrew's output is not printed as JSON to stdout — Block drives
// exit 2 and Message goes to stderr (which KiroCrew relays to the LLM).
type Output struct {
	// Block is true when the spawn should be denied (exit 2) so the agent
	// respawns at the recommended tier. False means allow (exit 0).
	Block bool
	// Message is the actionable text shown to the LLM on a block. Empty on allow.
	Message string
}

// Handle classifies a preToolUse event and decides allow vs block.
// Returns the policy output, a human-readable note for telemetry/stderr, and
// the full routing Decision so the runner can record telemetry without
// re-classifying. A zero Decision (Harness == "") means "not a subagent spawn".
func Handle(ev Event, r ...core.Resolver) (Output, string, core.Decision) {
	if !subagentTools[ev.ToolName] {
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

	var res core.Resolver
	if len(r) > 0 {
		res = r[0]
	}
	decision := core.Route(subPrompt, harnessID, currentModel, res)
	decision.RequestedID = currentModel
	session := core.ResolveSession(harnessID, ev.SessionModels, ev.AvailableModels)
	session = session.WithHookQuota(ev.IncludedModels, ev.UnavailableModels)
	decision.SessionUnknown = !session.Known

	// The user deliberately picked an explicit_only model: never block it or
	// ask for a respawn on another model.
	if decision.ShouldPreserveExplicitModel(currentModel, res) {
		return allow(), "", decision
	}

	// Right gear already: nothing to do.
	if decision.Verdict == core.VerdictOK {
		return allow(), "", decision
	}

	// An uncertain downshift must never block: blocking on doubt would trade
	// a real spawn for a wrong-way demotion. Upshifts are safe to enforce even
	// when confidence is soft — under-powering a subagent is the costlier error —
	// but like every block they still need an actionable, held-free,
	// in-session target (see the policy-mode gate below).
	if decision.Verdict == core.VerdictDownshift && !decision.Confident {
		return allow(), "", decision
	}

	// VerdictUnknown means we could not compare (no current model, or an
	// unrecognised one). Fail open — do not block a spawn we cannot reason about.
	if decision.Verdict == core.VerdictUnknown {
		return allow(), "", decision
	}

	// Policy-mode gate: use the same session-only target plan as rewrite
	// adapters. KiroCrew cannot rewrite in place, but may ask for a respawn
	// only when the exact target is selectable in this session.
	plan := decision.PlanForSession(core.KiroCrewCaps, res, session)
	if plan.HoldForeign || plan.PreserveExplicit || plan.Model.ID == "" || plan.Model.ID == currentModel || !session.Contains(plan.Model.ID) {
		return allow(), "", decision
	}
	if len(decision.Corrections) > 0 {
		return allow(), "", decision
	}
	if res != nil && res.IsExplicitOnly(harnessID, plan.Model.ID) {
		return allow(), "", decision
	}

	// A confident mismatch with an actionable in-session target (downshift or
	// upshift): block and tell the agent exactly which model to respawn with.
	decision.Model = plan.Model
	note := decision.Summary()
	msg := fmt.Sprintf("downshift: %s\nRespawn this subagent with model=%s (tier %s).",
		note, plan.Model.ID, decision.Tier)
	// The context-trim hint only applies when moving DOWN to a small window;
	// an upshift to a frontier model has room to spare.
	if decision.Tier == core.TierSmall {
		msg += " For a small tier, also trim context (include_memory/include_lessons/" +
			"include_project=false) so it fits the model's window."
	}
	return Output{Block: true, Message: msg}, note, decision
}

func allow() Output { return Output{Block: false} }
