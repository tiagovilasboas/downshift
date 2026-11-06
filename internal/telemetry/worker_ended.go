// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "time"

// OutcomeWorkerEnded marks one finished worker that the MCP delegate tool
// started. It is an observation, not a routing decision: Aggregate skips it,
// so it never moves decision, usage or savings counts.
const OutcomeWorkerEnded = "worker_ended"

// WorkerEnded builds the record of one finished worker: the harness that ran
// it, how it ended (done, failed, timeout, cancelled) and the token counts its
// CLI reported. Tokens only, never dollars; never the task text or a file
// path. A nil usage means the CLI reported none (for example, a worker killed
// by its time budget), and the record then carries no token fields.
func WorkerEnded(harness, outcome string, usage *TokenUsage) Event {
	ev := Event{
		CorrelationID: NewCorrelationID(),
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Source:        "mcp",
		Agent:         "worker",
		Harness:       harness,
		Outcome:       OutcomeWorkerEnded,
		WorkerOutcome: outcome,
	}
	if usage != nil {
		u := *usage
		ev.InputTokens, ev.OutputTokens = &u.InputTokens, &u.OutputTokens
		ev.CachedTokens, ev.CacheWriteTokens = &u.CachedTokens, &u.CacheWriteTokens
	}
	return ev
}
