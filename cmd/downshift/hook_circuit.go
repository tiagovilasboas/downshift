// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Routing circuit breaker: after repeated identical failures, stop and ask a
// human. The state is the event log itself, so no state file is kept: when the
// last circuitTripCount hook events of one harness are failures (outcome error
// or fail_open) with the same error code, that harness's circuit is open.
// Only failures and routed decisions count; usage, honor and other rows are
// skipped. A routed decision (allow, baseline, rewrite_emitted, blocked) for
// the harness closes the circuit.
//
// While it is open the shared runner allows each spawn unchanged without
// routing it and records no further failure. Once circuitProbeAfter has passed
// since the newest failure, the runner routes again (half-open): a fixed cause
// records a decision and closes the circuit, the same failure reopens it for
// another interval. Every harness gets this from runHookAdapter.

const (
	circuitTripCount  = 3
	circuitProbeAfter = 5 * time.Minute
	// circuitTailBytes bounds the hook's read of the event log to its newest
	// events; the hook never scans the whole file.
	circuitTailBytes int64 = 128 << 10
)

// wireFailOpenCircuit is the failOpen cause while the circuit is open: the
// harness's usual fail-open response, with nothing recorded.
const wireFailOpenCircuit = "circuit_open"

// routingCircuit is one harness whose circuit is open.
type routingCircuit struct {
	Harness string
	Code    string
	// LastFailure is the newest failure's time; zero when it does not parse.
	LastFailure time.Time
}

// line is the one sentence a human reads, on hook stderr and in doctor.
func (c routingCircuit) line() string {
	return fmt.Sprintf("circuit open for %s after %d %s failures: run downshift doctor and fix the cause",
		c.Harness, circuitTripCount, c.Code)
}

// isRoutedOutcome reports whether an outcome is a routing decision the hook
// made for a spawn, which closes that harness's circuit.
func isRoutedOutcome(outcome string) bool {
	switch outcome {
	case telemetry.OutcomeAllow, telemetry.OutcomeBaseline, telemetry.OutcomeRewriteEmitted, telemetry.OutcomeBlocked:
		return true
	}
	return false
}

// openCircuits returns every harness whose newest hook events are
// circuitTripCount failures with one error code, sorted by harness. It walks
// the events from the newest; a decision, a different code or a failure with
// no code ends that harness's streak.
func openCircuits(events []telemetry.Event) []routingCircuit {
	type streak struct {
		circuit routingCircuit
		n       int
		done    bool
	}
	byHarness := map[string]*streak{}
	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
		failure := telemetry.IsFailureOutcome(ev.Outcome)
		if ev.Harness == "" || (!failure && !isRoutedOutcome(ev.Outcome)) {
			continue
		}
		s := byHarness[ev.Harness]
		if s == nil {
			s = &streak{}
			byHarness[ev.Harness] = s
		}
		if s.done {
			continue
		}
		if !failure || ev.ErrorCode == "" || (s.n > 0 && ev.ErrorCode != s.circuit.Code) {
			s.done = true
			continue
		}
		if s.n == 0 {
			at, _ := time.Parse(time.RFC3339Nano, ev.Timestamp)
			s.circuit = routingCircuit{Harness: ev.Harness, Code: ev.ErrorCode, LastFailure: at}
		}
		s.n++
		s.done = s.n == circuitTripCount
	}
	var open []routingCircuit
	for _, s := range byHarness {
		if s.n == circuitTripCount {
			open = append(open, s.circuit)
		}
	}
	sort.Slice(open, func(i, j int) bool { return open[i].Harness < open[j].Harness })
	return open
}

// hookCircuit is the breaker of one hook call. The event log is read at most
// once, and only when the call fails or is about to route, so a shell-tool
// call that the runner answers on its own never pays for the read.
type hookCircuit struct {
	harness string
	read    bool
	open    bool
	circuit routingCircuit
}

// state reports whether this harness's circuit is open and not yet due for a
// probe. Any error or panic while reading reports it closed: the breaker can
// only hold routing back, never block a spawn.
func (b *hookCircuit) state() (routingCircuit, bool) {
	if b.read {
		return b.circuit, b.open
	}
	b.read = true
	defer func() {
		if recover() != nil {
			b.open = false
		}
	}()
	events, err := telemetry.ReadEventsTail(circuitTailBytes)
	if err != nil {
		return b.circuit, false
	}
	for _, c := range openCircuits(events) {
		if c.Harness == b.harness && time.Since(c.LastFailure) < circuitProbeAfter {
			b.circuit, b.open = c, true
		}
	}
	return b.circuit, b.open
}

// hold reports whether the open circuit holds this call back, and says so on
// stderr when it does.
func (b *hookCircuit) hold(correlationID string) bool {
	c, open := b.state()
	if open {
		fmt.Fprintf(os.Stderr, "downshift: correlation_id=%s %s\n", correlationID, c.line())
	}
	return open
}

// checkCircuits adds one failed doctor check per open circuit. It reports the
// trip condition alone: a circuit due for a probe is still open until a
// routed spawn closes it.
func checkCircuits(rep *doctorReport, events []telemetry.Event) {
	for _, c := range openCircuits(events) {
		rep.add(doctorCheck{ID: "circuit", Status: doctorFail, Detail: c.line(),
			Fix: fmt.Sprintf("fix what causes %s; the hook routes %s again %d minutes after the last failure, and the first routed spawn closes the circuit",
				c.Code, c.Harness, int(circuitProbeAfter.Minutes()))})
	}
}
