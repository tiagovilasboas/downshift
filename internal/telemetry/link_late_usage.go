// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// linkLateUsage joins a usage record to its decision when the usage was
// written before the launch observation that names the decision.
//
// In Claude Code the child's SubagentStop fires a few milliseconds before the
// Agent tool's PostToolUse, so the usage record (priced from the transcript)
// lands in the log before the "resolved" record that links the agent to its
// decision. Written that way the usage has no decision: it counts in the
// totals but can never count as saved, so every live view reads zero. The two
// records share the agent hash, so the join is made here, once, for every
// reader and for the history already on disk. Only an empty link is filled; a
// usage that already names a decision, or an agent with no resolved record,
// is left as written. The log is never rewritten.
func linkLateUsage(events []Event) {
	type agentKey struct{ harness, hash string }
	linked := map[agentKey]string{}
	for _, ev := range events {
		if ev.Outcome == OutcomeResolved && ev.AgentHash != "" && ev.LinkedDecision != "" {
			linked[agentKey{ev.Harness, ev.AgentHash}] = ev.LinkedDecision
		}
	}
	if len(linked) == 0 {
		return
	}
	decisions := map[linkKey]Event{}
	for _, ev := range events {
		if IsDecisionRow(ev) {
			decisions[linkKey{ev.Harness, DecisionKey(ev)}] = ev
		}
	}
	for i := range events {
		ev := &events[i]
		if ev.Outcome != OutcomeUsage || ev.LinkedDecision != "" || ev.AgentHash == "" {
			continue
		}
		key, ok := linked[agentKey{ev.Harness, ev.AgentHash}]
		if !ok {
			continue
		}
		d, ok := decisions[linkKey{ev.Harness, key}]
		if !ok {
			continue
		}
		// The same provenance buildUsageEvent copies from a matched decision.
		ev.LinkedDecision = key
		ev.Complexity = d.Complexity
		ev.FromModel = d.FromModel
		ev.Verdict = d.Verdict
		ev.Tier = d.Tier
		if ev.SessionID == "" {
			ev.SessionID = d.SessionID
		}
	}
}
