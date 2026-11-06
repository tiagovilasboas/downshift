// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend

import (
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

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
			honored := rewriteHonored(got, want)
			ev.RewriteHonored = &honored
		}
	}
	telemetry.Record(ev)
}

// rewriteHonored reports whether the model the harness ran satisfies the model
// the hook wrote. An exact id match always does. Otherwise the observed model
// counts when it is a different version of the same family in the same catalog
// tier, for example a harness alias that resolves to a newer release than the
// catalog id the hook wrote. A different family or tier never does.
func rewriteHonored(got, want core.Model) bool {
	if got.ID == want.ID {
		return true
	}
	if got.Tier != want.Tier {
		return false
	}
	gf, wf := familyOf(got), familyOf(want)
	return gf != "" && gf == wf
}

// familyOf is the catalog family, or the id without its trailing version
// segments (claude-haiku-5-5 -> claude-haiku) for an entry that declares none.
func familyOf(m core.Model) string {
	if m.Family != "" {
		return m.Family
	}
	parts := strings.Split(m.ID, "-")
	n := len(parts)
	for n > 1 && isVersionPart(parts[n-1]) {
		n--
	}
	if n == len(parts) {
		return "" // no version suffix: nothing to compare across versions
	}
	return strings.Join(parts[:n], "-")
}

func isVersionPart(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
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
		ToModel:        childEndModel(ce),
		SessionID:      target.SessionID,
		LinkedDecision: decisionKey(*target),
		AgentHash:      agentHash,
		ChildStatus:    ce.Status,
	})
}
