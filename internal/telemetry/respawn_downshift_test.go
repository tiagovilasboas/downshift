// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"
	"time"
)

func TestFailedDownshiftWithin(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	at := func(ago time.Duration) string { return now.Add(-ago).Format(time.RFC3339Nano) }
	moved := Event{CorrelationID: "d1", SessionID: "s", Outcome: OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", Timestamp: at(5 * time.Minute)}
	kept := Event{CorrelationID: "d2", SessionID: "s", Outcome: OutcomeAllow, Verdict: "OK", Timestamp: at(5 * time.Minute)}
	end := func(id string, st ChildStatus, ago time.Duration) Event {
		return Event{CorrelationID: "e-" + id, SessionID: "s", Outcome: OutcomeChildEnded, LinkedDecision: id, ChildStatus: st, Timestamp: at(ago)}
	}
	cases := []struct {
		name   string
		events []Event
		want   bool
	}{
		{"moved down then failed", []Event{moved, end("d1", ChildError, 2*time.Minute)}, true},
		{"moved down then interrupted", []Event{moved, end("d1", ChildInterrupted, 2*time.Minute)}, true},
		{"moved down then completed", []Event{moved, end("d1", ChildCompleted, 2*time.Minute)}, false},
		{"kept then failed", []Event{kept, end("d2", ChildError, 2*time.Minute)}, false},
		{"failure older than the window", []Event{moved, end("d1", ChildError, 11*time.Minute)}, false},
	}
	for _, c := range cases {
		if got := FailedDownshiftWithin(c.events, "s", now); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if FailedDownshiftWithin([]Event{moved, end("d1", ChildError, time.Minute)}, "other", now) {
		t.Error("another session must not match")
	}
	if FailedDownshiftWithin([]Event{moved, end("d1", ChildError, time.Minute)}, "", now) {
		t.Error("an empty session must not match")
	}
}
