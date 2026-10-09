// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package claudecode — PostToolUse real-cost linkage (v1).
//
// A Claude Code PostToolUse hook fires when the Task/Agent tool returns. For an
// async subagent that is the LAUNCH, not the completion (captured 2026-10-06:
// tool_response is {isAsync, status, agentId, resolvedModel, ...}, duration_ms
// a few ms), so the payload carries no token usage and the "usage" path below
// stays idle until a harness reports tokens. What it does carry is
// tool_response.resolvedModel, the model the harness chose for the child:
// recordResolved turns that into an observed honor/refute signal for the
// rewrite. Token usage is still parsed defensively
// (see telemetry.TokenUsageFromPayload).
//
// v1 writes a NEW event with outcome "usage" and never modifies history in
// place: the original PreToolUse decision event keeps its normalised savings
// estimate, and the usage event carries the priced real dollars. Aggregate
// excludes "usage"/"baseline" outcomes from every decision count while still
// accumulating their RealSavedUSD/RealCostEvents.
//
// Linkage rule (approximation — documented limitations below): the usage
// record attaches to the most recent decision event that (a) belongs to
// harness "claude-code", (b) has not been priced by an earlier usage record
// (linked_decision, keyed by correlation id), (c) parses to a timestamp
// within the 24h before the hook fires, and (d) matches by hashed session id
// when the payload carries one, else by harness + final model. The verdict is
// copied from the matched decision, or "UNKNOWN" when nothing matches.
//
// Pricing follows what actually ran. Only a decision whose rewrite was
// applied (outcome rewrite_emitted, no guardrail correction) is priced as
// routed-vs-requested. An allow, held or blocked decision left the spawn on
// its requested model, so routed = baseline = requested model and real
// savings are zero. A model reported by the payload always wins as routed.
//
// Known limitations:
//   - Session matching is mostly future-proofing today: the PreToolUse path
//     only recently started persisting hashed session ids (Event.SessionID),
//     so older log lines match via the harness+final_model fallback, which
//     can misattribute usage when several spawns share one model.
//   - Matching is heuristic, not causal: concurrent same-model spawns in one
//     session link to the latest decision, which may be the wrong sibling.
package claudecode

