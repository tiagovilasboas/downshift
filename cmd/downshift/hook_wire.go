// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tiagovilasboas/downshift/internal/adapters/opencode"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// hookWire is how one harness receives the routed decision: the only
// per-harness step of the shared runner, chosen by the registry entry in
// harnesses.go. The runner itself holds no harness branch.
type hookWire interface {
	// failOpen writes the "allow unchanged" response. cause is the failure
	// code on runner errors (recorded by wires that log fail-open),
	// wireFailOpenNoRoute in the control group, when paused and under
	// DOWNSHIFT_NO_ROUTE, or wireFailOpenCircuit while the routing circuit is
	// open (same response, nothing recorded). Never called more than once per run.
	failOpen(cause, correlationID string)
	// emit writes the routed response and returns the hook's exit code. A
	// non-zero code denies the spawn: every supported harness reads exit 2
	// as "block".
	emit(out any, correlationID string) (int, error)
}

// wireFailOpenNoRoute is the failOpen cause for "no route was attempted"
// (control group, per-session pause, DOWNSHIFT_NO_ROUTE): not a failure.
const wireFailOpenNoRoute = "no_route"

// rewriteWire serves harnesses with an in-place rewrite channel: the adapter
// output is one JSON line on stdout and the exit code is always 0. The func
// value prints that harness's fail-open envelope.
type rewriteWire func()

func (w rewriteWire) failOpen(_, _ string) { w() }

func (w rewriteWire) emit(out any, _ string) (int, error) {
	encoded, err := json.Marshal(out)
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(os.Stdout, "%s\n", encoded)
	return 0, nil
}

// policyOutput is the adapter output of a policy-mode harness, one whose hook
// contract is exit-code only (no updated_input): allow unchanged or deny.
type policyOutput interface {
	// Denied reports whether the spawn is blocked and, if so, the message
	// relayed to the agent on stderr.
	Denied() (blocked bool, message string)
}

// policyWire serves policy-mode harnesses: exit 0 allows the spawn unchanged
// with nothing on stdout; exit 2 blocks it and the stderr line is what the
// harness relays to the LLM. Fail-open is silence, so a router error can
// never be read as a block.
type policyWire struct{}

func (policyWire) failOpen(_, _ string) {}

func (policyWire) emit(out any, correlationID string) (int, error) {
	o, ok := out.(policyOutput)
	if !ok {
		return 0, fmt.Errorf("policy wire: %T does not implement Denied", out)
	}
	if blocked, message := o.Denied(); blocked {
		fmt.Fprintf(os.Stderr, "downshift: correlation_id=%s %s\n", correlationID, message)
		return 2, nil
	}
	return 0, nil
}

// --- Harness-specific fail-open responses ---
// Each harness has a different envelope for "allow unchanged". These are the
// minimal valid JSON outputs that let the tool call proceed unmodified.

var printAllow = rewriteWire(func() {
	// No permissionDecision: an explicit allow would override the user's ask
	// rules. An empty object leaves the harness's own permission flow intact.
	fmt.Println(`{}`)
})

var printCursorAllow = rewriteWire(func() {
	fmt.Println(`{"permission":"allow"}`)
})

// openCodeWire serves the OpenCode route contract: one JSON object on stdout
// and the exit code always 0. Unlike the rewrite wires, fail-open is a valid
// allow answer the plugin caches: under no-route it answers reasonCode
// "no_route" (the decision was recorded as a baseline), and on any internal
// error it answers reasonCode "fail_open" and records the fail-open outcome
// so the rate is visible in stats. The adapter's own invalid-request answer
// ("fail_open" from Handle) is recorded here too, once.
type openCodeWire struct{}

func (openCodeWire) failOpen(cause, correlationID string) {
	if cause == wireFailOpenNoRoute {
		fmt.Printf("{\"v\":1,\"action\":\"allow\",\"reasonCode\":%q,\"ttlMs\":15000}\n", wireFailOpenNoRoute)
		return
	}
	if cause != wireFailOpenCircuit {
		telemetry.Record(telemetry.FailOpen(correlationID, buildVersion, "opencode", cause))
	}
	fmt.Printf("{\"v\":1,\"action\":\"allow\",\"reasonCode\":%q}\n", opencode.ReasonFailOpen)
}

func (openCodeWire) emit(out any, correlationID string) (int, error) {
	resp, ok := out.(opencode.Response)
	if !ok {
		return 0, fmt.Errorf("opencode wire: %T is not a route response", out)
	}
	if resp.ReasonCode == opencode.ReasonFailOpen {
		telemetry.Record(telemetry.FailOpen(correlationID, buildVersion, "opencode", "INVALID_REQUEST"))
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(os.Stdout, "%s\n", encoded)
	return 0, nil
}
