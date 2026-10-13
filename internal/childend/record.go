// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package childend is the one shared recorder for how a child spawn ended, for
// every harness. An adapter parses its own payload into hookport.ChildEnd; this
// package writes the telemetry: a "resolved" observation (the model the harness
// says the child ran on), a "child_ended" observation (how it ended) and a
// "usage" record (priced tokens), each linked to the routing decision made for
// that spawn, plus the one-line child-end text. It holds no harness name and
// reads no prompt text.
//
// Linkage rule (approximation — limits below): a record attaches to the most
// recent decision event that (a) belongs to the same harness, (b) has not
// already been linked by a record of the same kind, (c) is within the 24h
// before the hook fires, and (d) matches by hashed session id when the payload
// carries one, else by harness + final model. When the payload carries a
// tool-call id, the decision recorded for that same call wins outright and
// decisions recorded for other calls are never candidates.
//
// Pricing follows what actually ran. Only a decision whose rewrite was applied
// (outcome rewrite_emitted, no guardrail correction) is priced as
// routed-vs-requested. An allow, held or blocked decision left the spawn on its
// requested model, so routed = baseline = requested and real savings are zero.
// A model reported by the harness always wins as routed.
//
// Known limitations:
//   - Without a tool-call id (older decision lines, or a payload that omits it)
//     matching is heuristic: concurrent spawns in one session link to the
//     latest decision, which may be the wrong sibling.
//   - A harness that sends neither session nor tool-call id links nothing
//     (resolved and child_ended need one of them); its usage is still recorded,
//     unlinked, priced as unrouted.
package childend

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// linkWindow bounds how far back a record may reach for its decision. Older
// than this, the spend almost certainly belongs to a previous session.
const linkWindow = 24 * time.Hour

// Result is what Record did, for the caller's stderr note and wire output.
type Result struct {
	// Note is the stderr note ("" when no usage was recorded).
	Note string
	// Linked reports that a usage record attached to a decision.
	Linked bool
	// Usage is the usage record written, nil when none was.
	Usage *telemetry.Event
	// Line is the child-end line for the record just written, "" when there
	// is none to show (no usage, not a child, nothing known). Whether the
	// harness may display it is the caller's decision (CanShowChildLine).
	Line string
}

// Record writes the telemetry for one child end. It is fail-open: any lookup
// or write problem records less and never returns an error. The same child is
// priced once: a transcript read and a repeated tool-call id both dedupe.
func Record(ce hookport.ChildEnd, binaryVersion string, r core.Resolver) Result {
	var res Result
	if ce.Harness == "" {
		return res
	}
	sessionHash := telemetry.HashSessionID(ce.SessionID)
	callHash := telemetry.HashSessionID(ce.ToolUseID)
	agentHash := telemetry.HashSessionID(ce.AgentKey)
	now := time.Now().UTC()
	prior, _ := telemetry.ReadEvents() // missing/unreadable log: link nothing

	if ce.Source != hookport.SourceTranscript {
		recordResolved(ce, sessionHash, callHash, agentHash, binaryVersion, prior, r, now)
		recordChildEnd(ce, sessionHash, callHash, agentHash, binaryVersion, prior, now)
	}

	var usage telemetry.TokenUsage
	// A harness whose usage source is not proven (UsageNone or empty) records
	// the end and the status but never a cost: unverified tokens could belong
	// to the parent thread and would corrupt cost per completed task.
	if caps, ok := core.CapsFor(ce.Harness); ok && caps.UsageSource != "" && caps.UsageSource != core.UsageNone && ce.Usage != nil {
		usage = *ce.Usage
	}
	if usage == (telemetry.TokenUsage{}) {
		return res // nothing billable reported
	}

	var target *telemetry.Event
	if ce.Source == hookport.SourceTranscript {
		// A stop event: the agent id joins this record to the launch-time
		// observation, and through it to the decision.
		if agentHash == "" {
			return res
		}
		var resolved *telemetry.Event
		for i := range prior {
			ev := &prior[i]
			if ev.AgentHash != agentHash {
				continue
			}
			if ev.Outcome == telemetry.OutcomeUsage {
				return res // this agent is already priced
			}
			if ev.Outcome == telemetry.OutcomeResolved {
				resolved = ev
			}
		}
		if resolved != nil {
			for i := range prior {
				if !telemetry.IsCostOnlyOutcome(prior[i].Outcome) && decisionKey(prior[i]) == resolved.LinkedDecision {
					target = &prior[i]
				}
			}
		}
	} else {
		if callHash != "" && pricedCall(prior, callHash) {
			return res // the same spawn was already priced
		}
		target = LinkTargetBy(ce.Harness, prior, sessionHash, callHash, ce.Model, now, telemetry.OutcomeUsage)
	}

	ev := buildUsageEvent(ce.Harness, usage, ce.Model, sessionHash, binaryVersion, target, r, now)
	// The same agent id lets a later stop see this child as already priced, so
	// a foreground child is neither double counted nor announced twice.
	ev.AgentHash = agentHash
	ev.Verification = ce.Verification
	telemetry.Record(ev)
	res.Usage = &ev
	if ce.IsChild {
		res.Line = telemetry.ChildEndLine(ev)
	}
	if target != nil {
		res.Note, res.Linked = "usage linked to "+target.Verdict+" decision", true
	} else {
		res.Note = "usage recorded without matching decision"
	}
	return res
}

