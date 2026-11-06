// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package childend is the one shared recorder for how a child spawn ended, for
// every harness. An adapter parses its own payload into hookport.ChildEnd; this
// package writes the telemetry: a "resolved" observation (the model the harness
// says the child ran on), a "child_ended" observation (how it ended) and a
// "usage" record (observed tokens), each linked to the routing decision made
// for that spawn, plus the one-line child-end text. It holds no harness name and
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
// Attribution follows what actually ran. Only a decision whose rewrite was
// applied (outcome rewrite_emitted, no guardrail correction) counts as
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
//     unlinked, counted as unrouted.
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
// counted once: a transcript read and a repeated tool-call id both dedupe.
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
	priced := false // a later stop of an agent already priced (Cumulative)
	// A harness whose usage source is not proven (UsageNone or empty) records
	// the end and the status but never a cost: unverified tokens could belong
	// to the parent thread and would corrupt tokens per completed task.
	if caps, ok := core.CapsFor(ce.Harness); ok && caps.UsageSource != "" && caps.UsageSource != core.UsageNone && ce.Usage != nil {
		usage = *ce.Usage
	}
	if usage == (telemetry.TokenUsage{}) {
		return res // nothing billable reported
	}

	var target, resolved *telemetry.Event
	if ce.Source == hookport.SourceTranscript {
		// A stop event: the agent id joins this record to the launch-time
		// observation, and through it to the decision.
		if agentHash == "" {
			return res
		}
		var firstUsage *telemetry.Event
		var recorded telemetry.TokenUsage
		for i := range prior {
			ev := &prior[i]
			if ev.AgentHash != agentHash {
				continue
			}
			if ev.Outcome == telemetry.OutcomeUsage {
				if !ce.Cumulative {
					return res // this agent is already priced
				}
				if firstUsage == nil {
					firstUsage = ev
				}
				recorded = addUsage(recorded, ev.TokenUsageOrZero())
				continue
			}
			if ev.Outcome == telemetry.OutcomeResolved {
				resolved = ev
			}
		}
		if firstUsage != nil {
			// The agent stopped before and was priced then. Its transcript is
			// cumulative, so only the growth is new; an unchanged transcript (a
			// repeated stop) records nothing.
			usage = usageGrowth(usage, recorded)
			if usage == (telemetry.TokenUsage{}) {
				return res
			}
			priced = true
		}
		if resolved != nil {
			for i := range prior {
				if !telemetry.IsObservationOutcome(prior[i].Outcome) && decisionKey(prior[i]) == resolved.LinkedDecision {
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

	// The model the harness reports it resolved is what ran; without a usage
	// model, pricing on the rewrite target would credit a refuted rewrite.
	ranOn := ce.Model
	if ranOn == "" {
		ranOn = ce.ResolvedModel
	}
	ev := buildUsageEvent(ce.Harness, usage, ranOn, sessionHash, binaryVersion, target, r, now)
	// The same agent id lets a later stop see this child as already priced, so
	// a foreground child is neither double counted nor announced twice.
	ev.AgentHash = agentHash
	ev.Verification = ce.Verification
	if ce.Effort != "" {
		ev.FinalEffort = telemetry.EffortOrUnknown(ce.Effort)
	}
	if slug := gatewayRanModel(resolved, ce); slug != "" {
		markGatewayRun(&ev, slug)
	}
	telemetry.Record(ev)
	res.Usage = &ev
	if ce.IsChild && !priced {
		res.Line = telemetry.ChildEndLine(ev)
	}
	if target != nil {
		res.Note, res.Linked = "usage linked to "+target.Verdict+" decision", true
	} else {
		res.Note = "usage recorded without matching decision"
	}
	return res
}