import (
	"encoding/json"
	"time"

	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/sensor"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// linkWindow bounds how far back a usage record may reach for its decision.
// Older than this, the spend almost certainly belongs to a previous session.
const linkWindow = 24 * time.Hour

// SessionIdentifier returns the raw session id supplied by the harness, if
// any. runHookAdapter hashes it via telemetry.HashSessionID before durable
// logging, so future PostToolUse records can link by session.
func (ev Event) SessionIdentifier() string { return ev.SessionID }

// stringFieldAny reads a string value from a decoded payload map.
func stringFieldAny(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// parseEventTime parses an event timestamp with the same layouts as
// telemetry.FilterByDays (nano first, legacy fallback). Unparseable
// timestamps exclude the event from linkage.
func parseEventTime(ts string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// linkTarget finds the most recent decision event a usage record attaches to.
// sessionHash is the already-hashed payload session ("" when absent);
// modelID is the model reported by the payload ("" when absent). Returns nil
// when nothing matches.
func linkTarget(prior []telemetry.Event, sessionHash, modelID string, now time.Time) *telemetry.Event {
	return linkTargetBy(prior, sessionHash, modelID, now, telemetry.OutcomeUsage)
}

// linkTargetBy is linkTarget where a decision already linked by a record of
// the given outcome is skipped, so each observation kind links once per decision.
func linkTargetBy(prior []telemetry.Event, sessionHash, modelID string, now time.Time, linkedBy string) *telemetry.Event {
	priced := make(map[string]bool)
	for _, ev := range prior {
		if ev.Outcome == linkedBy && ev.LinkedDecision != "" {
			priced[ev.LinkedDecision] = true
		}
	}
	var best *telemetry.Event
	var bestTime time.Time
	for i := range prior {
		ev := &prior[i]
		if telemetry.IsCostOnlyOutcome(ev.Outcome) || ev.Outcome == "error" {
			continue // cost records and fail-open errors are not decisions
		}
		if ev.HasRealCost() || priced[decisionKey(*ev)] {
			continue // already priced — one decision prices once
		}
		if ev.Harness != harnessID {
			continue
		}
		t, ok := parseEventTime(ev.Timestamp)
		if !ok {
			continue
		}
		if age := now.Sub(t); age < 0 || age > linkWindow {
			continue
		}
		if sessionHash != "" {
			if ev.SessionID == "" || ev.SessionID != sessionHash {
				continue
			}
		} else if modelID != "" {
			if ranModel(*ev) != modelID {
				continue
			}
		}
		if best == nil || !t.Before(bestTime) {
			best = ev
			bestTime = t
		}
	}
	return best
}

// decisionKey identifies a decision event for once-only pricing: its
// correlation id, or timestamp+session for lines written without one.
func decisionKey(ev telemetry.Event) string { return telemetry.DecisionKey(ev) }

// ranModel is the model the child actually ran on according to the decision
// record: the rewrite target when the rewrite was applied, otherwise the
// requested model.
func ranModel(ev telemetry.Event) string {
	if ev.Outcome == telemetry.OutcomeRewriteEmitted && telemetry.AppliedRewrite(ev.Outcome, ev.Corrections) {
		return ev.ToModel
	}
	return ev.FromModel
}

// buildUsageEvent assembles the NEW cost record. Provenance (harness,
// complexity, models, verdict, tier, session) is copied from the matched
// decision when present; unmatched records use explicit "unknown" markers.
// Token pointers are set only for positive counts so absent usage stays
// absent in JSON (omitempty), and costs are filled only when the spend is
// real (tokens > 0) and the routed model resolved against the catalog.
func buildUsageEvent(usage telemetry.TokenUsage, payloadModel, sessionHash, binaryVersion string, target *telemetry.Event, r core.Resolver, now time.Time) telemetry.Event {
	ev := telemetry.Event{
		CorrelationID: telemetry.NewCorrelationID(),
		Timestamp:     now.UTC().Format(time.RFC3339Nano),
		Source:        "hook",
		Agent:         "subagent",
		Harness:       harnessID,
		Outcome:       telemetry.OutcomeUsage,
		PolicyVersion: telemetry.PolicyVersion,
		BinaryVersion: binaryVersion,
		Verdict:       "UNKNOWN",
		FromModel:     "unknown",
		ToModel:       telemetry.ModelOrUnknown(payloadModel),
	}
	if target != nil {
		ev.Harness = target.Harness
		ev.Complexity = target.Complexity
		ev.FromModel = target.FromModel
		ev.ToModel = ranModel(*target)
		ev.Verdict = target.Verdict
		ev.Tier = target.Tier
		ev.SessionID = target.SessionID
		ev.LinkedDecision = decisionKey(*target)
		if payloadModel != "" {
			ev.ToModel = telemetry.ModelOrUnknown(payloadModel)
		}
	}
	if ev.SessionID == "" {
		ev.SessionID = sessionHash
	}
	if usage.InputTokens > 0 {
		v := usage.InputTokens
		ev.InputTokens = &v
	}
	if usage.OutputTokens > 0 {
		v := usage.OutputTokens
		ev.OutputTokens = &v
	}
	if usage.CachedTokens > 0 {
		v := usage.CachedTokens
		ev.CachedTokens = &v
	}

	// Price only real spend against a catalog-resolved routed model.
	routedID := payloadModel
	if routedID == "" && target != nil {
		routedID = ranModel(*target)
	}
	if r == nil || routedID == "" || routedID == "unknown" {
		return ev // tokens without prices: counted as neither real nor fake
	}
	routed, ok := r.LookupByID(harnessID, routedID)
	if !ok {
		return ev
	}
	baseline := routed // fallback: unknown baseline prices as zero savings
	if target != nil && target.FromModel != "" && target.FromModel != "unknown" {
		if b, ok := r.LookupByID(harnessID, target.FromModel); ok {
			baseline = b
		}
	}
	if usage.InputTokens+usage.OutputTokens+usage.CachedTokens > 0 {
		telemetry.FillRealCost(&ev, baseline, routed)
	}
	return ev
}

// HandlePostToolUse processes one Claude Code PostToolUse stdin payload:
// it extracts the reported model, session hint, and token usage, links to
// the most recent matching decision event, appends a NEW "usage" event to
// the hermetic event log (DOWNSHIFT_EVENT_LOG respected via telemetry), and
// returns the neutral PostToolUse response ({} — this hook cannot steer, so
// there is nothing to rewrite).
//
// Fail-open: any parse, lookup, or write failure returns the neutral
// response with linked=false. The spawn already completed, so nothing is
// ever blocked; the boolean only reports whether a decision was linked.
func HandlePostToolUse(raw []byte, binaryVersion string, r core.Resolver) (Output, string, bool) {
	neutral := Output{}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return neutral, "", false
	}
	sessionHash := telemetry.HashSessionID(stringFieldAny(payload, "session_id", "sessionId", "sessionID"))
	now := time.Now().UTC()
	prior, _ := telemetry.ReadEvents() // missing/unreadable log: link nothing
	recordResolved(payload, sessionHash, binaryVersion, prior, r, now)

	// Context Sensor observation: track tool execution volume safely without prompt text
	recordSensorToolObservation(payload, sessionHash)

	usage := telemetry.TokenUsageFromPayload(payload)
	if usage == (telemetry.TokenUsage{}) {
		return neutral, "", false // nothing billable reported
	}
	model := stringFieldAny(payload, "model", "model_id")
	target := linkTarget(prior, sessionHash, model, now)
	ev := buildUsageEvent(usage, model, sessionHash, binaryVersion, target, r, now)
	telemetry.Record(ev)
	if target != nil {
		return neutral, "usage linked to " + target.Verdict + " decision", true
	}
	return neutral, "usage recorded without matching decision", false
}

// resolvedModel reads the model the harness chose for the spawn. Claude Code
// reports it as tool_response.resolvedModel when it launches a subagent; this
// is the only post-spawn model signal the hook receives (the launch payload
// carries no token usage for async subagents).
func agentID(payload map[string]any) string {
	resp, _ := payload["tool_response"].(map[string]any)
	return stringFieldAny(resp, "agentId", "agent_id")
}

func resolvedModel(payload map[string]any) string {
	resp, _ := payload["tool_response"].(map[string]any)
	return stringFieldAny(resp, "resolvedModel", "resolved_model")
}

// recordResolved appends a "resolved" observation for the applied rewrite this
// spawn belongs to. It records nothing when the payload has no resolved model,
// the session is unknown, or no applied rewrite matches: there is nothing to
// confirm or refute. The comparison goes through the catalog so a dated id
// (claude-haiku-4-5-20251001) matches the id the hook wrote (claude-haiku-4-5);
// without a catalog match RewriteHonored stays unset rather than guessing.
func recordResolved(payload map[string]any, sessionHash, binaryVersion string, prior []telemetry.Event, r core.Resolver, now time.Time) {
	resolved := resolvedModel(payload)
	if resolved == "" || sessionHash == "" {
		return
	}
	target := linkTargetBy(prior, sessionHash, "", now, telemetry.OutcomeResolved)
	if target == nil || !telemetry.IsAppliedShift(*target) {
		return
	}
	ev := telemetry.Event{
		CorrelationID:  telemetry.NewCorrelationID(),
		Timestamp:      now.UTC().Format(time.RFC3339Nano),
		Source:         "hook",
		Agent:          "subagent",
		Harness:        target.Harness,
		Outcome:        telemetry.OutcomeResolved,
		PolicyVersion:  telemetry.PolicyVersion,
		BinaryVersion:  binaryVersion,
		Verdict:        "UNKNOWN",
		FromModel:      target.ToModel,
		ToModel:        telemetry.ModelOrUnknown(resolved),
		SessionID:      target.SessionID,
		LinkedDecision: decisionKey(*target),
		AgentHash:      telemetry.HashSessionID(agentID(payload)),
	}
	if r != nil {
		got, okGot := r.LookupByID(harnessID, resolved)
		want, okWant := r.LookupByID(harnessID, target.ToModel)
		if okGot && okWant {
			honored := got.ID == want.ID
			ev.RewriteHonored = &honored
		}
	}
	telemetry.Record(ev)
}

func recordSensorToolObservation(payload map[string]any, sessionHash string) {
	if sessionHash == "" || !nativeContextObserveOn() {
		return
	}
	st, err := sensor.DefaultStore()
	if err != nil {
		return
	}
	toolName := stringFieldAny(payload, "tool_name", "toolName")
	if toolName == "" {
		toolName = "tool"
	}

	var outputBytes []byte
	isError := false
	exitCode := -1
	if resp, ok := payload["tool_response"].(map[string]any); ok {
		if content, ok := resp["content"].(string); ok {
			outputBytes = []byte(content)
		}
		if status, ok := resp["status"].(string); ok && (status == "error" || status == "failed") {
			isError = true
		}
		if code, ok := exitCodeField(resp); ok {
			exitCode = code
		}
	}
	_ = st.RecordToolResult(sessionHash, harnessID, toolName, outputBytes, exitCode, isError)
}

// nativeContextObserveOn is the persisted native-provider flag. Missing
// config uses the default (enabled, provider native). A read error fails
// closed. The stored record is observe-mode only; this hook does not
// replace the tool result the model sees.
func nativeContextObserveOn() bool {
	cfg, err := contextopt.LoadConfig()
	if err != nil || !cfg.Enabled {
		return false
	}
	return cfg.Provider == "" || cfg.Provider == "native"
}

func exitCodeField(resp map[string]any) (int, bool) {
	for _, key := range []string{"exit_code", "exitCode"} {
		n, ok := resp[key].(float64)
		if !ok || n < 0 || n > 255 || n != float64(int(n)) {
			continue
		}
		return int(n), true
	}
	return -1, false
}
