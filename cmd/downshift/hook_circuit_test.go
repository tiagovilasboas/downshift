// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

const circuitSpawn = `{"hook_event_name":"preToolUse","tool_name":"Task","tool_input":{"task":"rename the userId variable to userIdentifier"}}`

// circuitEvent is one seeded event-log row for harness, age before now.
func circuitEvent(harness, outcome, code string, age time.Duration) telemetry.Event {
	return telemetry.Event{Source: "hook", Agent: "subagent", Harness: harness, Outcome: outcome,
		ErrorCode: code, Timestamp: time.Now().Add(-age).UTC().Format(time.RFC3339Nano)}
}

func cursorFailure(code string) telemetry.Event {
	return circuitEvent("cursor", "error", code, time.Second)
}

func seedCircuitLog(t *testing.T, path string, events []telemetry.Event) {
	t.Helper()
	for _, ev := range events {
		if err := telemetry.AppendTo(path, ev); err != nil {
			t.Fatal(err)
		}
	}
}

func readCircuitLog(t *testing.T, path string) []telemetry.Event {
	t.Helper()
	events, err := telemetry.ReadEventsFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

// circuitHookCall is what one shared-runner call did.
type circuitHookCall struct {
	routed, failedOpen bool
	stderr             string
}

// runCircuitHook runs one cursor hook call through runHookAdapter.
func runCircuitHook(t *testing.T, stdin string) circuitHookCall {
	t.Helper()
	var call circuitHookCall
	captureStdout(func() {
		call.stderr = captureStderr(func() {
			rc := runHookAdapter("cursor", strings.NewReader(stdin),
				func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
				func(e cursor.Event) (any, string, core.Decision) {
					call.routed = true
					return cursor.Handle(e, cmdCat)
				},
				rewriteWire(func() { call.failedOpen = true }))
			if rc != 0 {
				t.Errorf("rc = %d, want 0", rc)
			}
		})
	})
	return call
}

func outcomesOf(events []telemetry.Event) []string {
	var out []string
	for _, ev := range events {
		out = append(out, ev.Outcome)
	}
	return out
}

func TestRunHookAdapter_RoutingCircuit(t *testing.T) {
	line := "circuit open for cursor after 3 INVALID_EVENT failures: run downshift doctor and fix the cause"
	fresh := cursorFailure("INVALID_EVENT")
	cases := []struct {
		name string
		seed []telemetry.Event
		// stdin defaults to a valid spawn.
		stdin        string
		wantRouted   bool
		wantFailOpen bool
		wantAppended []string
	}{
		{name: "three identical failures open the circuit",
			seed:         []telemetry.Event{fresh, fresh, fresh},
			wantFailOpen: true},
		{name: "error and fail_open with one code trip together",
			seed:         []telemetry.Event{fresh, circuitEvent("cursor", telemetry.OutcomeFailOpen, "INVALID_EVENT", time.Second), fresh},
			wantFailOpen: true},
		{name: "usage rows do not break a streak",
			seed:         []telemetry.Event{fresh, circuitEvent("cursor", telemetry.OutcomeUsage, "", time.Second), fresh, fresh},
			wantFailOpen: true},
		{name: "an open circuit records no further failure",
			seed:         []telemetry.Event{fresh, fresh, fresh},
			stdin:        "{not json",
			wantFailOpen: true},
		{name: "mixed codes do not trip",
			seed:         []telemetry.Event{fresh, cursorFailure("PAYLOAD_TOO_LARGE"), fresh},
			wantRouted:   true,
			wantAppended: []string{telemetry.OutcomeAllow}},
		{name: "two failures do not trip",
			seed:         []telemetry.Event{fresh, fresh},
			wantRouted:   true,
			wantAppended: []string{telemetry.OutcomeAllow}},
		{name: "a routed decision closes the circuit",
			seed:         []telemetry.Event{fresh, fresh, fresh, circuitEvent("cursor", telemetry.OutcomeBaseline, "", time.Second)},
			wantRouted:   true,
			wantAppended: []string{telemetry.OutcomeAllow}},
		{name: "another harness's circuit stays with it",
			seed: []telemetry.Event{circuitEvent("codex", "error", "PANIC", time.Second),
				circuitEvent("codex", "error", "PANIC", time.Second), circuitEvent("codex", "error", "PANIC", time.Second)},
			wantRouted:   true,
			wantAppended: []string{telemetry.OutcomeAllow}},
		{name: "a probe routes once the interval has passed",
			seed: []telemetry.Event{circuitEvent("cursor", "error", "INVALID_EVENT", time.Hour),
				circuitEvent("cursor", "error", "INVALID_EVENT", time.Hour), circuitEvent("cursor", "error", "INVALID_EVENT", circuitProbeAfter+time.Minute)},
			wantRouted:   true,
			wantAppended: []string{telemetry.OutcomeAllow}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			logPath := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)
			seedCircuitLog(t, logPath, tc.seed)
			stdin := tc.stdin
			if stdin == "" {
				stdin = circuitSpawn
			}

			call := runCircuitHook(t, stdin)

			if call.routed != tc.wantRouted {
				t.Errorf("routed = %v, want %v", call.routed, tc.wantRouted)
			}
			if call.failedOpen != tc.wantFailOpen {
				t.Errorf("fail-open response = %v, want %v", call.failedOpen, tc.wantFailOpen)
			}
			if held := strings.Contains(call.stderr, line); held == tc.wantRouted {
				t.Errorf("stderr circuit line present = %v, want %v; stderr:\n%s", held, !tc.wantRouted, call.stderr)
			}
			appended := outcomesOf(readCircuitLog(t, logPath)[len(tc.seed):])
			if strings.Join(appended, ",") != strings.Join(tc.wantAppended, ",") {
				t.Errorf("appended outcomes = %v, want %v", appended, tc.wantAppended)
			}
		})
	}
}

