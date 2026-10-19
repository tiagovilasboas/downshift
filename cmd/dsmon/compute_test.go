// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func ev(e telemetry.Event) event { return event{Event: e} }

// dsmon counts savings only for applied rewrites and shows held decisions
// as unchanged.
func TestCompute_CountsOnlyAppliedRewrites(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	all := []event{
		ev(telemetry.Event{Timestamp: ts, Harness: "claude-code", Verdict: "DOWNSHIFT", EstimatedSavings: 0.8, Outcome: "rewrite_emitted", QuotaStatus: "available"}),
		ev(telemetry.Event{Timestamp: ts, Harness: "claude-code", Verdict: "DOWNSHIFT", EstimatedSavings: 0.4, Outcome: "allow"}),
		ev(telemetry.Event{Timestamp: ts, Harness: "codex", Verdict: "DOWNSHIFT", EstimatedSavings: 0.5, Outcome: "rewrite_emitted", Corrections: []string{"R1_UNCONFIDENT_DOWNSHIFT"}}),
		ev(telemetry.Event{Timestamp: ts, Harness: "claude-code", Verdict: "DOWNSHIFT", Outcome: "usage"}),
	}
	st, recent := compute(all)
	if st.total != 3 || st.down != 1 || st.held != 2 {
		t.Fatalf("total=%d down=%d held=%d, want 3/1/2", st.total, st.down, st.held)
	}
	if st.totalSavingsUnits < 0.79 || st.totalSavingsUnits > 0.81 {
		t.Fatalf("savings units=%.2f, want 0.8", st.totalSavingsUnits)
	}
	if len(recent) != 3 || recent[1].Verdict != "HELD" {
		t.Fatalf("held decision must render as HELD, got %+v", recent)
	}
}

func usageLine(t *testing.T, ts string, baseline, actual float64) string {
	t.Helper()
	b, err := json.Marshal(telemetry.Event{
		Timestamp: ts, Harness: "claude-code", Outcome: "usage",
		BaselineCostUSD: &baseline, ActualCostUSD: &actual,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

// A rewrite that cost more must lower the widget's number: it shows the net
// value `downshift stats` prints, not the positive-only sum.
func TestCompute_SavingsAreNet(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	base1, act1, base2, act2 := 1.0, 0.2, 0.1, 0.4
	all := []event{
		ev(telemetry.Event{Timestamp: ts, Harness: "h", Outcome: "usage", BaselineCostUSD: &base1, ActualCostUSD: &act1}),
		ev(telemetry.Event{Timestamp: ts, Harness: "h", Outcome: "usage", BaselineCostUSD: &base2, ActualCostUSD: &act2}),
	}
	st, _ := compute(all)
	if st.savings.CostEvents != 2 || diff(st.savings.NetSavedUSD, 0.5) || diff(st.savings.GrossSavedUSD, 0.8) {
		t.Fatalf("savings = %+v, want net 0.5 (gross 0.8, overspend 0.3)", st.savings)
	}
}

func diff(a, b float64) bool {
	d := a - b
	return d > 1e-9 || d < -1e-9
}

// Lines appended after the first read are read once, partial lines wait for
// their newline, and the offset follows the bytes actually consumed.
func TestPoll_AppendedLinesReadOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	line := func(i int) string {
		return fmt.Sprintf(`{"timestamp":%q,"harness":"h%d","verdict":"OK","outcome":"allow"}`+"\n", ts, i)
	}
	write := func(flag int, s string) {
		f, err := os.OpenFile(path, flag, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.WriteString(s); err != nil {
			t.Fatal(err)
		}
	}
	write(os.O_CREATE|os.O_WRONLY, line(1)+line(2))

	s := &state{}
	s.reload(path)
	if len(s.all) != 2 {
		t.Fatalf("after reload: %d events, want 2", len(s.all))
	}

	// One full line plus half of another: only the full one counts.
	half := line(4)
	write(os.O_APPEND|os.O_WRONLY, line(3)+half[:20])
	s.poll(path)
	if len(s.all) != 3 {
		t.Fatalf("after append: %d events, want 3", len(s.all))
	}
	s.poll(path) // nothing new: must not double-read
	if len(s.all) != 3 {
		t.Fatalf("after idle poll: %d events, want 3", len(s.all))
	}

	write(os.O_APPEND|os.O_WRONLY, half[20:])
	s.poll(path)
	if len(s.all) != 4 || s.all[3].Harness != "h4" {
		t.Fatalf("after completing the partial line: %d events, last %q, want 4 / h4", len(s.all), s.all[len(s.all)-1].Harness)
	}
	fi, _ := os.Stat(path)
	if s.size != fi.Size() {
		t.Fatalf("offset = %d, want file size %d", s.size, fi.Size())
	}
}

func TestPrune_BoundsMemory(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	s := &state{}
	for i := 0; i < maxKeptEvents+100; i++ {
		s.all = append(s.all, ev(telemetry.Event{Timestamp: ts, Harness: "h"}))
	}
	old := ev(telemetry.Event{Timestamp: time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339Nano), Harness: "old"})
	s.all = append([]event{old}, s.all...)
	s.prune()
	if len(s.all) != maxKeptEvents {
		t.Fatalf("kept %d events, want %d", len(s.all), maxKeptEvents)
	}
	for _, e := range s.all {
		if e.Harness == "old" {
			t.Fatal("event outside the 24h window was kept")
		}
	}
}

func TestReadNewLines_SkipsOversizedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.jsonl")
	big := strings.Repeat("a", telemetry.MaxLineBytes+10)
	if err := os.WriteFile(path, []byte("one\n"+big+"\ntwo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var got []string
	n, err := readNewLines(f, 0, func(l []byte) { got = append(got, strings.TrimSpace(string(l))) })
	if err != nil || len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("got %v err %v, want [one two]", got, err)
	}
	fi, _ := f.Stat()
	if n != fi.Size() {
		t.Fatalf("offset %d, want %d", n, fi.Size())
	}
}
