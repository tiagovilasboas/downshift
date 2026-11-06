// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"time"
)

// Outcome values for hook routing events.
const (
	// OutcomeRewriteEmitted marks a hook response that changed the spawn's model.
	OutcomeRewriteEmitted = "rewrite_emitted"
	// OutcomeAllow marks a routing decision whose hook response left the spawn unchanged.
	OutcomeAllow = "allow"
	// OutcomeBlocked marks a policy-mode hook (KiroCrew) that denied the
	// spawn and asked the agent to respawn on another model. Nothing was
	// rewritten.
	OutcomeBlocked = "blocked"
	// OutcomeFailOpen marks a hook that could not answer (malformed input,
	// internal error, oversized payload): the spawn was allowed unchanged and
	// the hook's own fail-open response was emitted. It records no decision.
	OutcomeFailOpen = "fail_open"
)

// IsFailureOutcome reports whether an outcome is a hook failure rather than a
// routing decision: an error record or a fail-open answer. Failure records
// count in no routed/decision aggregate, exactly like errors.
func IsFailureOutcome(outcome string) bool {
	return outcome == "error" || outcome == OutcomeFailOpen
}

// AppliedRewrite reports whether a decision event changed the spawn: the
// hook emitted a rewrite and no guardrail held the decision. Allow events,
// held decisions (corrections present) and blocks never count as savings.
// Lines written before the outcome field existed (empty outcome) count as
// emitted for backward compatibility.
func AppliedRewrite(outcome string, corrections []string) bool {
	if len(corrections) > 0 {
		return false
	}
	return outcome == OutcomeRewriteEmitted || outcome == ""
}

// AppliedRouting reports whether a decision event effectively changed the
// model that will run. This is broader than AppliedRewrite: it also counts
// blocked outcomes where the harness is forced to respawn on a different
// model (e.g. KiroCrew's allow/block-only contract).
func AppliedRouting(ev Event) bool {
	if len(ev.Corrections) > 0 {
		return false
	}
	// Classic rewrite: hook modified the spawn in place
	if ev.Outcome == OutcomeRewriteEmitted || ev.Outcome == "" {
		return true
	}
	// Blocked with model change: harness must respawn on the recommended model
	if ev.Outcome == OutcomeBlocked && ev.ToModel != "" && ev.FromModel != "" && ev.ToModel != ev.FromModel {
		return true
	}
	return false
}

// MarkUnchanged rewrites final_model/final_reasoning_effort on an event
// whose hook response left the spawn unchanged (allow or baseline), so the
// log records what the child actually runs on. The recommendation moves to
// recommended_model. Rewrite and blocked events are left as they are.
func MarkUnchanged(ev *Event) {
	if ev.Outcome != OutcomeAllow && ev.Outcome != OutcomeBaseline {
		return
	}
	if ev.ToModel != ev.FromModel {
		ev.RecommendedModel = ev.ToModel
	}
	ev.ToModel = ev.FromModel
	ev.FinalEffort = ev.RequestedEffort
}

// Failure records a hook fail-open path without accepting raw input or errors
// that could contain prompt content.
func Failure(correlationID, binaryVersion, harness, errorCode string) Event {
	return Event{
		CorrelationID: correlationID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Source:        "hook",
		Agent:         "subagent",
		Harness:       harness,
		PolicyVersion: PolicyVersion,
		BinaryVersion: binaryVersion,
		Outcome:       "error",
		ErrorCode:     errorCode,
	}
}

// FailOpen records one harness answer that could not route a spawn: the hook
// emitted its fail-open response and the child ran unchanged. outcome is
// "fail_open" (never "error") so fail-open rates stay countable apart from
// hard failures; cause is a short code such as the wire's "INVALID_REQUEST",
// never raw input.
func FailOpen(correlationID, binaryVersion, harness, cause string) Event {
	return Event{
		CorrelationID: correlationID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Source:        "hook",
		Agent:         "subagent",
		Harness:       harness,
		PolicyVersion: PolicyVersion,
		BinaryVersion: binaryVersion,
		Outcome:       OutcomeFailOpen,
		ErrorCode:     cause,
	}
}

// WithInvalidField returns the event with the name of the payload field that
// failed validation. Only the field name is kept, never its value.
func (e Event) WithInvalidField(field string) Event {
	e.InvalidField = field
	return e
}
