// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// The panel names the scope of each figure: the bar is the report window over
// all sessions, the compactor block is this session, and runs with no session
// tag get their own line instead of vanishing.
func TestPanel_CompactorNamesItsScopeAndShowsDirectCalls(t *testing.T) {
	mine := telemetry.HashSessionID("s-mine")
	now := time.Now().UTC()
	run := func(session string, at time.Time, raw, emitted int64) execwrap.Event {
		return execwrap.Event{TS: at.Format(time.RFC3339), Family: "go", RawBytes: raw, EmittedBytes: emitted, Compacted: true, Session: session}
	}
	cases := []struct {
		name       string
		events     []execwrap.Event
		wantBlock  bool
		wantDirect string // "" means no direct line
	}{
		{"session and direct", []execwrap.Event{run(mine, now, 1000, 200), run("", now, 4000, 400), run("", now, 2000, 400)},
			true, "direct calls, no session: 2, ~1.3k tokens saved"},
		{"only direct calls still shows the block", []execwrap.Event{run("", now, 800, 0)},
			true, "direct calls, no session: 1, ~200 tokens saved"},
		{"another session is neither mine nor direct", []execwrap.Event{run("other", now, 1000, 0)},
			false, ""},
		{"direct call older than the window is left out", []execwrap.Event{run("", now.AddDate(0, 0, -90), 800, 0)},
			false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := panelWithReport(telemetry.PanelView{Session: mine}, mine, nil, tc.events)
			if got.TokensSavedScope != "last 30 days, all sessions" {
				t.Fatalf("tokens_saved_scope = %q", got.TokensSavedScope)
			}
			if (got.Compactor != nil) != tc.wantBlock {
				t.Fatalf("compactor block = %+v, want present=%v", got.Compactor, tc.wantBlock)
			}
			if !tc.wantBlock {
				return
			}
			if got.Compactor.Scope != "this session" {
				t.Fatalf("scope = %q", got.Compactor.Scope)
			}
			d := got.Compactor.Direct
			if (d != nil) != (tc.wantDirect != "") || (d != nil && d.Line != tc.wantDirect) {
				t.Fatalf("direct = %+v, want line %q", d, tc.wantDirect)
			}
		})
	}
}
