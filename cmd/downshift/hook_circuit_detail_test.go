// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func circuitRow(version, field string, age time.Duration) telemetry.Event {
	ev := circuitEvent("opencode", telemetry.OutcomeFailOpen, "INVALID_REQUEST", age)
	ev.BinaryVersion = version
	ev.InvalidField = field
	return ev
}

func TestOpenCircuitsAt_DevFailuresAndDetail(t *testing.T) {
	now := time.Now()
	day := circuitDevFailureTTL
	cases := []struct {
		name      string
		events    []telemetry.Event
		wantOpen  bool
		wantField string
		wantSince time.Duration // age of the first failure of the run
	}{
		{name: "recent dev failures count",
			events:   []telemetry.Event{circuitRow("dev", "model.id", 3*time.Hour), circuitRow("dev", "model.id", 2*time.Hour), circuitRow("dev", "model.id", time.Hour)},
			wantOpen: true, wantField: "model.id", wantSince: 3 * time.Hour},
		{name: "stale dev failures are ignored",
			events: []telemetry.Event{circuitRow("dev", "model.id", day+3*time.Hour), circuitRow("dev", "model.id", day+2*time.Hour), circuitRow("dev", "model.id", day+time.Hour)}},
		{name: "stale non-dev failures still count",
			events:   []telemetry.Event{circuitRow("0.8.1", "agent", day+3*time.Hour), circuitRow("0.8.1", "agent", day+2*time.Hour), circuitRow("0.8.1", "agent", day+time.Hour)},
			wantOpen: true, wantField: "agent", wantSince: day + 3*time.Hour},
		{name: "the run extends past the trip count",
			events:   []telemetry.Event{circuitRow("0.8.1", "", 5*time.Hour), circuitRow("0.8.1", "", 4*time.Hour), circuitRow("0.8.1", "", 3*time.Hour), circuitRow("0.8.1", "", time.Hour)},
			wantOpen: true, wantSince: 5 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := openCircuitsAt(tc.events, now)
			if (len(got) == 1) != tc.wantOpen {
				t.Fatalf("open = %v, want %v: %+v", len(got) == 1, tc.wantOpen, got)
			}
			if !tc.wantOpen {
				return
			}
			if got[0].Field != tc.wantField {
				t.Errorf("field = %q, want %q", got[0].Field, tc.wantField)
			}
			if d := now.Sub(got[0].Since) - tc.wantSince; d < -time.Minute || d > time.Minute {
				t.Errorf("since age = %v, want about %v", now.Sub(got[0].Since), tc.wantSince)
			}
		})
	}
}

func TestRoutingCircuit_DetailHalfOpen(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		age  time.Duration
		want string
	}{
		{name: "inside the retry window", age: time.Minute, want: "open: retry window starts"},
		{name: "past the retry window", age: circuitProbeAfter + time.Minute, want: "half-open"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := circuitRow("0.8.1", "model.id", tc.age)
			got := openCircuitsAt([]telemetry.Event{ev, ev, ev}, now)
			if len(got) != 1 {
				t.Fatalf("circuits = %+v", got)
			}
			d := got[0].detail(now)
			for _, w := range []string{tc.want, "field model.id", "error INVALID_REQUEST", "open since"} {
				if !strings.Contains(d, w) {
					t.Errorf("detail %q lacks %q", d, w)
				}
			}
		})
	}
}