// pricedCall reports that a usage record already attaches to the decision made
// for this tool call.
func pricedCall(prior []telemetry.Event, callHash string) bool {
	keys := map[string]bool{}
	for _, ev := range prior {
		if ev.ToolUseHash == callHash && !telemetry.IsCostOnlyOutcome(ev.Outcome) {
			keys[decisionKey(ev)] = true
		}
	}
	if len(keys) == 0 {
		return false
	}
	for _, ev := range prior {
		if ev.Outcome == telemetry.OutcomeUsage && keys[ev.LinkedDecision] {
			return true
		}
	}
	return false
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

// LinkTargetBy finds the most recent decision event a post-spawn record
// attaches to. A decision already linked by a record of outcome linkedBy is
// skipped, so each observation kind links once per decision. sessionHash is the
// hashed payload session ("" when absent); callHash the hashed tool-call id;
// modelID the model the harness reported ("" when absent). Returns nil when
// nothing matches.
func LinkTargetBy(harness string, prior []telemetry.Event, sessionHash, callHash, modelID string, now time.Time, linkedBy string) *telemetry.Event {
	priced := make(map[string]bool)
	for _, ev := range prior {
		if ev.Outcome == linkedBy && ev.LinkedDecision != "" {
			priced[ev.LinkedDecision] = true
		}
	}
	// Once any decision carries this call's id, only that decision may match.
	callSeen := false
	if callHash != "" {
		for _, ev := range prior {
			if ev.ToolUseHash == callHash && !telemetry.IsCostOnlyOutcome(ev.Outcome) {
				callSeen = true
				break
			}
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
		if ev.Harness != harness {
			continue
		}
		t, ok := parseEventTime(ev.Timestamp)
		if !ok {
			continue
		}
		if age := now.Sub(t); age < 0 || age > linkWindow {
			continue
		}
		if callHash != "" && (ev.ToolUseHash != "" || callSeen) {
			if ev.ToolUseHash == callHash {
				return ev // the decision made for this exact call
			}
			continue // a sibling call's decision
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
// Token pointers are set only for positive counts so absent usage stays absent
// in JSON (omitempty), and costs are filled only when the spend is real
// (tokens > 0) and the routed model resolved against the catalog.
func buildUsageEvent(harness string, usage telemetry.TokenUsage, payloadModel, sessionHash, binaryVersion string, target *telemetry.Event, r core.Resolver, now time.Time) telemetry.Event {
	ev := telemetry.Event{
		CorrelationID: telemetry.NewCorrelationID(),
		Timestamp:     now.UTC().Format(time.RFC3339Nano),
		Source:        "hook",
		Agent:         "subagent",
		Harness:       harness,
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
	routed, ok := r.LookupByID(harness, routedID)
	if !ok {
		return ev
	}
	if usage == (telemetry.TokenUsage{}) {
		return ev
	}
	actual := telemetry.CostUSD(routed, usage)
	ev.ActualCostUSD = &actual
	// Observed routed spend does not establish the requested baseline. Keep
	// it absent unless the linked decision names a catalog-resolved model.
	if target != nil && target.FromModel != "" && target.FromModel != "unknown" {
		if baseline, ok := r.LookupByID(harness, target.FromModel); ok {
			base := telemetry.CostUSD(baseline, usage)
			ev.BaselineCostUSD = &base
			ev.BaselineSource = telemetry.BaselineRequested
		}
	}
	return ev
}

// recordResolved appends a "resolved" observation for the decision this spawn
// belongs to. It records nothing when the event has no model, no session or
// call id, or no decision matches. A decision found by the session heuristic
// must be an applied rewrite; one found by its exact tool-call id may be any
// decision (allow, held), so the child's later cost still links to it.
// RewriteHonored is set only for applied rewrites: there is nothing to confirm
// or refute otherwise. The comparison goes through the catalog so a dated id
// (claude-haiku-4-5-20251001) matches the id the hook wrote (claude-haiku-4-5);
// without a catalog match RewriteHonored stays unset rather than guessing.
func recordResolved(ce hookport.ChildEnd, sessionHash, callHash, agentHash, binaryVersion string, prior []telemetry.Event, r core.Resolver, now time.Time) {
	if ce.ResolvedModel == "" || (sessionHash == "" && callHash == "") {
		return
	}
	target := LinkTargetBy(ce.Harness, prior, sessionHash, callHash, "", now, telemetry.OutcomeResolved)
	if !linkable(target, callHash) {
		return
	}
	applied := telemetry.IsAppliedShift(*target)
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
		FromModel:      ranModel(*target),
		ToModel:        telemetry.ModelOrUnknown(ce.ResolvedModel),
		SessionID:      target.SessionID,
		LinkedDecision: decisionKey(*target),
		AgentHash:      agentHash,
	}
	if r != nil && applied {
		got, okGot := r.LookupByID(ce.Harness, ce.ResolvedModel)
		want, okWant := r.LookupByID(ce.Harness, target.ToModel)
		if okGot && okWant {
			honored := got.ID == want.ID
			ev.RewriteHonored = &honored
		}
	}
	telemetry.Record(ev)
}

// linkable reports whether a post-spawn observation may attach to target: an
// exact tool-call id match always may; a heuristic match only for an applied
// rewrite, where a wrong sibling is least likely to matter.
func linkable(target *telemetry.Event, call string) bool {
	if target == nil {
		return false
	}
	if call != "" && target.ToolUseHash == call {
		return true
	}
	return telemetry.IsAppliedShift(*target)
}

// recordChildEnd records how a spawned child ended. Only the closed status is
// stored: error text and tool output are never read, so nothing prompt-like
// reaches telemetry.
func recordChildEnd(ce hookport.ChildEnd, sessionHash, callHash, agentHash, binaryVersion string, prior []telemetry.Event, now time.Time) {
	if ce.Status == "" || ce.Status == telemetry.ChildUnknown || (sessionHash == "" && callHash == "") {
		return
	}
	target := LinkTargetBy(ce.Harness, prior, sessionHash, callHash, "", now, telemetry.OutcomeChildEnded)
	if !linkable(target, callHash) {
		return
	}
	telemetry.Record(telemetry.Event{
		CorrelationID:  telemetry.NewCorrelationID(),
		Timestamp:      now.UTC().Format(time.RFC3339Nano),
		Source:         "hook",
		Agent:          "subagent",
		Harness:        target.Harness,
		Outcome:        telemetry.OutcomeChildEnded,
		PolicyVersion:  telemetry.PolicyVersion,
		BinaryVersion:  binaryVersion,
		Verdict:        "UNKNOWN",
		FromModel:      ranModel(*target),
		ToModel:        telemetry.ModelOrUnknown(""),
		SessionID:      target.SessionID,
		LinkedDecision: decisionKey(*target),
		AgentHash:      agentHash,
		ChildStatus:    ce.Status,
	})
}
