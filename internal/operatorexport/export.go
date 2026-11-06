// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package operatorexport builds the one artefact an outside operator shares
// after a measurement week (issue #104, roadmap P3.2). It carries only counts
// and token totals: no prompts, no file paths, no session ids, no model free
// text, no hostnames and no usernames. The Check function in allowlist.go is
// the sensor that proves it: it fails on any string the export does not allow.
package operatorexport

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// SchemaVersion is the operator export's own version, apart from every other
// export's schema. Bump it only when a field is added or removed.
const SchemaVersion = 3

// Note is the fixed honesty line on every export. Single-user dogfood data is
// directional, never a billing claim; an operator pastes it as-is.
const Note = "Counts and token totals only: no prompts, paths, ids, model names, hostnames or usernames. Single-user dogfood, directional, not billing."

// Export is the whole artefact. Every string field is either a fixed schema
// label (Note), a timestamp (GeneratedAt, which carries no identity), or a
// harness name bound to the allowlist (Harnesses[].Harness). Everything else
// is a count or a token total.
type Export struct {
	Schema      int          `json:"schema"`
	WindowDays  int          `json:"window_days"`
	GeneratedAt string       `json:"generated_at"`
	Routed      Group        `json:"routed"`
	Tokens      Tokens       `json:"tokens"`
	Harnesses   []HarnessRow `json:"harnesses"`
	Note        string       `json:"note"`
}

// Group is the routed spawns of the window, counts only.
type Group struct {
	Spawns    int `json:"spawns"`
	Ended     int `json:"ended"`
	Completed int `json:"completed"`
	Reworked  int `json:"reworked"`
}

// Tokens are the token totals the report already computes. No per-session or
// per-model breakdown: an operator shares totals, not a map keyed by anything
// private.
type Tokens struct {
	// RoutedBillable is the billable tokens (input, output and cache write)
	// of the routed completed tasks.
	RoutedBillable int64 `json:"routed_billable"`
	// CompactorAvoided is the tokens the compactor kept out of the context,
	// summed across harnesses (an estimate: saved bytes / 4).
	CompactorAvoided int64 `json:"compactor_avoided"`
}

// HarnessRow is one harness's token totals. Harness is the only free string in
// the whole export, and it must be a known adapter name (see allowlist.go).
// The routing figures are per harness and never summed across harnesses:
// session_* are the spawns placed against the session model, fallback_* the
// spawns placed against the requested model only (a different baseline, never
// added), cache read apart from billable.
type HarnessRow struct {
	Harness               string `json:"harness"`
	SessionBillable       int64  `json:"session_billable"`
	BelowSessionBillable  int64  `json:"below_session_billable"`
	FallbackBillable      int64  `json:"fallback_billable"`
	FallbackBelowBillable int64  `json:"fallback_below_billable"`
	UnobservedSpawns      int    `json:"unobserved_spawns"`
	CacheRead             int64  `json:"cache_read"`
	CompactorAvoided      int64  `json:"compactor_avoided"`
}

// Build assembles the export from the report aggregation downshift already
// computes, so the operator export and `downshift report` read the same
// windowed events. It performs no I/O and no clock read of its own: the caller
// passes events, exec events, the window and now, which keeps it a pure
// function the test can pin.
func Build(events []telemetry.Event, execEvents []execwrap.Event, days int, now time.Time) Export {
	rep := telemetry.BuildReport(events, days)

	scope := accounting.Scope{Kind: accounting.ScopeDays, Days: days}
	nums := accounting.Compute(events, execEvents, scope)
	perHarness := accounting.TokensByHarness(events, execEvents, scope, nil)

	// A harness name outside the adapter table (a renamed or removed adapter in
	// an old log) is folded into the unknown sentinel, so it can neither leak
	// nor make the allowlist refuse the whole export.
	byName := map[string]*HarnessRow{}
	routing := map[string]*telemetry.RoutingRow{}
	for _, h := range perHarness {
		name := h.Harness
		if !knownHarnessValue(name) {
			name = accounting.UnknownHarness
		}
		row, ok := byName[name]
		if !ok {
			row = &HarnessRow{Harness: name}
			byName[name] = row
		}
		row.CompactorAvoided += h.CompactorAvoided
		if routing[name] == nil {
			routing[name] = &telemetry.RoutingRow{}
		}
		routing[name].Add(h.Routing)
	}
	rows := make([]HarnessRow, 0, len(byName))
	for name, row := range byName {
		r := routing[name]
		row.SessionBillable, row.BelowSessionBillable = r.Billable, r.BelowBillable
		row.FallbackBillable, row.FallbackBelowBillable = r.FallbackBillable, r.FallbackBelowBillable
		row.UnobservedSpawns, row.CacheRead = r.UnobservedSpawns, r.CacheRead
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Harness < rows[j].Harness })

	return Export{
		Schema:      SchemaVersion,
		WindowDays:  days,
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Routed:      groupOf(rep.Routed),
		Tokens: Tokens{
			RoutedBillable:   rep.Routed.Tokens.Billable(),
			CompactorAvoided: nums.Compactor.Tokens,
		},
		Harnesses: rows,
		Note:      Note,
	}
}

func groupOf(g telemetry.GroupSummary) Group {
	return Group{Spawns: g.Spawns, Ended: g.Ended, Completed: g.Completed, Reworked: g.Reworked}
}

// JSON renders the export as stable indented JSON an operator can paste.
func JSON(e Export) (string, error) {
	raw, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
