// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// numbersFixture is one log with two sessions, a Compactor run outside any
// session, and dates inside the report window.
func numbersFixture(t *testing.T) ([]telemetry.Event, []execwrap.Event) {
	t.Helper()
	ts := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	alpha, beta := telemetry.HashSessionID("alpha"), telemetry.HashSessionID("beta")

	var events []telemetry.Event
	work, cache := int64(100), int64(1000)
	events = append(events,
		telemetry.Event{CorrelationID: "a1", Timestamp: ts, SessionID: alpha, Harness: "claude-code",
			Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", FromModel: "claude-opus-5-5", ToModel: "claude-haiku-4-5"},
		telemetry.Event{Timestamp: ts, SessionID: alpha, Harness: "claude-code", Outcome: telemetry.OutcomeUsage,
			LinkedDecision: "a1", InputTokens: &work, CachedTokens: &cache},
		telemetry.Event{CorrelationID: "a2", Timestamp: ts, SessionID: alpha, Harness: "claude-code",
			Outcome: telemetry.OutcomeAllow, Verdict: "DOWNSHIFT", Corrections: []string{"R1_SAFETY"},
			FromModel: "claude-opus-5-5", ToModel: "claude-haiku-4-5"},
		telemetry.Event{CorrelationID: "b1", Timestamp: ts, SessionID: beta, Harness: "codex",
			Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT",
			FromModel: "claude-opus-5-5", ToModel: "claude-opus-5-5", RequestedEffort: "high", FinalEffort: "low"},
	)

	runs := []execwrap.Event{
		{TS: ts, Session: alpha, RawBytes: 4000, EmittedBytes: 1000, Compacted: true},
		{TS: ts, Session: beta, RawBytes: 9000, EmittedBytes: 100, Compacted: true},
		{TS: ts, RawBytes: 5000, EmittedBytes: 500, Compacted: true},
	}
	logPath := filepath.Join(t.TempDir(), "exec-events.jsonl")
	for _, r := range runs {
		if err := execwrap.Append(logPath, r); err != nil {
			t.Fatalf("append exec run: %v", err)
		}
	}
	execEvents, err := execwrap.ReadEvents(logPath)
	if err != nil {
		t.Fatalf("read exec log: %v", err)
	}
	return events, execEvents
}

// TestAPINumbersEqualCompute: every figure the API renders is the figure
// accounting.Compute gives for the same logs and scope.
func TestAPINumbersEqualCompute(t *testing.T) {
	events, execEvents := numbersFixture(t)
	report := reportNumbers(events, execEvents)
	list := buildSessionsResponse(events, execEvents, report)

	for _, row := range list.Sessions {
		want := accounting.Compute(events, execEvents, accounting.Scope{Kind: accounting.ScopeSession, SessionHash: row.SessionID})
		if row.Tokens.Routing != want.Routing {
			t.Errorf("%s routing: got %+v, want %+v", row.SessionID[:8], row.Tokens.Routing, want.Routing)
		}
		if row.Switched != want.ModelSwitched+want.EffortChanged || row.Held != want.Held ||
			row.HeldGuardrail != want.HeldGuardrail || row.UpshiftNotApplied != want.UpshiftNotApplied {
			t.Errorf("%s counts differ from Compute: row=%+v want=%+v", row.SessionID[:8], row, want)
		}
		if row.TokensSaved != want.TokensSavedEstimate {
			t.Errorf("%s tokens_saved: got %d, want %d", row.SessionID[:8], row.TokensSaved, want.TokensSavedEstimate)
		}
		if got, wantC := row.Tokens.Compactor, compactorOf(want.Compactor, shortHash(row.SessionID)); (got == nil) != (wantC == nil) || (got != nil && *got != *wantC) {
			t.Errorf("%s compactor: got %+v, want %+v", row.SessionID[:8], got, wantC)
		}

		panel := buildSessionPanel(events, execEvents, report, row.SessionID)
		if panel.Switched != row.Switched || panel.Held != row.Held || panel.HeldGuardrail != row.HeldGuardrail {
			t.Errorf("%s list and panel counts disagree", row.SessionID[:8])
		}
		if panel.TokensSavedEstimate != row.TokensSaved || panel.Tokens != row.Tokens.Routing {
			t.Errorf("%s list and panel savings disagree", row.SessionID[:8])
		}
		if (panel.Compactor == nil) != (row.Tokens.Compactor == nil) {
			t.Errorf("%s list and panel disagree on the compactor block", row.SessionID[:8])
		}
		if len(panel.Spawns) != row.SpawnCount {
			t.Errorf("%s spawns: panel %d, list %d", row.SessionID[:8], len(panel.Spawns), row.SpawnCount)
		}
	}

	// The report block is `downshift report --json`'s numbers: the report window
	// over the same logs.
	want := accounting.Compute(events, execEvents, accounting.Scope{Kind: accounting.ScopeDays, Days: reportDays})
	if !jsonEqual(t, list.Report, want) {
		t.Errorf("report block: got %+v, want %+v", list.Report, want)
	}
	if report.Compactor.Commands != 3 || report.Compactor.SavedBytes != 3000+8900+4500 {
		t.Errorf("report window compactor = %+v, want all three runs", report.Compactor)
	}
}

// TestSessionPanelCarriesReport: the single-session endpoint and the list carry
// the same report block.
func TestSessionPanelCarriesReport(t *testing.T) {
	events, execEvents := numbersFixture(t)
	report := reportNumbers(events, execEvents)
	alpha := telemetry.HashSessionID("alpha")
	panel := buildSessionPanel(events, execEvents, report, alpha)
	if !jsonEqual(t, panel.Report, report) {
		t.Errorf("panel report differs from the list's report")
	}

	raw, err := json.Marshal(panel)
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"switched", "held", "compactor", "tokens_saved_estimate", "report", "spawns"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("panel JSON lacks key %q", k)
		}
	}
}

func jsonEqual(t *testing.T, a, b any) bool {
	t.Helper()
	ja, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	jb, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(ja) == string(jb)
}
