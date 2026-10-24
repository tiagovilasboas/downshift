// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"sort"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// sessionSummary is the metadata for one session in the /api/sessions list.
// Its counts and savings are the session's accounting.Numbers, the same ones
// /api/session/{id} renders.
type sessionSummary struct {
	SessionID         string       `json:"session_id"`
	Harness           string       `json:"harness"`
	FirstSeen         string       `json:"first_seen"`
	LastSeen          string       `json:"last_seen"`
	SpawnCount        int          `json:"spawn_count"`
	Switched          int          `json:"switched"`
	ModelSwitched     int          `json:"model_switched"`
	EffortChanged     int          `json:"effort_changed"`
	Held              int          `json:"held"`
	HeldGuardrail     int          `json:"held_guardrail"`
	UpshiftNotApplied int          `json:"upshift_not_applied"`
	TokensSaved       int64        `json:"tokens_saved"`
	Tokens            tokensDetail `json:"tokens"`
}

// tokensDetail exposes routing and compactor savings for one session.
type tokensDetail struct {
	Routing   telemetry.TokenSaving `json:"routing"`
	Compactor *compactorWire        `json:"compactor,omitempty"`
}

// sessionsResponse is the /api/sessions response. Report is the report block.
type sessionsResponse struct {
	Sessions  []sessionSummary   `json:"sessions"`
	Harnesses []string           `json:"harnesses"`
	Report    accounting.Numbers `json:"report"`
}

// buildSessionsResponse lists the sessions in events, most recent first. Each
// row takes its figures from the session's Numbers.
func buildSessionsResponse(events []telemetry.Event, execEvents []execwrap.Event, report accounting.Numbers) sessionsResponse {
	sessions := make(map[string]*sessionSummary)
	harnessSet := make(map[string]bool)

	for _, ev := range events {
		if !telemetry.ValidSessionHash(ev.SessionID) {
			continue
		}
		sum, exists := sessions[ev.SessionID]
		if !exists {
			sum = &sessionSummary{SessionID: ev.SessionID, Harness: ev.Harness, FirstSeen: ev.Timestamp, LastSeen: ev.Timestamp}
			sessions[ev.SessionID] = sum
		}
		if ev.Timestamp > sum.LastSeen {
			sum.LastSeen = ev.Timestamp
		}
		if ev.Timestamp < sum.FirstSeen {
			sum.FirstSeen = ev.Timestamp
		}
		harnessSet[ev.Harness] = true
	}

	var list []sessionSummary
	for id, sum := range sessions {
		n := sessionNumbers(events, execEvents, id)
		sum.SpawnCount = len(telemetry.BuildPanel(events, id).Spawns)
		sum.Switched = n.ModelSwitched + n.EffortChanged
		sum.ModelSwitched = n.ModelSwitched
		sum.EffortChanged = n.EffortChanged
		sum.Held = n.Held
		sum.HeldGuardrail = n.HeldGuardrail
		sum.UpshiftNotApplied = n.UpshiftNotApplied
		sum.TokensSaved = n.TokensSavedEstimate
		sum.Tokens = tokensDetail{Routing: n.Routing, Compactor: compactorOf(n.Compactor, shortHash(id))}
		list = append(list, *sum)
	}

	// Most recent first; equal times fall back to the id so the order is stable.
	sort.Slice(list, func(i, j int) bool {
		if list[i].LastSeen != list[j].LastSeen {
			return list[i].LastSeen > list[j].LastSeen
		}
		return list[i].SessionID < list[j].SessionID
	})

	var harnesses []string
	for h := range harnessSet {
		harnesses = append(harnesses, h)
	}
	return sessionsResponse{Sessions: list, Harnesses: harnesses, Report: report}
}
