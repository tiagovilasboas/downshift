// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// belowSession is the session every belowEvents event belongs to.
var belowSession = telemetry.HashSessionID("s-below")

// belowEvents is one decision and its usage for a harness, in belowSession;
// parent "" leaves the session model unknown.
func belowEvents(harness, id, parent, child string) []telemetry.Event {
	in, out, write := int64(1000), int64(500), int64(200)
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	return []telemetry.Event{
		{Timestamp: ts, CorrelationID: id, SessionID: belowSession, Harness: harness, Outcome: telemetry.OutcomeAllow, FromModel: child, ToModel: child, ParentModel: parent},
		{Timestamp: ts, CorrelationID: id + "u", SessionID: belowSession, Harness: harness, Outcome: telemetry.OutcomeUsage, LinkedDecision: id, FromModel: child, ToModel: child,
			InputTokens: &in, OutputTokens: &out, CacheWriteTokens: &write},
	}
}

func TestPanelBelowSession_PerHarnessNeverSummed(t *testing.T) {
	cases := []struct {
		name   string
		events []telemetry.Event
		want   map[string]belowSessionRow
	}{
		{"no events gives no rows", nil, map[string]belowSessionRow{}},
		{"below the session model is observed", belowEvents("claude-code", "d1", "claude-opus-4-8", "claude-sonnet-4-6"),
			map[string]belowSessionRow{"claude-code": {Harness: "claude-code", Observed: true,
				RoutingRow: telemetry.RoutingRow{Spawns: 1, BelowSpawns: 1, Billable: 1500, BelowBillable: 1500}}}},
		{"no rankable model reads unobserved", belowEvents("codex", "d2", "", "claude-sonnet-4-6"),
			map[string]belowSessionRow{"codex": {Harness: "codex", Observed: false,
				RoutingRow: telemetry.RoutingRow{UnobservedSpawns: 1}}}},
		{"two harnesses stay two rows", append(
			belowEvents("claude-code", "d1", "claude-opus-4-8", "claude-sonnet-4-6"),
			belowEvents("codex", "d2", "", "claude-sonnet-4-6")...),
			map[string]belowSessionRow{
				"claude-code": {Harness: "claude-code", Observed: true,
					RoutingRow: telemetry.RoutingRow{Spawns: 1, BelowSpawns: 1, Billable: 1500, BelowBillable: 1500}},
				"codex": {Harness: "codex", RoutingRow: telemetry.RoutingRow{UnobservedSpawns: 1}},
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := panelWithReport(telemetry.PanelView{}, belowSession, tc.events, nil).BelowSession
			if len(rows) != len(tc.want) {
				t.Fatalf("rows = %+v, want %d", rows, len(tc.want))
			}
			for _, r := range rows {
				if r != tc.want[r.Harness] {
					t.Fatalf("%s: got %+v, want %+v", r.Harness, r, tc.want[r.Harness])
				}
			}
		})
	}
}

// The JSON carries counts and the harness name only: no prompt, path or id.
func TestPanelBelowSession_JSONShape(t *testing.T) {
	out := panelWithReport(telemetry.PanelView{}, belowSession, belowEvents("claude-code", "secret-id", "claude-opus-4-8", "claude-sonnet-4-6"), nil)
	raw, err := json.Marshal(out.BelowSession)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["observed"] != true || rows[0]["below_billable"] != float64(1500) {
		t.Fatalf("shape: %s", raw)
	}
	if strings.Contains(string(raw), "secret-id") {
		t.Fatalf("an id leaked: %s", raw)
	}
}

// Another session's spawns are not this pane's: the rows keep the session only.
func TestPanelBelowSession_OtherSessionLeftOut(t *testing.T) {
	events := belowEvents("claude-code", "d1", "claude-opus-4-8", "claude-sonnet-4-6")
	other := belowEvents("kirocrew", "d2", "claude-opus-4-8", "claude-sonnet-4-6")
	for i := range other {
		other[i].SessionID = telemetry.HashSessionID("s-other")
	}
	rows := panelWithReport(telemetry.PanelView{}, belowSession, append(events, other...), nil).BelowSession
	if len(rows) != 1 || rows[0].Harness != "claude-code" {
		t.Fatalf("rows = %+v, want this session's claude-code row only", rows)
	}
}
