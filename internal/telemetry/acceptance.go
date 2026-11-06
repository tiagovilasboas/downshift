// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"time"
)

// OutcomeAcceptance marks an event that records developer acceptance of
// a subagent's changes (e.g. preserved in git commit vs reverted).
const OutcomeAcceptance = "acceptance"

// AcceptanceState represents whether code delivered by subagents was accepted or discarded.
type AcceptanceState string

const (
	AcceptanceUsed      AcceptanceState = "used"
	AcceptanceDiscarded AcceptanceState = "discarded"
	AcceptanceUnknown   AcceptanceState = "unknown"
)

// RecordAcceptance writes an acceptance observation event for the given session.
func RecordAcceptance(sessionID string, state AcceptanceState, harness string) error {
	if sessionID == "" {
		return nil
	}
	ev := Event{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Source:     "git",
		Harness:    harness,
		SessionID:  sessionID,
		Outcome:    OutcomeAcceptance,
		Acceptance: string(state),
	}
	return AppendTo(defaultEventPath(), ev)
}
