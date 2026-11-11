// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

var brt = time.FixedZone("BRT", -3*3600)

func execAt(ts, harness string, raw, emitted int64) execwrap.Event {
	return execwrap.Event{TS: ts, Harness: harness, RawBytes: raw, EmittedBytes: emitted, Compacted: true}
}

// routedAt is a decision plus its usage record on the same instant.
func routedAt(ts, harness, id string, work int64) []telemetry.Event {
	big, small := "claude-opus-5-5", "claude-haiku-4-5"
	if harness == "codex" {
		big, small = "gpt-5.6-sol", "gpt-5.6-terra"
	}
	return []telemetry.Event{
		{CorrelationID: id, Timestamp: ts, Harness: harness, Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT",
			FromModel: big, ToModel: small, ParentModel: big},
		{Timestamp: ts, Harness: harness, Outcome: telemetry.OutcomeUsage, LinkedDecision: id, InputTokens: i64(work)},
	}
}

func TestBuildPeriods_DayBucketingAndTimeZone(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	// 02:30Z on Nov 6 is still Nov 5 in BRT.
	execs := []execwrap.Event{
		execAt("2026-11-06T02:30:00Z", "codex", 800, 0),
		execAt("2026-11-06T00:00:00Z", "codex", 400, 0),
		execAt("2026-11-06T03:00:00Z", "codex", 400, 0), // midnight BRT: new day
	}
	tests := []struct {
		name string
		loc  *time.Location
		want map[string]int64
	}{
		{"utc", time.UTC, map[string]int64{"2026-11-06": 400}},
		{"brt", brt, map[string]int64{"2026-11-05": 300, "2026-11-06": 100}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := BuildPeriods(nil, execs, PeriodOptions{Per: PerDay, Now: now, Loc: tc.loc})
			got := map[string]int64{}
			for _, r := range rows {
				for _, h := range r.Harnesses {
					got[r.Period] += h.CompactionTokens
				}
			}
			if len(got) != len(tc.want) {
				t.Fatalf("periods = %v, want %v", got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("%s = %d, want %d (all %v)", k, got[k], v, got)
				}
			}
		})
	}
}

func TestBuildPeriods_WeekStartsMonday(t *testing.T) {
	tests := []struct{ ts, week string }{
		{"2026-11-02T10:00:00Z", "2026-11-02"}, // Monday
		{"2026-11-08T23:00:00Z", "2026-11-02"}, // Sunday
		{"2026-11-09T00:30:00Z", "2026-11-09"}, // next Monday
		{"2026-11-01T10:00:00Z", "2026-10-26"}, // across the month
	}
	for _, tc := range tests {
		rows := BuildPeriods(nil, []execwrap.Event{execAt(tc.ts, "codex", 40, 0)},
			PeriodOptions{Per: PerWeek, Now: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)})
		if len(rows) != 1 || rows[0].Period != tc.week {
			t.Errorf("%s -> %+v, want week %s", tc.ts, rows, tc.week)
		}
	}
}

func TestBuildPeriods_PerHarnessSplitAndRouting(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	var events []telemetry.Event
	events = append(events, routedAt("2026-11-05T10:00:00Z", "claude-code", "a", 1000)...)
	events = append(events, routedAt("2026-11-05T11:00:00Z", "codex", "b", 500)...)
	execs := []execwrap.Event{
		execAt("2026-11-05T09:00:00Z", "claude-code", 400, 0),
		execAt("2026-11-05T09:00:00Z", "", 80, 0),
	}
	rows := BuildPeriods(events, execs, PeriodOptions{Per: PerDay, Days: 30, Now: now})
	if len(rows) != 1 || rows[0].Period != "2026-11-05" {
		t.Fatalf("rows = %+v", rows)
	}
	got := map[string]PeriodHarness{}
	for _, h := range rows[0].Harnesses {
		got[h.Harness] = h
	}
	if got["claude-code"].RoutingBelowTokens != 1000 || got["claude-code"].CompactionTokens != 100 {
		t.Errorf("claude-code = %+v", got["claude-code"])
	}
	if got["codex"].RoutingBelowTokens != 500 || got["codex"].CompactionTokens != 0 {
		t.Errorf("codex = %+v", got["codex"])
	}
	if got["unknown"].CompactionTokens != 20 {
		t.Errorf("unknown = %+v", got["unknown"])
	}
}

