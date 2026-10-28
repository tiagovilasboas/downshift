// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"sort"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// SessionReport counts the acceptance observations of one session.
type SessionReport struct {
	SessionID string `json:"session_id"`
	Used      int    `json:"used"`
	Discarded int    `json:"discarded"`
	Unknown   int    `json:"unknown"`
}

// Report is the factory summary built from acceptance telemetry. AcceptanceRate
// is used / (used + discarded); it is nil when no run was decided either way,
// so an empty history never reads as 0% or 100%.
type Report struct {
	Used           int             `json:"used"`
	Discarded      int             `json:"discarded"`
	Unknown        int             `json:"unknown"`
	AcceptanceRate *float64        `json:"acceptance_rate"`
	Sessions       []SessionReport `json:"sessions"`
}

// BuildReport aggregates the acceptance events. Only events with the acceptance
// outcome count; a session filter, when given, keeps the rest out.
func BuildReport(events []telemetry.Event, sessionFilter string) Report {
	bySession := map[string]*SessionReport{}
	r := Report{Sessions: []SessionReport{}}
	for _, ev := range events {
		if ev.Outcome != telemetry.OutcomeAcceptance {
			continue
		}
		if sessionFilter != "" && ev.SessionID != sessionFilter {
			continue
		}
		sr := bySession[ev.SessionID]
		if sr == nil {
			sr = &SessionReport{SessionID: ev.SessionID}
			bySession[ev.SessionID] = sr
		}
		switch telemetry.AcceptanceState(ev.Acceptance) {
		case telemetry.AcceptanceUsed:
			sr.Used++
			r.Used++
		case telemetry.AcceptanceDiscarded:
			sr.Discarded++
			r.Discarded++
		default:
			sr.Unknown++
			r.Unknown++
		}
	}
	for _, sr := range bySession {
		r.Sessions = append(r.Sessions, *sr)
	}
	sort.Slice(r.Sessions, func(i, j int) bool { return r.Sessions[i].SessionID < r.Sessions[j].SessionID })
	if decided := r.Used + r.Discarded; decided > 0 {
		rate := float64(r.Used) / float64(decided)
		r.AcceptanceRate = &rate
	}
	return r
}
