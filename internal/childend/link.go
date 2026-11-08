// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend

import (
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// pricedCall reports that a usage record already attaches to the decision made
// for this tool call.
func pricedCall(prior []telemetry.Event, callHash string) bool {
	keys := map[string]bool{}
	for _, ev := range prior {
		if ev.ToolUseHash == callHash && !telemetry.IsObservationOutcome(ev.Outcome) {
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
			if ev.ToolUseHash == callHash && !telemetry.IsObservationOutcome(ev.Outcome) {
				callSeen = true
				break
			}
		}
	}
	var best *telemetry.Event
	var bestTime time.Time
	for i := range prior {
		ev := &prior[i]
		if telemetry.IsObservationOutcome(ev.Outcome) || telemetry.IsFailureOutcome(ev.Outcome) {
			continue // cost records and fail-open/error lines are not decisions
		}
		if ev.HasTokenUsage() || priced[decisionKey(*ev)] {
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
