// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// reportDays is the window `downshift report` uses without --days
// (cmd/downshift/report.go). It is not exported there, so it is repeated here.
const reportDays = 30

// compactorEstimate is the note the Compactor carries about its token figure.
const compactorEstimate = "estimated: saved_bytes / 4"

// compactorWire is the Compactor block the dashboard reads
// (dashboard/src/shared/data/types.ts). The keys stay as they were; the values
// are accounting.Compactor's.
type compactorWire struct {
	Scope                string `json:"scope"`
	Commands             int64  `json:"commands"`
	Compacted            int64  `json:"compacted"`
	RawBytes             int64  `json:"raw_bytes"`
	SavedBytes           int64  `json:"saved_bytes"`
	EstimatedTokensSaved int64  `json:"estimated_tokens_saved"`
	TokenEstimate        string `json:"token_estimate"`
}

// compactorOf renders a Compactor. No run in scope means no block, never zeros.
func compactorOf(c accounting.Compactor, scope string) *compactorWire {
	if c.Commands == 0 {
		return nil
	}
	return &compactorWire{
		Scope: scope, Commands: c.Commands, Compacted: c.Compacted, RawBytes: c.RawBytes,
		SavedBytes: c.SavedBytes, EstimatedTokensSaved: c.Tokens, TokenEstimate: compactorEstimate,
	}
}

// shortHash is the session label the dashboard shows: the first 8 characters.
func shortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

// readDashboardEvents is the event window every endpoint reads, so the session
// list and the session panel count the same events.
func readDashboardEvents() []telemetry.Event {
	events, _ := telemetry.ReadEventsTail(telemetry.DefaultTailBytes)
	return events
}

// readExecEvents is the whole exec log; a missing log is empty.
func readExecEvents() []execwrap.Event {
	events, _ := accounting.ReadExecEvents()
	return events
}

// loadReport reads the full event log and returns the report block: the same
// Numbers as `downshift report --json` for the same logs.
func loadReport(execEvents []execwrap.Event) accounting.Numbers {
	events, _ := telemetry.ReadEvents()
	return reportNumbers(events, execEvents)
}

// reportNumbers is the report window (reportDays) over the given logs.
func reportNumbers(events []telemetry.Event, execEvents []execwrap.Event) accounting.Numbers {
	return accounting.Compute(events, execEvents, accounting.Scope{Kind: accounting.ScopeDays, Days: reportDays})
}

// sessionNumbers is the Numbers of one session over the given logs.
func sessionNumbers(events []telemetry.Event, execEvents []execwrap.Event, hash string) accounting.Numbers {
	return accounting.Compute(events, execEvents, accounting.Scope{Kind: accounting.ScopeSession, SessionHash: hash})
}
