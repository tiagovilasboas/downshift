// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func childEv(st telemetry.ChildStatus) telemetry.Event {
	return telemetry.Event{Outcome: telemetry.OutcomeChildEnded, ChildStatus: st}
}

func printed(evs []telemetry.Event) string {
	var b bytes.Buffer
	telemetry.PrintStats(evs, telemetry.StatsOptions{Detail: true}, &b)
	return b.String()
}

func TestPrintStats_NoChildEventsUnchanged(t *testing.T) {
	out := printed(nil)
	if strings.Contains(out, "How children ended") {
		t.Fatalf("section must be absent:\n%s", out)
	}
}

func TestPrintStats_ChildSectionCountsAndOrder(t *testing.T) {
	evs := []telemetry.Event{
		childEv(telemetry.ChildError), childEv(telemetry.ChildCompleted),
		childEv(telemetry.ChildCompleted), childEv(telemetry.ChildUnknown),
		childEv(telemetry.ChildLaunched),
	}
	out := printed(evs)
	i := strings.Index(out, "How children ended")
	if i < 0 {
		t.Fatalf("section missing:\n%s", out)
	}
	sec := out[i:]
	last := -1
	for _, want := range []string{"completed", "launched", "error", "unknown"} {
		j := strings.Index(sec, want)
		if j < 0 || j < last {
			t.Fatalf("%q missing or out of order:\n%s", want, sec)
		}
		last = j
	}
	if strings.Contains(sec, "interrupted ") || strings.Contains(sec, "stopped ") {
		t.Errorf("zero-count statuses must be skipped:\n%s", sec)
	}
	if !strings.Contains(sec, "background start, not a success") {
		t.Errorf("launched note missing")
	}
	if !strings.Contains(out, "Subagent decisions           0") {
		t.Errorf("child events must not change total:\n%s", out)
	}
}

func TestExportSummary_ChildStatus(t *testing.T) {
	now := time.Now()
	empty := telemetry.ExportSummary(nil, 0, now)
	if empty.ChildStatus == nil || len(empty.ChildStatus) != 0 {
		t.Fatalf("want empty non-nil map, got %#v", empty.ChildStatus)
	}
	js, err := telemetry.ExportJSON(empty)
	if err != nil || !strings.Contains(js, `"child_status": {}`) {
		t.Fatalf("json: %v %s", err, js)
	}
	got := telemetry.ExportSummary([]telemetry.Event{childEv(telemetry.ChildError), childEv(telemetry.ChildError), childEv(telemetry.ChildLaunched)}, 0, now)
	if got.ChildStatus["error"] != 2 || got.ChildStatus["launched"] != 1 || got.Total != 0 {
		t.Fatalf("got %#v total=%d", got.ChildStatus, got.Total)
	}
}
