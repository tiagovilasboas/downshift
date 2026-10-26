// Copyright (c) 2026 Tiago Vilas Boas.
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
// no model field in the input, an unknown current model, a low-confidence
// downshift, a target outside the session, a guardrail-held decision, or an
// explicit-only target all allow the spawn. The router must never block a
// spawn on its own doubt, and a block must never name a model id the session
// does not have.
//
// Spawn detection: only the KiroCrew spawn tools listed in spawnToolNames are
// routed. Any other tool (for example an image generator that takes model and
// prompt) passes through silently. Within a spawn tool, a tool_input without a
// model field or without task text is also passed through. A spawn_run with a
// tasks[] array is a batch: each entry is routed on its own (see batch.go).
package kirocrew

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
	"github.com/tiagovilasboas/downshift/internal/quota"
)

const harnessID = "kirocrew"

// spawnToolNames are the KiroCrew tools that spawn a subagent. Only these are
// routed; a tool that merely carries model and prompt fields is not a spawn.
var spawnToolNames = map[string]bool{
	"spawn_run":        true,
	"spawn_sub_agents": true,
}

// smallTierHint is appended to a block that names a small-tier target: the
// context-trim advice only applies when moving DOWN to a small window.
const smallTierHint = " For a small tier, also trim context (include_memory/include_lessons/" +
	"include_project=false) so it fits the model's window."

// Event is the JSON a harness sends on stdin for a hook.
// Field names follow KiroCrew's documented hook event schema; the same
// structure is used for preToolUse and postToolUse events.
type Event struct {
	HookEventName string `json:"hook_event_name"`
	// Cwd is the working directory, used to find a project policy file.
	Cwd string `json:"cwd,omitempty"`
	// SessionID is the opaque session identifier supplied by KiroCrew. Enables
	// PostToolUse cost linkage by session and makes the session visible in the
	// dashboard's /api/sessions list.
	SessionID         string              `json:"session_id,omitempty"`
	CorrelationID     string              `json:"correlation_id,omitempty"`
	ToolName          string              `json:"tool_name"`
	ToolInput         json.RawMessage     `json:"tool_input"`
	ToolResponse      json.RawMessage     `json:"tool_response,omitempty"` // postToolUse only
	ModelMetadata     []core.SessionModel `json:"model_metadata,omitempty"`
	SessionModels     *[]string           `json:"session_models,omitempty"`
	AvailableModels   *[]string           `json:"available_models,omitempty"`
	IncludedModels    *[]string           `json:"included_models,omitempty"`
	UnavailableModels *[]string           `json:"unavailable_models,omitempty"`
	UsageQuota        *quota.Snapshot     `json:"usage_quota,omitempty"`
}

// ProjectDir returns the working directory the harness reported, if any.
func (ev Event) ProjectDir() string { return ev.Cwd }

// SessionIdentifier returns the session ID for telemetry correlation, enabling
// the session to appear in the dashboard's /api/sessions list.
func (ev Event) SessionIdentifier() string { return ev.SessionID }

// CorrelationIdentifier returns the optional opaque ID supplied by a caller.
func (ev Event) CorrelationIdentifier() string { return ev.CorrelationID }

// RequestedModel exposes the payload baseline to the shared telemetry runner.
func (ev Event) RequestedModel() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	return hookutil.StringField(ti, "model")
}

// TaskText returns the subagent task content used for prompt-free feature
// extraction. Common field names across harnesses: "task", "prompt",
// "description". The first non-empty value wins.
func (ev Event) TaskText() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	return hookutil.TaskText(ti, "task", "prompt", "description")
}

