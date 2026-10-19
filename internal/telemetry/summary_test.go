// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func usage(id string, baseline, actual float64) telemetry.Event {
	return telemetry.Event{
		CorrelationID: id, Harness: "claude-code", Outcome: telemetry.OutcomeUsage,
		BaselineCostUSD: &baseline, ActualCostUSD: &actual,
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSummarize(t *testing.T) {
	tests := []struct {
		name                  string
		events                []telemetry.Event
		costEvents            int
		gross, overspend, net float64
	}{
		{"no events", nil, 0, 0, 0, 0},
		{"only savings", []telemetry.Event{usage("a", 1, 0.25), usage("b", 0.5, 0.25)}, 2, 1.0, 0, 1.0},
		{
			// Positive-only sums would report 0.80; the honest net is 0.50.
			"a rewrite that cost more lowers the net",
			[]telemetry.Event{usage("a", 1.0, 0.2), usage("b", 0.1, 0.4)},
			2, 0.8, 0.3, 0.5,
		},
		{"net negative when the router cost more overall", []telemetry.Event{usage("a", 0.1, 0.5)}, 1, 0, 0.4, -0.4},
		{"usage without both costs is not counted", []telemetry.Event{{Harness: "h", Outcome: telemetry.OutcomeUsage}}, 0, 0, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := telemetry.Summarize(tc.events)
			if got.CostEvents != tc.costEvents || !near(got.GrossSavedUSD, tc.gross) ||
				!near(got.OverspendUSD, tc.overspend) || !near(got.NetSavedUSD, tc.net) {
				t.Fatalf("got %+v, want events=%d gross=%v overspend=%v net=%v", got, tc.costEvents, tc.gross, tc.overspend, tc.net)
			}
			if got.Basis != telemetry.SummaryBasis {
				t.Fatalf("basis = %q", got.Basis)
			}
		})
	}
}

// `stats` prints Aggregate(...).Summary(); this pins that the headline and
// Summarize agree on the same events.
func TestSummarize_MatchesStatsHeadline(t *testing.T) {
	events := []telemetry.Event{usage("a", 1.0, 0.2), usage("b", 0.1, 0.4)}
	var sb strings.Builder
	telemetry.PrintStats(events, telemetry.StatsOptions{Days: 0}, &sb)
	if !strings.Contains(sb.String(), "net saved $0.50 (saved $0.80, cost more $0.30") {
		t.Fatalf("headline does not carry the Summarize numbers:\n%s", sb.String())
	}
}

func TestParseEvents_OversizedLineSurfacesError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	good := `{"harness":"h","outcome":"allow"}` + "\n"
	huge := `{"harness":"` + strings.Repeat("x", telemetry.MaxLineBytes+1) + `"}` + "\n"
	if err := os.WriteFile(path, []byte(good+huge+good), 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := telemetry.ReadEventsFrom(path)
	if err == nil {
		t.Fatal("expected an error for a line above MaxLineBytes")
	}
	if len(events) != 1 {
		t.Fatalf("events before the bad line must be kept, got %d", len(events))
	}
}

func TestParseEvents_LargeLineUnderLimitIsRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	line := `{"harness":"` + strings.Repeat("x", 200*1024) + `","outcome":"allow"}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := telemetry.ReadEventsFrom(path)
	if err != nil || len(events) != 1 {
		t.Fatalf("200 KiB line: events=%d err=%v", len(events), err)
	}
}

func TestRotatedBackupCount(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	if n := telemetry.RotatedBackupCount(); n != 0 {
		t.Fatalf("no backups yet, got %d", n)
	}
	for _, name := range []string{"events.jsonl.2026-01-01T00-00-00", "events.jsonl.2026-02-01T00-00-00", "events.jsonl.lock", "events.jsonl"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if n := telemetry.RotatedBackupCount(); n != 2 {
		t.Fatalf("backups = %d, want 2 (lock and live log excluded)", n)
	}
}
