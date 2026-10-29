// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"reflect"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestCompute_Ledger(t *testing.T) {
	recent := cheaperChild("s1", "new", time.Hour, 100, 1000)
	old := cheaperChild("s1", "old", 40*24*time.Hour, 500, 0)
	both := append(append([]telemetry.Event{}, recent...), old...)
	execs := []execwrap.Event{exec("s1", time.Hour, 400, 0, true), exec("s1", 40*24*time.Hour, 4000, 0, true)}
	note := compactorEstimateNote
	tests := []struct {
		name   string
		events []telemetry.Event
		execs  []execwrap.Event
		scope  Scope
		want   Ledger
	}{
		{name: "empty logs", scope: Scope{Kind: ScopeDays, Days: 7},
			want: Ledger{CompactorEstimate: note}},
		{name: "routing only", events: recent, scope: Scope{Kind: ScopeDays, Days: 7},
			want: Ledger{Spent: 1100, RoutingSaved: 1100, CompactorEstimate: note}},
		{name: "compactor only, saved bytes over four", execs: []execwrap.Event{exec("", time.Hour, 4100, 100, true)},
			scope: Scope{Kind: ScopeDays, Days: 7},
			want:  Ledger{CompactorSaved: 1000, CompactorEstimate: note}},
		{name: "emitted above raw saves nothing", execs: []execwrap.Event{exec("", time.Hour, 100, 400, false)},
			scope: Scope{Kind: ScopeAll},
			want:  Ledger{CompactorEstimate: note}},
		{name: "window drops old routing and old compactor", events: both, execs: execs,
			scope: Scope{Kind: ScopeDays, Days: 30},
			want:  Ledger{Spent: 1100, RoutingSaved: 1100, CompactorSaved: 100, CompactorEstimate: note}},
		{name: "all time keeps both", events: both, execs: execs, scope: Scope{Kind: ScopeAll},
			want: Ledger{Spent: 1600, RoutingSaved: 1600, CompactorSaved: 1100, CompactorEstimate: note}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Compute(tc.events, tc.execs, tc.scope).Ledger
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ledger = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLedgerLine(t *testing.T) {
	tests := []struct {
		name  string
		l     Ledger
		scope Scope
		want  string
	}{
		{name: "zero", l: Ledger{CompactorEstimate: compactorEstimateNote}, scope: Scope{Kind: ScopeDays, Days: 7},
			want: "Token ledger (last 7 days): 0 spent · 0 saved by routing · 0 saved by compactor (estimate: saved bytes / 4)"},
		{name: "all three", l: Ledger{Spent: 1_200_000, RoutingSaved: 300_000, CompactorSaved: 45_000, CompactorEstimate: compactorEstimateNote},
			scope: Scope{Kind: ScopeAll},
			want:  "Token ledger (all time): 1.2M spent · 300k saved by routing · 45k saved by compactor (estimate: saved bytes / 4)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LedgerLine(tc.l, tc.scope); got != tc.want {
				t.Errorf("LedgerLine = %q, want %q", got, tc.want)
			}
		})
	}
}
