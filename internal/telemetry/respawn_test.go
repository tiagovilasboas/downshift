// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"
	"time"
)

var base = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

func at(d time.Duration) string { return base.Add(d).Format(time.RFC3339Nano) }

func decision(id, sess, tier string, d time.Duration) Event {
	return Event{CorrelationID: id, Timestamp: at(d), SessionID: sess, Tier: tier, Outcome: OutcomeRewriteEmitted}
}

func childEnd(link, sess string, st ChildStatus, d time.Duration) Event {
	return Event{CorrelationID: "c-" + link, Timestamp: at(d), SessionID: sess, Outcome: OutcomeChildEnded, LinkedDecision: link, ChildStatus: st}
}

func TestRespawnAfterFailure(t *testing.T) {
	cases := []struct {
		name   string
		events []Event
		want   int
	}{
		{"error then respawn", []Event{
			decision("d1", "s", "SMALL", 0), childEnd("d1", "s", ChildError, time.Minute), decision("d2", "s", "MID", 2*time.Minute),
		}, 1},
		{"completed child is not a retry", []Event{
			decision("d1", "s", "SMALL", 0), childEnd("d1", "s", ChildCompleted, time.Minute), decision("d2", "s", "MID", 2*time.Minute),
		}, 0},
		{"respawn outside the window", []Event{
			decision("d1", "s", "SMALL", 0), childEnd("d1", "s", ChildError, time.Minute), decision("d2", "s", "MID", 2*time.Hour),
		}, 0},
		{"other session is not a retry", []Event{
			decision("d1", "s", "SMALL", 0), childEnd("d1", "s", ChildError, time.Minute), decision("d2", "other", "MID", 2*time.Minute),
		}, 0},
		{"no session cannot be attributed", []Event{
			decision("d1", "", "SMALL", 0), childEnd("d1", "", ChildError, time.Minute), decision("d2", "", "MID", 2*time.Minute),
		}, 0},
	}
	for _, c := range cases {
		got := RespawnAfterFailure(c.events, 10*time.Minute)
		if len(got) != c.want {
			t.Errorf("%s: %d retries, want %d", c.name, len(got), c.want)
		}
	}
}

func TestRespawnAfterFailure_OneFailureOneRetry(t *testing.T) {
	ev := []Event{
		decision("d1", "s", "SMALL", 0), childEnd("d1", "s", ChildInterrupted, time.Minute),
		decision("d2", "s", "MID", 2*time.Minute), decision("d3", "s", "MID", 3*time.Minute),
	}
	got := RespawnAfterFailure(ev, 10*time.Minute)
	if len(got) != 1 || got[0].FailedTier != "SMALL" || got[0].NextTier != "MID" {
		t.Fatalf("got %+v", got)
	}
}
