// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
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
		full := panelWithReport(view, hash, allEvents, execEvents)
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

// compactorPanel is the Compactor block of the panel: accounting.Compute's
// Compactor for the session scope (the runs tagged with the session's hashed
// id), since the pane sits beside one session. `downshift report` keeps every
// session and harness.
type compactorPanel struct {
	Scope                string `json:"scope"`
	Commands             int64  `json:"commands"`
	Compacted            int64  `json:"compacted"`
	RawBytes             int64  `json:"raw_bytes"`
	SavedBytes           int64  `json:"saved_bytes"`
	EstimatedTokensSaved int64  `json:"estimated_tokens_saved"`
	TokenEstimate        string `json:"token_estimate"`
	// Families is the per-family breakdown of this session's runs; never null.
	Families []compactorFamilyPanel `json:"families"`
	// Direct is the runs with no session tag in the report window (a direct
	// `downshift compactor -- ...`). The session figures above never include
	// them, so they are shown apart, never dropped and never added in.
	Direct *compactorDirect `json:"direct,omitempty"`
}

// compactorDirect is the untagged Compactor runs of the report window.
type compactorDirect struct {
	Scope                string `json:"scope"`
	Commands             int64  `json:"commands"`
	SavedBytes           int64  `json:"saved_bytes"`
	EstimatedTokensSaved int64  `json:"estimated_tokens_saved"`
	// Line is the ready text: "direct calls, no session: 3, ~1.2k tokens saved".
	Line string `json:"line"`
}

// panelOut is the `session --panel` JSON. The session fields (PanelView), the
// Compactor block and BelowSession are this session only, since the pane sits
// beside one session; Compactor.Direct is the untagged runs of the report
// window. Report and TokensSavedEstimate stay the accounting.Numbers of
// `downshift report` (same window, all sessions, full event and exec logs),
// and TokensSavedScope names that scope.
type panelOut struct {
	telemetry.PanelView
	Report              accounting.Numbers `json:"report"`
	Compactor           *compactorPanel    `json:"compactor,omitempty"`
	TokensSavedEstimate int64              `json:"tokens_saved_estimate"`
	// TokensSavedScope says what TokensSavedEstimate (and Report) cover, so the
	// bar is not read as this session: "last 30 days, all sessions".
	TokensSavedScope string `json:"tokens_saved_scope"`
	// BelowSession is, per harness of this session, the billable tokens that
	// ran below the session model plus the requested-model fallback apart
	// (telemetry.RoutingTokens). Never summed.
	BelowSession []belowSessionRow `json:"below_session_by_harness"`
	// Efficiency is the `downshift report --json` efficiency block for the
	// report's default window: per harness, never summed, no tokens, no money.
	Efficiency accounting.Efficiency `json:"efficiency"`
	// Why is one entry per spawn: requested model, final model, verdict and
	// reason in one cell. Enums and declared ids only.
	Why []accounting.SpawnWhy `json:"routing_why"`
}

// panelWithReport builds the panel from the session view and the full logs.
// Report uses the window `downshift report` uses by default; the Compactor
// block and BelowSession use the session scope. With no exec run of the
// session and no direct call there is no Compactor block; with only direct
// calls the block carries them and zero session commands.
func panelWithReport(v telemetry.PanelView, hash string, events []telemetry.Event, execEvents []execwrap.Event) panelOut {
	scope := accounting.Scope{Kind: accounting.ScopeDays, Days: defaultReportDays}
	nums := accounting.Compute(events, execEvents, scope)
	session := accounting.Scope{Kind: accounting.ScopeSession, SessionHash: hash}
	out := panelOut{
		PanelView: v, Report: nums, TokensSavedEstimate: nums.TokensSavedEstimate,
		TokensSavedScope: scope.Label() + ", all sessions",
		BelowSession:     belowSessionRows(events, execEvents, session),
		Efficiency:       accounting.BuildEfficiency(events, scope),
	}
	c := accounting.Compute(events, execEvents, session).Compactor
	direct := accounting.DirectCompactor(execEvents, scope, time.Now())
	if c.Commands == 0 && direct.Commands == 0 {
		return out
	}
	out.Compactor = &compactorPanel{
		Scope: session.Label(), Commands: c.Commands, Compacted: c.Compacted, RawBytes: c.RawBytes,
		SavedBytes: c.SavedBytes, EstimatedTokensSaved: c.Tokens, TokenEstimate: compactorTokenEstimate,
		Families: sessionFamilies(execEvents, session),
	}
	if direct.Commands > 0 {
		out.Compactor.Direct = &compactorDirect{
			Scope: scope.Label(), Commands: direct.Commands, SavedBytes: direct.SavedBytes,
			EstimatedTokensSaved: direct.Tokens, Line: accounting.DirectLine(direct),
		}
	}
	return out
}
