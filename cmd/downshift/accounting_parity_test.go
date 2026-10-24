// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/api"
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
	if all.Routing.Children == 0 || all.Routing.SavedChildren == 0 || all.Routing.AtFloor == 0 || all.Routing.GatewayUnattributed == 0 {
		t.Errorf("fixture lacks a routing case (children, saved, at floor, gateway): %+v", all.Routing)
	}
	if all.Compactor.Commands <= win.Compactor.Commands || all.Routing.Children <= win.Routing.Children {
		t.Errorf("fixture has no event older than %d days: all=%+v window=%+v", defaultReportDays, all, win)
	}
	if win.Compactor.Compacted == win.Compactor.Commands {
		t.Errorf("fixture has no passthrough exec run: %+v", win.Compactor)
	}
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
		TokensSaved             telemetry.TokenSaving `json:"tokens_saved"`
		CompactorSavedBytes     int64                 `json:"compactor_saved_bytes"`
		CompactorTokensEstimate int64                 `json:"compactor_tokens_estimate"`
		TokensSavedEstimate     int64                 `json:"tokens_saved_estimate"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	requireSame(t, "report --json .numbers", []byte(rep.Numbers), want)
	requireSame(t, "report --json tokens_saved", rep.TokensSaved, want.Routing)
	requireSame(t, "report --json compactor keys",
		map[string]int64{"saved_bytes": rep.CompactorSavedBytes, "tokens": rep.CompactorTokensEstimate, "estimate": rep.TokensSavedEstimate},
		map[string]int64{"saved_bytes": want.Compactor.SavedBytes, "tokens": want.Compactor.Tokens, "estimate": want.TokensSavedEstimate})
}

// The text report ends with Display.TokensSaved and prints Display.Compactor.
func TestParity_ReportText(t *testing.T) {
	events, execEvents := parityState(t)
	want := accounting.Compute(events, execEvents, reportScope())
	var out, errOut bytes.Buffer
	if rc := runReport(nil, &out, &errOut); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errOut.String())
	}
	for _, line := range []string{want.Display.TokensSaved, want.Display.Compactor} {
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
			if panel.TokensSavedEstimate != wantReport.TokensSavedEstimate {
				t.Errorf("session --panel tokens_saved_estimate = %d, report window Compute = %d", panel.TokensSavedEstimate, wantReport.TokensSavedEstimate)
			}
			if panel.Compactor == nil {
				t.Fatalf("session --panel has no compactor block")
			}
			requireSame(t, "session --panel .compactor", *panel.Compactor, map[string]int64{
				"commands": wantReport.Compactor.Commands, "compacted": wantReport.Compactor.Compacted,
				"raw_bytes": wantReport.Compactor.RawBytes, "saved_bytes": wantReport.Compactor.SavedBytes,
				"estimated_tokens_saved": wantReport.Compactor.Tokens})

			// The per-session figures of the panel against the session scope.
			ses := accounting.Compute(events, execEvents, sessionScope(name))
			requireSame(t, "session --panel per-session figures", map[string]any{
				"tokens": json.RawMessage(panel.Tokens), "switched": panel.Switched, "model_switched": panel.ModelSwitched,
				"effort_changed": panel.EffortChanged, "held": panel.Held, "held_guardrail": panel.HeldGuardrail,
				"upshift_not_applied": panel.UpshiftNotApplied,
			}, map[string]any{
				"tokens": ses.Routing, "switched": ses.ModelSwitched + ses.EffortChanged, "model_switched": ses.ModelSwitched,
				"effort_changed": ses.EffortChanged, "held": ses.Held, "held_guardrail": ses.HeldGuardrail,
				"upshift_not_applied": ses.UpshiftNotApplied,
			})
		})
	}
}

// startAPI serves the dashboard API on a loopback port for the test.
func startAPI(t *testing.T) string {
	t.Helper()
	srv, err := api.NewServer(0)
	if err != nil {
		t.Fatal(err)
	}
	go srv.Start()
	t.Cleanup(func() { _ = srv.Shutdown(t.Context()) })
	return "http://" + srv.Addr()
}

func getJSON(t *testing.T, url string) []byte {
	t.Helper()
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d %s", url, resp.StatusCode, body)
	}
	return body
}

func TestParity_DashboardAPI(t *testing.T) {
	events, execEvents := parityState(t)
	wantReport := accounting.Compute(events, execEvents, reportScope())
	base := startAPI(t)

	var list struct {
		Report   json.RawMessage `json:"report"`
		Sessions []struct {
			SessionID         string `json:"session_id"`
			Switched          int    `json:"switched"`
			ModelSwitched     int    `json:"model_switched"`
			EffortChanged     int    `json:"effort_changed"`
			Held              int    `json:"held"`
			HeldGuardrail     int    `json:"held_guardrail"`
			UpshiftNotApplied int    `json:"upshift_not_applied"`
			TokensSaved       int64  `json:"tokens_saved"`
			Tokens            struct {
				Routing   telemetry.TokenSaving `json:"routing"`
				Compactor *struct {
					Commands   int64 `json:"commands"`
					Compacted  int64 `json:"compacted"`
					RawBytes   int64 `json:"raw_bytes"`
					SavedBytes int64 `json:"saved_bytes"`
					Tokens     int64 `json:"estimated_tokens_saved"`
				} `json:"compactor"`
			} `json:"tokens"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(getJSON(t, base+"/api/sessions"), &list); err != nil {
		t.Fatal(err)
	}
	requireSame(t, "/api/sessions .report", []byte(list.Report), wantReport)
	if len(list.Sessions) != len(paritySessions) {
		t.Fatalf("/api/sessions lists %d sessions, fixture has %d", len(list.Sessions), len(paritySessions))
	}

	figures := func(n accounting.Numbers) map[string]any {
		comp := map[string]int64{}
		if n.Compactor.Commands > 0 {
			comp = map[string]int64{"commands": n.Compactor.Commands, "compacted": n.Compactor.Compacted,
				"raw_bytes": n.Compactor.RawBytes, "saved_bytes": n.Compactor.SavedBytes, "tokens": n.Compactor.Tokens}
		}
		return map[string]any{"routing": n.Routing, "switched": n.ModelSwitched + n.EffortChanged,
			"model_switched": n.ModelSwitched, "effort_changed": n.EffortChanged, "held": n.Held,
			"held_guardrail": n.HeldGuardrail, "upshift_not_applied": n.UpshiftNotApplied,
			"tokens_saved": n.TokensSavedEstimate, "compactor": comp}
	}
	for _, row := range list.Sessions {
		name := ""
		for _, n := range paritySessions {
			if parityHash(n) == row.SessionID {
				name = n
			}
		}
		if name == "" {
			t.Fatalf("/api/sessions lists an unknown session %s", row.SessionID[:8])
		}
		want := accounting.Compute(events, execEvents, sessionScope(name))
		comp := map[string]int64{}
		if c := row.Tokens.Compactor; c != nil {
			comp = map[string]int64{"commands": c.Commands, "compacted": c.Compacted, "raw_bytes": c.RawBytes, "saved_bytes": c.SavedBytes, "tokens": c.Tokens}
		}
		requireSame(t, "/api/sessions row "+name, map[string]any{"routing": row.Tokens.Routing, "switched": row.Switched,
			"model_switched": row.ModelSwitched, "effort_changed": row.EffortChanged, "held": row.Held,
			"held_guardrail": row.HeldGuardrail, "upshift_not_applied": row.UpshiftNotApplied,
			"tokens_saved": row.TokensSaved, "compactor": comp}, figures(want))

		var panel struct {
			Report              json.RawMessage       `json:"report"`
			Tokens              telemetry.TokenSaving `json:"tokens"`
			Switched            int                   `json:"switched"`
			ModelSwitched       int                   `json:"model_switched"`
			EffortChanged       int                   `json:"effort_changed"`
			Held                int                   `json:"held"`
			HeldGuardrail       int                   `json:"held_guardrail"`
			UpshiftNotApplied   int                   `json:"upshift_not_applied"`
			TokensSavedEstimate int64                 `json:"tokens_saved_estimate"`
			Compactor           *struct {
				Commands   int64 `json:"commands"`
				Compacted  int64 `json:"compacted"`
				RawBytes   int64 `json:"raw_bytes"`
				SavedBytes int64 `json:"saved_bytes"`
				Tokens     int64 `json:"estimated_tokens_saved"`
			} `json:"compactor"`
		}
		if err := json.Unmarshal(getJSON(t, base+"/api/session/"+row.SessionID), &panel); err != nil {
			t.Fatal(err)
		}
		requireSame(t, "/api/session/"+name+" .report", []byte(panel.Report), wantReport)
		pc := map[string]int64{}
		if c := panel.Compactor; c != nil {
			pc = map[string]int64{"commands": c.Commands, "compacted": c.Compacted, "raw_bytes": c.RawBytes, "saved_bytes": c.SavedBytes, "tokens": c.Tokens}
		}
		requireSame(t, "/api/session/"+name+" per-session figures", map[string]any{"routing": panel.Tokens, "switched": panel.Switched,
			"model_switched": panel.ModelSwitched, "effort_changed": panel.EffortChanged, "held": panel.Held,
			"held_guardrail": panel.HeldGuardrail, "upshift_not_applied": panel.UpshiftNotApplied,
			"tokens_saved": panel.TokensSavedEstimate, "compactor": pc}, figures(want))
	}
}