// The runner's own failures trip the circuit: three bad payloads are recorded,
// the fourth call is held without routing and adds nothing to the log.
func TestRunHookAdapter_CircuitTripsOnItsOwnFailures(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	logPath := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)
	for i := 0; i < circuitTripCount; i++ {
		if call := runCircuitHook(t, "{not json"); !call.failedOpen {
			t.Fatalf("call %d: fail-open response not emitted", i+1)
		}
	}
	if got := outcomesOf(readCircuitLog(t, logPath)); strings.Join(got, ",") != "error,error,error" {
		t.Fatalf("recorded outcomes = %v, want three errors", got)
	}
	call := runCircuitHook(t, circuitSpawn)
	if call.routed || !call.failedOpen {
		t.Fatalf("fourth call routed=%v failedOpen=%v, want held with the fail-open response", call.routed, call.failedOpen)
	}
	if n := len(readCircuitLog(t, logPath)); n != circuitTripCount {
		t.Fatalf("event log has %d rows, want %d: an open circuit records nothing", n, circuitTripCount)
	}
}

// OpenCode's fail-open answer keeps its shape while the circuit is open, and
// the wire records nothing for it.
func TestOpenCodeWire_CircuitOpenKeepsShapeRecordsNothing(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)
	held := captureStdout(func() { openCodeWire{}.failOpen(wireFailOpenCircuit, "c1") })
	if n := len(readCircuitLog(t, logPath)); n != 0 {
		t.Fatalf("circuit-open answer recorded %d events, want 0", n)
	}
	failed := captureStdout(func() { openCodeWire{}.failOpen("PANIC", "c2") })
	if held != failed {
		t.Fatalf("circuit-open answer %q differs from the fail-open answer %q", held, failed)
	}
}

// BenchmarkHookCircuitState is the per-spawn cost of the breaker: one bounded
// tail read and walk over a full window of decision rows.
func BenchmarkHookCircuitState(b *testing.B) {
	logPath := filepath.Join(b.TempDir(), "events.jsonl")
	b.Setenv("DOWNSHIFT_EVENT_LOG", logPath)
	for i := 0; i < 2000; i++ {
		ev := circuitEvent("cursor", telemetry.OutcomeRewriteEmitted, "", time.Second)
		ev.FromModel, ev.ToModel, ev.Verdict, ev.Tier = "frontier-model", "small-model", "DOWNSHIFT", "SMALL"
		if err := telemetry.AppendTo(logPath, ev); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		breaker := hookCircuit{harness: "cursor"}
		if _, open := breaker.state(); open {
			b.Fatal("decision rows opened the circuit")
		}
	}
}

func TestDoctor_ShowsOneLinePerOpenCircuit(t *testing.T) {
	f := newDoctorFixture(t)
	logPath := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)
	panicked := circuitEvent("codex", "error", "PANIC", time.Hour)
	seedCircuitLog(t, logPath, []telemetry.Event{panicked, panicked, panicked,
		cursorFailure("INVALID_EVENT"), cursorFailure("READ_ERROR"), cursorFailure("INVALID_EVENT")})

	rep := buildDoctorReport(f.env)
	rep.width = 200
	var out bytes.Buffer
	rep.writeText(&out, false, false)

	want := "circuit open for codex after 3 PANIC failures: run downshift doctor and fix the cause"
	if n := strings.Count(out.String(), "circuit open for"); n != 1 {
		t.Fatalf("doctor shows %d circuit lines, want 1:\n%s", n, out.String())
	}
	if !strings.Contains(out.String(), want) {
		t.Fatalf("doctor output lacks %q:\n%s", want, out.String())
	}
	if rep.Failed == 0 {
		t.Fatal("an open circuit must fail doctor")
	}
}
