// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"sort"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// spawnFate is what the runner did with one spawn. The stderr line and the
// telemetry outcome both read from it, so they cannot disagree.
type spawnFate struct {
	// paused means the spawn was allowed unchanged because the user paused
	// routing for the session (`downshift session --pause`).
	paused bool
	// denied means the wire exited non-zero and the harness blocks the spawn.
	denied bool
	// note is the adapter's summary, set only when it rewrote or blocked.
	note string
}

// stderrLine is the one line that says what happened, or "" when the spawn
// ran untouched and the adapter had nothing to say.
func (f spawnFate) stderrLine(correlationID string) string {
	switch {
	case f.paused:
		return fmt.Sprintf("downshift: correlation_id=%s routing paused for this session, spawn unchanged (downshift session --resume)\n", correlationID)
	case f.denied:
		// The wire already relayed the block message on stderr.
		return ""
	case f.note != "":
		return fmt.Sprintf("downshift: correlation_id=%s %s\n", correlationID, f.note)
	}
	return ""
}

// recordOutcome sets the event outcome and reason codes. A FromDecision event
// starts as a rewrite; only the fates below change that.
func (f spawnFate) recordOutcome(event *telemetry.Event) {
	switch {
	case f.paused:
		event.Outcome = telemetry.OutcomeBaseline
		event.ReasonCodes = append(event.ReasonCodes, telemetry.ReasonPaused)
		sort.Strings(event.ReasonCodes)
	case f.denied:
		event.Outcome = telemetry.OutcomeBlocked
	case f.note == "":
		// Adapters return a note only when they rewrote or blocked the spawn.
		event.Outcome = telemetry.OutcomeAllow
	}
}

// recordedDecisions returns the routing decisions one spawn produced: every
// entry when the adapter output reports them (a spawn_run batch), otherwise
// the single decision returned by the handler.
func recordedDecisions(first core.Decision, out any) []core.Decision {
	if multi, ok := out.(interface{ Decisions() []core.Decision }); ok && multi.Decisions() != nil {
		return multi.Decisions()
	}
	return []core.Decision{first}
}