func TestBuildPeriods_DecisionOnAnotherDayStillLinks(t *testing.T) {
	events := routedAt("2026-11-04T23:59:00Z", "codex", "x", 700)
	events[1].Timestamp = "2026-11-05T00:01:00Z" // usage lands the next day
	rows := BuildPeriods(events, nil, PeriodOptions{Per: PerDay, Now: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)})
	if len(rows) != 1 || rows[0].Period != "2026-11-05" || rows[0].Harnesses[0].RoutingBelowTokens != 700 {
		t.Errorf("rows = %+v", rows)
	}
}

func TestBuildPeriods_WindowAndEmpty(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	execs := []execwrap.Event{
		execAt("2026-10-01T10:00:00Z", "codex", 400, 0),  // outside 3 days
		execAt("2026-11-04T00:00:00Z", "codex", 400, 0),  // first day of the window
		execAt("2026-11-03T23:59:59Z", "codex", 400, 0),  // one second before
		{TS: "garbage", Harness: "codex", RawBytes: 400}, // never in a window
	}
	rows := BuildPeriods(nil, execs, PeriodOptions{Per: PerDay, Days: 3, Fill: true, Now: now})
	if len(rows) != 3 || rows[0].Period != "2026-11-04" || rows[2].Period != "2026-11-06" {
		t.Fatalf("filled rows = %+v", rows)
	}
	if len(rows[0].Harnesses) != 1 || rows[0].Harnesses[0].CompactionTokens != 100 || len(rows[1].Harnesses) != 0 {
		t.Errorf("rows = %+v", rows)
	}
	all := BuildPeriods(nil, execs, PeriodOptions{Per: PerDay, Now: now})
	if len(all) != 3 { // Oct 1, Nov 3, Nov 4
		t.Errorf("history rows = %+v", all)
	}
	if got := BuildPeriods(nil, nil, PeriodOptions{Per: PerDay, Now: now}); len(got) != 0 {
		t.Errorf("empty history = %+v", got)
	}
}

func TestBuildPeriods_JSONShapeHasNoMoney(t *testing.T) {
	rows := BuildPeriods(nil, []execwrap.Event{execAt("2026-11-05T10:00:00Z", "codex", 400, 0)},
		PeriodOptions{Per: PerDay, Now: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)})
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"period":"2026-11-05","harnesses":[{"harness":"codex","compaction_tokens":100,"routing_below_tokens":0,"compactor_commands":1,"compactor_compacted":1}]}]`
	if string(b) != want {
		t.Errorf("json = %s\nwant   %s", b, want)
	}
	for _, bad := range []string{"usd", "cost", "dollar", "$"} {
		if strings.Contains(string(b), bad) {
			t.Errorf("json mentions %q", bad)
		}
	}
}

// A run whose raw output was read back with retrieve counts what it still
// saved, the same figure report and the panel use (NetSavedBytes).
func TestBuildPeriods_SubtractsRetrievedBytes(t *testing.T) {
	ev := execAt("2026-11-05T10:00:00Z", "claude-code", 1000, 200)
	ev.RetrievedBytes = 400
	gone := execAt("2026-11-05T11:00:00Z", "claude-code", 1000, 200)
	gone.RetrievedBytes = 1000 // read back whole: nothing left saved
	rows := BuildPeriods(nil, []execwrap.Event{ev, gone}, PeriodOptions{Per: PerDay, Now: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)})
	if len(rows) != 1 || len(rows[0].Harnesses) != 1 || rows[0].Harnesses[0].CompactionTokens != 100 {
		t.Fatalf("rows = %+v, want 400 net bytes = 100 tokens", rows)
	}
}

// A day with runs and nothing compacted reads apart from a day with no runs.
func TestBuildPeriods_CountsRunsAndCompacted(t *testing.T) {
	kept := execAt("2026-11-05T10:00:00Z", "claude-code", 900, 100)
	short := execwrap.Event{TS: "2026-11-05T11:00:00Z", Harness: "claude-code", RawBytes: 20, EmittedBytes: 20}
	rows := BuildPeriods(nil, []execwrap.Event{kept, short}, PeriodOptions{Per: PerDay, Now: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)})
	if len(rows) != 1 || len(rows[0].Harnesses) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	if h := rows[0].Harnesses[0]; h.Commands != 2 || h.Compacted != 1 || h.CompactionTokens != 200 {
		t.Errorf("cell = %+v, want 2 runs, 1 compacted, 200 tokens", h)
	}
}
