// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

const keptID = "0123456789ab"

// seedRetrieval writes one compacted run (raw 41000 cut to 1000, kept under
// keptID, session "s1") and one retrieval of n bytes through the same log
// writers `downshift compact` and `retrieve` use, then reads the exec log back
// the way every surface does.
func seedRetrieval(t *testing.T, retrieved int64) []execwrap.Event {
	t.Helper()
	t.Setenv(paths.EnvStateDir, t.TempDir())
	now := time.Now().UTC()
	path, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	ev := execwrap.EventFrom(execwrap.Result{
		Family: execwrap.FamilyGoTest, RawBytes: 41000, EmittedBytes: 1000,
		ExitClass: execwrap.ExitClassZero, Compacted: true, KeptID: keptID,
	}, now)
	ev.Session, ev.Harness = "s1", "claude-code"
	if err := execwrap.Append(path, ev); err != nil {
		t.Fatal(err)
	}
	execwrap.RecordRetrieval(keptID, retrieved, now)
	evs, err := ReadExecEvents()
	if err != nil {
		t.Fatal(err)
	}
	return evs
}

// Every surface that totals the Compactor reads the same net saving: report and
// session (Compute), the per-harness lead, the detail and the period rows.
func TestRetrieval_AllSurfacesAgree(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name          string
		retrieved     int64
		wantSaved     int64
		wantRetrieved int64
	}{
		{"nothing retrieved", 0, 40000, 0},
		{"partly retrieved", 10000, 30000, 10000},
		{"fully retrieved", 40000, 0, 40000},
		{"retrieved more than it saved", 90000, 0, 90000},
	}
	scopes := []Scope{{Kind: ScopeAll}, {Kind: ScopeDays, Days: 30}, {Kind: ScopeSession, SessionHash: "s1"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evs := seedRetrieval(t, tc.retrieved)
			wantTokens := tc.wantSaved / execwrap.BytesPerToken
			for _, scope := range scopes {
				n := Compute(nil, evs, scope)
				c := n.Compactor
				if c.SavedBytes != tc.wantSaved || c.RetrievedBytes != tc.wantRetrieved || c.Tokens != wantTokens {
					t.Fatalf("%s: compactor = %+v", scope.Label(), c)
				}
				if n.TokensSavedEstimate != wantTokens {
					t.Errorf("%s: tokens saved = %d, want %d", scope.Label(), n.TokensSavedEstimate, wantTokens)
				}
				line := n.Display.Retrieved
				if (tc.wantRetrieved == 0) != (line == "") {
					t.Errorf("%s: retrieved line = %q for %d retrieved", scope.Label(), line, tc.wantRetrieved)
				}
				if tc.wantRetrieved > 0 && !strings.Contains(line, "bytes") {
					t.Errorf("retrieved line must state bytes: %q", line)
				}
			}
			var avoided int64
			for _, h := range TokensByHarness(nil, evs, scopes[0], nil) {
				avoided += h.CompactorAvoided
			}
			if avoided != wantTokens {
				t.Errorf("tokens by harness = %d, want %d", avoided, wantTokens)
			}
			detail := BuildCompactionDetail(evs, nil, scopes[0], now)
			var families, cells int
			for _, h := range detail.Harnesses {
				for _, f := range h.Families {
					families++
					if f.AvoidedTokens != wantTokens {
						t.Errorf("detail %s/%s avoided = %d, want %d", h.Harness, f.Family, f.AvoidedTokens, wantTokens)
					}
				}
			}
			var period int64
			for _, row := range BuildPeriods(nil, evs, PeriodOptions{Per: PerDay, Days: 2, Now: now}) {
				for _, h := range row.Harnesses {
					cells++
					period += h.CompactionTokens
				}
			}
			wantCells := 0 // a period cell exists only for a run that saved something
			if tc.wantSaved > 0 {
				wantCells = 1
			}
			if families != 1 || cells != wantCells {
				t.Fatalf("detail families = %d (want 1), period cells = %d (want %d)", families, cells, wantCells)
			}
			if period != wantTokens {
				t.Errorf("period tokens = %d, want %d", period, wantTokens)
			}
		})
	}
}
