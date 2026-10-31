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

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
	"github.com/tiagovilasboas/downshift/internal/quota"
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
	decision := core.RouteAgent(subPrompt, harnessID, hookutil.StringField(ti, "subagent_type"), currentModel, res)
	decision.Baseline.RequestedID = currentModel
	decision.Baseline.Guessed = fromParent

	session = withMuseCatalogInclusion(session, res)
	plan := decision.PlanSession(core.CursorCaps, res, session, currentModel)
	if plan.HoldForeign || plan.PreserveExplicit {
		return allow(), "", decision
	}
	if hold, creditHeld, exhausted := cursorMuseSparkHold(decision, plan, session, res); hold {
		// Absent marks are not credit. Exhausted and unavailable are.
		decision.Session.CreditHeld = creditHeld
		if exhausted {
			decision.Session.QuotaStatus = string(quota.Exhausted)
		} else if decision.Session.QuotaStatus == string(quota.Available) {
			decision.Session.QuotaStatus = string(quota.Unknown)
		}
		return allow(), "", decision
	}

	target := plan.Model
	if target.ID == "" || target.ID == decision.Baseline.CurrentModel.ID {
		return allow(), "", decision
	}
	if !plan.RewriteModel {
		return allow(), "", decision
	}
	// On the parent's model as baseline, only the small tier is surely
	// cheaper than what the child runs; a mid target could be an upshift.
	if fromParent && decision.Verdict == core.VerdictDownshift && target.Tier != core.TierSmall {
		decision.Baseline.GuessHeld = true
		return allow(), "", decision
	}
	if !core.CanWriteSessionID(harnessID, target.ID, plan.Session, res) {
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

// cursorMuseSpark reports whether id is the Cursor muse-spark catalog entry
// or one of its aliases. Family comes from the catalog, so a later slug in
// that family is covered without a permanent ban.
func cursorMuseSpark(id string, res core.Resolver) (core.Model, bool) {
	if id == "" || res == nil {
		return core.Model{}, false
	}
	m, ok := res.LookupByID(harnessID, id)
	if !ok || m.Family != "muse-spark" {
		return core.Model{}, false
	}
	return m, true
}

// withMuseCatalogInclusion lets an included catalog id stand for the session
// slug Cursor actually listed (often muse-spark-1.3-high). Unavailable still
// wins: a blocked muse id is not copied onto the allowlist.
func withMuseCatalogInclusion(session core.SessionList, res core.Resolver) core.SessionList {
	if !session.Known || len(session.Included) == 0 {
		return session
	}
	extra := make([]string, 0, 1)
	for _, id := range session.IDs {
		m, ok := cursorMuseSpark(id, res)
		if !ok || session.IsIncluded(id) || museBlocked(session, id, m.ID, res) {
			continue
		}
		if session.IsIncluded(m.ID) {
			extra = append(extra, id)
		}
	}
	if len(extra) == 0 {
		return session
	}
	session.Included = append(append([]string(nil), session.Included...), extra...)
	return session
}

// cursorMuseSparkHold stops a Cursor rewrite from naming muse-spark unless
// this call has positive token-pool evidence. An upshift whose strongest
// session candidate is muse, and that id is unavailable or exhausted, holds
// the whole rewrite instead of substituting a weaker model.
func cursorMuseSparkHold(decision core.Decision, plan core.RewritePlan, session core.SessionList, res core.Resolver) (hold, creditHeld, exhausted bool) {
	ids := make([]string, 0, 4)
	strongestMuse := false
	if decision.Verdict == core.VerdictUpshift {
		if id := lastSessionID(session, res); id != "" {
			if m, ok := cursorMuseSpark(id, res); ok {
				strongestMuse = true
				ids = append(ids, id, m.ID)
			}
		}
	}
	writeMuse := false
	if plan.RewriteModel {
		for _, id := range []string{plan.Model.ID, plan.WriteName} {
			if m, ok := cursorMuseSpark(id, res); ok {
				writeMuse = true
				ids = append(ids, id, m.ID)
			}
		}
	}
	if !strongestMuse && !writeMuse {
		return false, false, false
	}
	ids = uniqueIDs(ids)
	blocked := false
	for _, id := range ids {
		if museBlocked(session, id, id, res) {
			blocked = true
		}
		if session.Usage != nil && session.QuotaStatus(harnessID, id) == quota.Exhausted {
			exhausted = true
		}
	}
	if blocked || exhausted {
		return true, true, exhausted
	}
	if (writeMuse || strongestMuse) && !musePoolPositive(session, ids) {
		// Unobserved pool: do not emit muse and do not call the hold credit.
		// A non-muse rewrite already chosen by core is left alone.
		if writeMuse || !plan.RewriteModel {
			return true, false, false
		}
	}
	return false, false, false
}

func museBlocked(session core.SessionList, sessionID, catalogID string, res core.Resolver) bool {
	if session.Blocks(sessionID) || (catalogID != "" && catalogID != sessionID && session.Blocks(catalogID)) {
		return true
	}
	for _, id := range session.Exhausted {
		if id == sessionID || id == catalogID {
			return true
		}
		if m, ok := cursorMuseSpark(id, res); ok && (m.ID == catalogID || m.ID == sessionID || id == sessionID) {
			return true
		}
	}
	return false
}

func musePoolPositive(session core.SessionList, ids []string) bool {
	for _, id := range ids {
		if session.IsIncluded(id) {
			return true
		}
	}
	if session.Usage == nil {
		return false
	}
	for _, id := range ids {
		if session.QuotaStatus(harnessID, id) == quota.Available {
			return true
		}
	}
	return false
}

// lastSessionID returns the last non-inherit, non-explicit_only session id.
func lastSessionID(session core.SessionList, res core.Resolver) string {
	if !session.Known {
		return ""
	}
	last := ""
	for _, id := range session.IDs {
		if id == "" || id == "inherit" {
			continue
		}
		if res != nil && res.IsExplicitOnly(harnessID, id) {
			continue
		}
		last = id
	}
	return last
}

func uniqueIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
