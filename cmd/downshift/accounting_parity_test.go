// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Parity: every surface shows what accounting.Compute gives for the same logs.
// The golden logs are testdata/parity (synthetic: no prompt, path or raw id).
// A surface that adds, rounds or windows on its own fails here, field by field.

func reportScope() accounting.Scope {
	return accounting.Scope{Kind: accounting.ScopeDays, Days: defaultReportDays}
}

func sessionScope(name string) accounting.Scope {
	return accounting.Scope{Kind: accounting.ScopeSession, SessionHash: parityHash(name)}
}

// TestParityFixtureIsDiverse guards the fixture: a parity test over a log with
// no held spawn or no old exec run proves nothing about that case.
func TestParityFixtureIsDiverse(t *testing.T) {
	events, execEvents := parityState(t)
	all := accounting.Compute(events, execEvents, accounting.Scope{Kind: accounting.ScopeAll})
	win := accounting.Compute(events, execEvents, reportScope())

	if all.ModelSwitched == 0 || all.EffortChanged == 0 || all.HeldGuardrail == 0 || all.UpshiftNotApplied == 0 {
		t.Errorf("fixture lacks a spawn kind: %+v", all)
	}
	if routingSpawnCount(all.Routing) == 0 || routingBelow(all.Routing) == 0 {
		t.Errorf("fixture lacks a routing case (spawns, tokens below a baseline): %+v", all.Routing)
	}
	if all.Compactor.Commands <= win.Compactor.Commands || routingSpawnCount(all.Routing) <= routingSpawnCount(win.Routing) {
		t.Errorf("fixture has no event older than %d days: all=%+v window=%+v", defaultReportDays, all, win)
	}
	if win.Compactor.Compacted == win.Compactor.Commands {
		t.Errorf("fixture has no passthrough exec run: %+v", win.Compactor)
	}
}

// routingSpawnCount counts the spawns of a view, a count and not a token figure.
func routingSpawnCount(v telemetry.RoutingView) int {
	n := 0
	for _, r := range v.ByHarness {
		n += r.Spawns + r.FallbackSpawns + r.UnobservedSpawns
	}
	return n
}

// routingBelow reports the tokens that ran below any baseline, only to check
// that the fixture has such a case.
func routingBelow(v telemetry.RoutingView) int64 {
	var n int64
	for _, r := range v.ByHarness {
		n += r.BelowBillable + r.FallbackBelowBillable
	}
	return n
}

func TestParity_ReportJSON(t *testing.T) {
	events, execEvents := parityState(t)
	want := accounting.Compute(events, execEvents, reportScope())

	var out, errOut bytes.Buffer
	if rc := runReport([]string{"--json"}, &out, &errOut); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errOut.String())
	}
	var rep struct {
		Numbers                 json.RawMessage       `json:"numbers"`
		Routing                 telemetry.RoutingView `json:"routing_tokens"`
		CompactorSavedBytes     int64                 `json:"compactor_saved_bytes"`
		CompactorTokensEstimate int64                 `json:"compactor_tokens_estimate"`
		TokensSavedEstimate     int64                 `json:"tokens_saved_estimate"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	requireSame(t, "report --json .numbers", []byte(rep.Numbers), want)
	requireSame(t, "report --json routing_tokens", rep.Routing.ByHarness, want.Routing.ByHarness)
	requireSame(t, "report --json compactor keys",
		map[string]int64{"saved_bytes": rep.CompactorSavedBytes, "tokens": rep.CompactorTokensEstimate, "estimate": rep.TokensSavedEstimate},
		map[string]int64{"saved_bytes": want.Compactor.SavedBytes, "tokens": want.Compactor.Tokens, "estimate": want.TokensSavedEstimate})
}

// The text report prints Display.Compactor and, once, the Compactor tokens per
// harness; there is no cross-harness total line.
func TestParity_ReportText(t *testing.T) {
	events, execEvents := parityState(t)
	want := accounting.Compute(events, execEvents, reportScope())
	var out, errOut bytes.Buffer
	if rc := runReport([]string{"--text"}, &out, &errOut); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errOut.String())
	}
	for _, line := range []string{want.Display.Compactor} {
		if !bytes.Contains(out.Bytes(), []byte(line)) {
			t.Errorf("report text lacks the Compute line %q:\n%s", line, out.String())
		}
	}
}

func TestParity_SessionPanel(t *testing.T) {
	events, execEvents := parityState(t)
	wantReport := accounting.Compute(events, execEvents, reportScope())

	for _, name := range []string{"alpha", "beta"} { // "old" is outside the 30-day tail of interest but still valid below
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if rc := runSession([]string{"--panel", "--session=" + parityHash(name)}, &out, &errOut, nil); rc != 0 {
				t.Fatalf("rc=%d %s", rc, errOut.String())
			}
			var panel struct {
				Report              json.RawMessage `json:"report"`
				Efficiency          json.RawMessage `json:"efficiency"`
				Tokens              json.RawMessage `json:"tokens"`
				Switched            int             `json:"switched"`
				ModelSwitched       int             `json:"model_switched"`
				EffortChanged       int             `json:"effort_changed"`
				Held                int             `json:"held"`
				HeldGuardrail       int             `json:"held_guardrail"`
				UpshiftNotApplied   int             `json:"upshift_not_applied"`
				TokensSavedEstimate int64           `json:"tokens_saved_estimate"`
				Compactor           *struct {
					Commands   int64 `json:"commands"`
					Compacted  int64 `json:"compacted"`
					RawBytes   int64 `json:"raw_bytes"`
					SavedBytes int64 `json:"saved_bytes"`
					Tokens     int64 `json:"estimated_tokens_saved"`
				} `json:"compactor"`
			}
			if err := json.Unmarshal(out.Bytes(), &panel); err != nil {
				t.Fatal(err)
			}
			requireSame(t, "session --panel .report", []byte(panel.Report), wantReport)
			requireSame(t, "session --panel .efficiency", []byte(panel.Efficiency), accounting.BuildEfficiency(events, reportScope()))
			if panel.TokensSavedEstimate != wantReport.TokensSavedEstimate {
				t.Errorf("session --panel tokens_saved_estimate = %d, report window Compute = %d", panel.TokensSavedEstimate, wantReport.TokensSavedEstimate)
			}
			// The pane sits beside one session: its Compactor is the session scope's.
			ses := accounting.Compute(events, execEvents, sessionScope(name))
			if panel.Compactor == nil {
				t.Fatalf("session --panel has no compactor block")
			}
			requireSame(t, "session --panel .compactor", *panel.Compactor, map[string]int64{
				"commands": ses.Compactor.Commands, "compacted": ses.Compactor.Compacted,
				"raw_bytes": ses.Compactor.RawBytes, "saved_bytes": ses.Compactor.SavedBytes,
				"estimated_tokens_saved": ses.Compactor.Tokens})

			// The per-session figures of the panel against the session scope.
			requireSame(t, "session --panel per-session figures", map[string]any{
				"tokens": json.RawMessage(panel.Tokens), "switched": panel.Switched, "model_switched": panel.ModelSwitched,
				"effort_changed": panel.EffortChanged, "held": panel.Held, "held_guardrail": panel.HeldGuardrail,
				"upshift_not_applied": panel.UpshiftNotApplied,
			}, map[string]any{
				"tokens": ses.Routing.Only(), "switched": ses.ModelSwitched + ses.EffortChanged, "model_switched": ses.ModelSwitched,
				"effort_changed": ses.EffortChanged, "held": ses.Held, "held_guardrail": ses.HeldGuardrail,
				"upshift_not_applied": ses.UpshiftNotApplied,
			})
		})
	}
}
