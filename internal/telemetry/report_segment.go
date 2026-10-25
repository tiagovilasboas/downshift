// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"io"
	"sort"
	"time"
)

// HarnessReport is one harness slice of the window. Current is the harness
// of the latest event in that window.
type HarnessReport struct {
	Harness string `json:"harness"`
	Current bool   `json:"current,omitempty"`
	Report  Report `json:"report"`
}

// SegmentReports splits the window by harness. The current harness (latest
// event) is first; the rest follow in name order. A log with one harness, or
// with no harness field, returns a single segment that is the whole window.
func SegmentReports(events []Event, days int) (current string, segments []HarnessReport) {
	windowed := FilterByDays(events, days)
	current = latestHarness(windowed)
	names := harnessNames(windowed)
	if current == "" || len(names) <= 1 {
		return current, []HarnessReport{{
			Harness: current,
			Current: current != "",
			Report:  BuildReport(events, days),
		}}
	}
	segments = append(segments, HarnessReport{
		Harness: current,
		Current: true,
		Report:  BuildReport(filterHarness(windowed, current), 0),
	})
	var others []string
	for _, name := range names {
		if name != current {
			others = append(others, name)
		}
	}
	sort.Strings(others)
	for _, name := range others {
		segments = append(segments, HarnessReport{
			Harness: name,
			Report:  BuildReport(filterHarness(windowed, name), 0),
		})
	}
	return current, segments
}

// PrintSegmentedReport prints the current harness in full, then each other
// harness as its own routed-versus-control block. One harness keeps the
// original report text.
func PrintSegmentedReport(w io.Writer, events []Event, days int) {
	current, segments := SegmentReports(events, days)
	if len(segments) <= 1 {
		PrintReport(w, segments[0].Report)
		return
	}
	fmt.Fprintf(w, "Current harness: %s\n\n", current)
	PrintReport(w, segments[0].Report)
	fmt.Fprintln(w, "Other harnesses")
	for _, seg := range segments[1:] {
		fmt.Fprintf(w, "\n%s\n", seg.Harness)
		printHarnessDigest(w, seg.Report)
	}
}

func printHarnessDigest(w io.Writer, r Report) {
	fmt.Fprintf(w, "%-9s %7s %7s %10s %17s %9s\n", "group", "spawns", "ended", "completed", "tokens/completed", "rework")
	for _, row := range []struct {
		name string
		g    GroupSummary
	}{{"routed", r.Routed}, {"control", r.Control}} {
		tokens := "-"
		if row.g.Completed > 0 {
			tokens = formatTokens(int64(row.g.TokensPerTask + 0.5))
		}
		rework := "-"
		if row.g.Ended > 0 {
			rework = fmt.Sprintf("%.1f%%", row.g.ReworkRate*100)
		}
		fmt.Fprintf(w, "%-9s %7d %7d %10d %17s %9s\n", row.name, row.g.Spawns, row.g.Ended, row.g.Completed, tokens, rework)
	}
	if line := r.TokensSaved.Line(); line != "" {
		fmt.Fprintf(w, "  %s\n", line)
	}
	if r.Message != "" && !r.Enough {
		fmt.Fprintf(w, "  %s.\n", r.Message)
	}
}

func latestHarness(events []Event) string {
	var name string
	var best time.Time
	var seen bool
	for _, ev := range events {
		if ev.Harness == "" || ev.Timestamp == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, ev.Timestamp)
		if err != nil {
			t, err = time.Parse(time.RFC3339, ev.Timestamp)
		}
		if err != nil {
			continue
		}
		if !seen || t.After(best) {
			seen = true
			best = t
			name = ev.Harness
		}
	}
	return name
}

func harnessNames(events []Event) []string {
	seen := map[string]bool{}
	var names []string
	for _, ev := range events {
		if ev.Harness == "" || seen[ev.Harness] {
			continue
		}
		seen[ev.Harness] = true
		names = append(names, ev.Harness)
	}
	return names
}

func filterHarness(events []Event, harness string) []Event {
	var out []Event
	for _, ev := range events {
		if ev.Harness == harness {
			out = append(out, ev)
		}
	}
	return out
}
