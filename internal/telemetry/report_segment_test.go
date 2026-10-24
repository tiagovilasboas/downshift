// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestPrintSegmentedReport_CurrentHarnessFirstThenOthers(t *testing.T) {
	now := time.Now().UTC()
	events := []Event{
		{Timestamp: now.Add(-2 * time.Hour).Format(time.RFC3339Nano), Harness: "claude-code", Outcome: OutcomeAllow, CorrelationID: "cc"},
		{Timestamp: now.Add(-time.Hour).Format(time.RFC3339Nano), Harness: "codex", Outcome: OutcomeAllow, CorrelationID: "cx"},
		{Timestamp: now.Add(-time.Minute).Format(time.RFC3339Nano), Harness: "cursor", Outcome: OutcomeAllow, CorrelationID: "cu"},
	}
	current, segments := SegmentReports(events, 30)
	if current != "cursor" || len(segments) != 3 || segments[0].Harness != "cursor" || !segments[0].Current {
		t.Fatalf("current=%q segments=%v", current, segmentNames(segments))
	}
	if segments[1].Harness != "claude-code" || segments[2].Harness != "codex" {
		t.Fatalf("others = %v, want claude-code then codex", segmentNames(segments))
	}
	if got := countDecisionTasks(segments[0].Report); got != 1 {
		t.Fatalf("cursor tasks = %d, want 1", got)
	}

	var b bytes.Buffer
	PrintSegmentedReport(&b, events, 30)
	text := b.String()
	cur := strings.Index(text, "Current harness: cursor")
	other := strings.Index(text, "Other harnesses")
	cc := strings.Index(text, "\nclaude-code\n")
	cx := strings.Index(text, "\ncodex\n")
	if cur < 0 || other < cur || cc < other || cx < cc {
		t.Fatalf("order broken:\n%s", text)
	}
}

func TestPrintSegmentedReport_OneHarnessStaysPlain(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	events := []Event{{Timestamp: now, Harness: "cursor", Outcome: OutcomeAllow, CorrelationID: "only"}}
	var b bytes.Buffer
	PrintSegmentedReport(&b, events, 30)
	if strings.Contains(b.String(), "Other harnesses") || strings.Contains(b.String(), "Current harness:") {
		t.Fatalf("single harness should keep the plain report:\n%s", b.String())
	}
	if !strings.Contains(b.String(), "Routed vs control") {
		t.Fatalf("missing comparison:\n%s", b.String())
	}
}

func segmentNames(segments []HarnessReport) []string {
	var names []string
	for _, s := range segments {
		names = append(names, s.Harness)
	}
	return names
}

func countDecisionTasks(r Report) int {
	return r.Routed.Spawns + r.Control.Spawns + r.Excluded
}
