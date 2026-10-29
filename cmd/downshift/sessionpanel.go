// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// panelOptions are the `session --panel` flags that trim the answer.
type panelOptions struct {
	// NoReport skips the 30-day report: no report, compactor or
	// tokens_saved_estimate, and the full logs are not read (the tail only).
	NoReport bool
	// Brief prints only what the agents band draws (panelBrief).
	Brief bool
}

// writePanel prints the panel JSON for hash. events is the session tail; the
// full logs are read only when the report is wanted.
func writePanel(events []telemetry.Event, hash string, opt panelOptions, stdout, stderr io.Writer) int {
	view := telemetry.BuildPanel(events, hash)
	why := accounting.BuildSpawnWhy(events, hash)
	var out any = panelNoReport{PanelView: view, Why: why}
	if !opt.NoReport {
		// The report's window reads the whole logs; the per-session view is from the tail.
		allEvents, _ := telemetry.ReadEvents()
		execEvents, _ := accounting.ReadExecEvents()
		full := panelWithReport(view, allEvents, execEvents)
		full.Why = why
		out = full
		if opt.Brief {
			out = briefWithTokens(view, full.TokensSavedEstimate)
		}
	} else if opt.Brief {
		out = briefOf(view)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(stderr, "downshift session: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(raw))
	return 0
}

// panelNoReport is the panel without the report: the session view plus, per
// spawn, why it changed or not (the routing note as one short cell).
type panelNoReport struct {
	telemetry.PanelView
	Why []accounting.SpawnWhy `json:"routing_why"`
}

// compactorTokenEstimate names how Compactor tokens are estimated; it is the
// same text execwrap.Summarize gives.
const compactorTokenEstimate = "estimated: saved_bytes / 4"

// compactorPanel is the Compactor block of the panel. It is the report's
// Compactor over the report's window (Scope is that window's label), so the
// panel and `downshift report` show the same figures.
type compactorPanel struct {
	Scope                string `json:"scope"`
	Commands             int64  `json:"commands"`
	Compacted            int64  `json:"compacted"`
	RawBytes             int64  `json:"raw_bytes"`
	SavedBytes           int64  `json:"saved_bytes"`
	EstimatedTokensSaved int64  `json:"estimated_tokens_saved"`
	TokenEstimate        string `json:"token_estimate"`
}

// panelOut is the `session --panel` JSON. The session fields (PanelView) are
// per session and unchanged. Report is the accounting.Numbers of `downshift
// report` (same window, full event and exec logs). TokensSavedEstimate and the
// Compactor block are the report's figures, so the Agents panel and the
// progress bar show what `report` shows.
type panelOut struct {
	telemetry.PanelView
	Report              accounting.Numbers `json:"report"`
	Compactor           *compactorPanel    `json:"compactor,omitempty"`
	TokensSavedEstimate int64              `json:"tokens_saved_estimate"`
	// Why is one entry per spawn: requested model, final model, verdict and
	// reason in one cell. Enums and declared ids only.
	Why []accounting.SpawnWhy `json:"routing_why"`
}

// panelWithReport builds the panel from the session view and the full logs.
// The window is the one `downshift report` uses by default. No exec run in
// that window means no Compactor block, never zeros.
func panelWithReport(v telemetry.PanelView, events []telemetry.Event, execEvents []execwrap.Event) panelOut {
	nums := accounting.Compute(events, execEvents, accounting.Scope{Kind: accounting.ScopeDays, Days: defaultReportDays})
	out := panelOut{PanelView: v, Report: nums, TokensSavedEstimate: nums.TokensSavedEstimate}
	c := nums.Compactor
	if c.Commands == 0 {
		return out
	}
	out.Compactor = &compactorPanel{
		Scope: nums.Scope.Label(), Commands: c.Commands, Compacted: c.Compacted, RawBytes: c.RawBytes,
		SavedBytes: c.SavedBytes, EstimatedTokensSaved: c.Tokens, TokenEstimate: compactorTokenEstimate,
	}
	return out
}
