// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import "github.com/tiagovilasboas/downshift/internal/telemetry"

// countSpawns folds the panel counts of every session with the same helper
// the Agents panel uses (telemetry.BuildPanel). Events without a session id
// are skipped: BuildPanel reads one session at a time and has no path for them.
func countSpawns(events []telemetry.Event) (modelSwitched, effort, held, guardrail, upshift int) {
	bySession := map[string][]telemetry.Event{}
	for _, ev := range events {
		if ev.SessionID != "" {
			bySession[ev.SessionID] = append(bySession[ev.SessionID], ev)
		}
	}
	for id, evs := range bySession {
		p := telemetry.BuildPanel(evs, id)
		modelSwitched += p.ModelSwitched
		effort += p.EffortChanged
		held += p.Held
		guardrail += p.HeldGuardrail
		upshift += p.UpshiftNotApplied
	}
	return modelSwitched, effort, held, guardrail, upshift
}