// canonicalToolName drops the MCP server prefix KiroCrew adds when the agent
// reaches a tool through a server: "@kirocrew-core/spawn_run" is spawn_run.
// Any other name is returned unchanged.
func canonicalToolName(name string) string {
	if !strings.HasPrefix(name, "@") {
		return name
	}
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// isSubagentSpawn reports whether the call is a subagent spawn that the router
// should classify: the tool must be a spawn tool in spawnToolNames, and the
// tool_input must carry a model field (something to route) and a
// task/prompt/description field (something to classify).
func isSubagentSpawn(toolName string, ti map[string]any) bool {
	if !spawnToolNames[toolName] {
		return false
	}
	model := hookutil.StringField(ti, "model")
	if model == "" {
		return false
	}
	task := hookutil.TaskText(ti, "task", "prompt", "description")
	return task != ""
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
	// decisions holds one routing decision per routed batch entry. Nil for a
	// single spawn, where the decision returned by Handle is the only one.
	decisions []core.Decision
}

// Decisions returns every routing decision of a spawn_run batch, one per
// routed entry, or nil for a single spawn.
func (o Output) Decisions() []core.Decision { return o.decisions }

// spawnRoute is the routing outcome for one subagent spawn.
type spawnRoute struct {
	decision core.Decision
	// block is true when the spawn should respawn on target.
	block  bool
	target core.Model
	// note is the decision summary, set only when block is true.
	note string
}

// routeSpawn classifies one subagent spawn (task text plus requested model)
// against the session and reports whether it should respawn on another model.
func routeSpawn(ev Event, res core.Resolver, task, currentModel string) spawnRoute {
	session := core.ResolveSession(harnessID, ev.SessionModels, ev.AvailableModels)
	session = session.WithHookQuota(ev.IncludedModels, ev.UnavailableModels)
	session = session.WithUsageQuota(harnessID, ev.UsageQuota)
	// KiroCrew cannot report quota (its hook contract is allow/block only), so
	// treating quota as required would hold every downshift indefinitely.
	// Disable the requirement: policy-mode enforcement is safe because the agent
	// must explicitly respawn, and the harness itself enforces its own limits.
	session.QuotaRequired = false
	if ev.ModelMetadata != nil {
		session.Metadata = ev.ModelMetadata
	}
	res = core.ResolveSessionModels(harnessID, session, res)
	decision := core.Route(task, harnessID, currentModel, res)
	decision.Baseline.RequestedID = currentModel
	decision.RecordSession(session, currentModel)
	allow := spawnRoute{decision: decision}

	// The user deliberately picked an explicit_only model: never block it or
	// ask for a respawn on another model.
	if decision.ShouldPreserveExplicitModel(currentModel, res) {
		return allow
	}

	// Right gear already: nothing to do.
	if decision.Verdict == core.VerdictOK {
		return allow
	}

	// An uncertain downshift must never block: blocking on doubt would trade
	// a real spawn for a wrong-way demotion. Upshifts are safe to enforce even
	// when confidence is soft — under-powering a subagent is the costlier error —
	// but like every block they still need an actionable, held-free,
	// in-session target (see the policy-mode gate below).
	if decision.Verdict == core.VerdictDownshift && !decision.Confident {
		return allow
	}

	// VerdictUnknown means we could not compare (no current model, or an
	// unrecognised one). Fail open — do not block a spawn we cannot reason about.
	if decision.Verdict == core.VerdictUnknown {
		return allow
	}

	// Policy-mode gate: use the same session-only target plan as rewrite
	// adapters. KiroCrew cannot rewrite in place, but may ask for a respawn
	// only when the exact target is selectable in this session.
	plan := decision.PlanSession(core.KiroCrewCaps, res, session, currentModel)
	if plan.HoldForeign || plan.PreserveExplicit || plan.Model.ID == "" || plan.Model.ID == currentModel || !session.Contains(plan.Model.ID) {
		return allow
	}
	if len(decision.Corrections) > 0 {
		return allow
	}
	// KiroCrew cannot report quota (its hook contract is allow/block, no
	// payload), so the credit gate would always hold when quota is unknown.
	// Skip the gate: policy-mode enforcement is safe because the agent must
	// explicitly respawn with the recommended model, and the harness itself
	// enforces whatever limits it has.
	// Other checks (session membership, explicit-only, blocks) are already
	// covered by the plan gates above.
	if session.Blocks(plan.Model.ID) {
		return allow
	}

	// A confident mismatch with an actionable in-session target (downshift or
	// upshift): the caller blocks and tells the agent exactly which model to
	// respawn with.
	decision.Model = plan.Model
	return spawnRoute{decision: decision, block: true, target: plan.Model, note: decision.Summary()}
}

// Handle classifies a preToolUse event and decides allow vs block.
// Returns the policy output, a human-readable note for telemetry/stderr, and
// the full routing Decision so the runner can record telemetry without
// re-classifying. A zero Decision (Harness == "") means "not a routable spawn".
// For a spawn_run batch the Output carries one Decision per entry; the returned
// Decision is the first of them.
func Handle(ev Event, r ...core.Resolver) (Output, string, core.Decision) {
	ev.ToolName = canonicalToolName(ev.ToolName)
	var ti map[string]any
	if err := json.Unmarshal(ev.ToolInput, &ti); err != nil {
		return allow(), "", core.Decision{}
	}
	var res core.Resolver
	if len(r) > 0 {
		res = r[0]
	}

	if ev.ToolName == spawnRunTool && hasTasksBatch(ti) {
		return handleBatch(ev, ti, res)
	}

	// Only spawn tools with both model and task text are routed. Everything
	// else passes through.
	if !isSubagentSpawn(ev.ToolName, ti) {
		return allow(), "", core.Decision{}
	}
	rt := routeSpawn(ev, res, hookutil.TaskText(ti, "task", "prompt", "description"), hookutil.StringField(ti, "model"))
	if !rt.block {
		return allow(), "", rt.decision
	}
	msg := fmt.Sprintf("downshift: %s\nRespawn this subagent with model=%s (tier %s).",
		rt.note, rt.target.ID, rt.target.Tier)
	// The context-trim hint only applies when moving DOWN to a small window;
	// an upshift to a frontier model has room to spare. It follows the target
	// written, which can sit above the decision's tier (nearest in session).
	if rt.target.Tier == core.TierSmall {
		msg += smallTierHint
	}
	return Output{Block: true, Message: msg}, rt.note, rt.decision
}

// Denied is the policy-mode wire contract read by the shared hook runner:
// exit 2 with Message on stderr when blocked, exit 0 otherwise.
func (o Output) Denied() (bool, string) { return o.Block, o.Message }

func allow() Output { return Output{Block: false} }
