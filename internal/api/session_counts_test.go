// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// decision is one routing decision row of a session, as the hook writes it.
func decision(session, ts string, ev telemetry.Event) telemetry.Event {
	ev.Harness = "claude-code"
	ev.SessionID = telemetry.HashSessionID(session)
	ev.Timestamp = ts
	return ev
}

func TestSessionCountsMatchPanel(t *testing.T) {
	const model = "claude-sonnet-4-5"
	cases := []struct {
		name string
		ev   telemetry.Event
		want struct{ switched, modelSwitched, effortChanged, held, heldGuardrail, upshiftNotApplied int }
	}{
		{
			name: "OK with allow is not held",
			ev:   telemetry.Event{Verdict: "OK", Outcome: telemetry.OutcomeAllow, FromModel: model, ToModel: model},
		},
		{
			name: "UPSHIFT with allow is not held and not a switch",
			ev:   telemetry.Event{Verdict: "UPSHIFT", Outcome: telemetry.OutcomeAllow, FromModel: model, ToModel: model},
			want: struct{ switched, modelSwitched, effortChanged, held, heldGuardrail, upshiftNotApplied int }{upshiftNotApplied: 1},
		},
		{
			name: "DOWNSHIFT to another model is a model switch",
			ev:   telemetry.Event{Verdict: "DOWNSHIFT", Outcome: telemetry.OutcomeRewriteEmitted, FromModel: "claude-opus-4-5", ToModel: model},
			want: struct{ switched, modelSwitched, effortChanged, held, heldGuardrail, upshiftNotApplied int }{switched: 1, modelSwitched: 1},
		},
		{
			name: "DOWNSHIFT with the same model is an effort change, not a model switch",
			ev: telemetry.Event{
				Verdict: "DOWNSHIFT", Outcome: telemetry.OutcomeRewriteEmitted,
				FromModel: model, ToModel: model, RequestedEffort: "high", FinalEffort: "low",
			},
			want: struct{ switched, modelSwitched, effortChanged, held, heldGuardrail, upshiftNotApplied int }{switched: 1, effortChanged: 1},
		},
		{
			name: "DOWNSHIFT held by a guardrail counts as held",
			ev: telemetry.Event{
				Verdict: "DOWNSHIFT", Outcome: telemetry.OutcomeAllow, Corrections: []string{"R1_SAFETY"},
				FromModel: "claude-opus-4-5", ToModel: model,
			},
			want: struct{ switched, modelSwitched, effortChanged, held, heldGuardrail, upshiftNotApplied int }{held: 1, heldGuardrail: 1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events := []telemetry.Event{decision("alpha", "2026-10-22T10:00:00Z", tc.ev)}
			list := buildSessionsResponse(events, nil, accounting.Numbers{}).Sessions
			if len(list) != 1 {
				t.Fatalf("sessions: got %d, want 1", len(list))
			}
			s := list[0]
			got := struct{ switched, modelSwitched, effortChanged, held, heldGuardrail, upshiftNotApplied int }{
				s.Switched, s.ModelSwitched, s.EffortChanged, s.Held, s.HeldGuardrail, s.UpshiftNotApplied,
			}
			if got != tc.want {
				t.Errorf("counts: got %+v, want %+v", got, tc.want)
			}
			panel := telemetry.BuildPanel(events, telemetry.HashSessionID("alpha"))
			if s.Switched != panel.Switched || s.Held != panel.Held {
				t.Errorf("list and panel disagree: list switched=%d held=%d, panel switched=%d held=%d",
					s.Switched, s.Held, panel.Switched, panel.Held)
			}
		})
	}
}

func TestSessionsSortedMostRecentFirst(t *testing.T) {
	events := []telemetry.Event{
		decision("old", "2026-10-20T08:00:00Z", telemetry.Event{Verdict: "OK", Outcome: telemetry.OutcomeAllow}),
		decision("new", "2026-10-22T08:00:00Z", telemetry.Event{Verdict: "OK", Outcome: telemetry.OutcomeAllow}),
		decision("mid", "2026-10-21T08:00:00Z", telemetry.Event{Verdict: "OK", Outcome: telemetry.OutcomeAllow}),
	}
	list := buildSessionsResponse(events, nil, accounting.Numbers{}).Sessions
	want := []string{telemetry.HashSessionID("new"), telemetry.HashSessionID("mid"), telemetry.HashSessionID("old")}
	for i, id := range want {
		if list[i].SessionID != id {
			t.Errorf("position %d: got %s, want %s", i, list[i].SessionID, id)
		}
	}
}

func TestCompactorScopeEqualInBothEndpoints(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "exec-events.jsonl")
	alpha := telemetry.HashSessionID("alpha")
	beta := telemetry.HashSessionID("beta")
	runs := []execwrap.Event{
		{Session: alpha, RawBytes: 4000, EmittedBytes: 1000, Compacted: true, ExitClass: execwrap.ExitClassZero},
		{Session: alpha, RawBytes: 800, EmittedBytes: 200, Compacted: true, ExitClass: execwrap.ExitClassZero},
		{Session: beta, RawBytes: 9000, EmittedBytes: 100, Compacted: true, ExitClass: execwrap.ExitClassZero},
		{RawBytes: 5000, EmittedBytes: 500, Compacted: true, ExitClass: execwrap.ExitClassZero},
	}
	for _, r := range runs {
		if err := execwrap.Append(logPath, r); err != nil {
			t.Fatalf("append exec run: %v", err)
		}
	}
	execEvents, err := execwrap.ReadEvents(logPath)
	if err != nil {
		t.Fatalf("read exec log: %v", err)
	}
	events := []telemetry.Event{
		decision("alpha", "2026-10-22T10:00:00Z", telemetry.Event{Verdict: "OK", Outcome: telemetry.OutcomeAllow}),
		decision("beta", "2026-10-22T11:00:00Z", telemetry.Event{Verdict: "OK", Outcome: telemetry.OutcomeAllow}),
	}

	var fromList *compactorWire
	var listSaved int64
	for _, s := range buildSessionsResponse(events, execEvents, accounting.Numbers{}).Sessions {
		if s.SessionID == alpha {
			fromList = s.Tokens.Compactor
			listSaved = s.TokensSaved
		}
	}
	fromPanel := buildSessionPanel(events, execEvents, accounting.Numbers{}, alpha)

	if fromList == nil || fromPanel.Compactor == nil {
		t.Fatalf("compactor block missing: list=%v panel=%v", fromList, fromPanel.Compactor)
	}
	if *fromList != *fromPanel.Compactor {
		t.Errorf("compactor differs between endpoints: list=%+v panel=%+v", *fromList, *fromPanel.Compactor)
	}
	if fromPanel.Compactor.Commands != 2 {
		t.Errorf("scope is not per session: got %d commands, want 2 (alpha only, not the 4-run total)", fromPanel.Compactor.Commands)
	}
	if listSaved != fromPanel.TokensSavedEstimate {
		t.Errorf("tokens_saved differs: list=%d panel=%d", listSaved, fromPanel.TokensSavedEstimate)
	}
}
