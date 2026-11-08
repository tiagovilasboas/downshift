// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// ── stats ─────────────────────────────────────────────────────────────────────

type stats struct {
	total, down, up, ok int
	// held counts classified shifts the hook left unchanged.
	held int
	// routing is the shared routing-tokens view (telemetry.RoutingTokens), per
	// harness and never summed: a live widget shows tokens, never dollars.
	routing   telemetry.RoutingView
	harnesses []string // all distinct harnesses active today, most-recent first
}

func compute(all []event) (stats, []event) {
	var st stats
	var recent []event
	var window []telemetry.Event
	seen := map[string]bool{}
	for _, e := range all {
		if !e.isRecent() {
			continue
		}
		window = append(window, e.Event)
		if telemetry.IsCostOnlyOutcome(e.Outcome) {
			// Usage records carry real cost (summed by Summarize below);
			// baseline records are unrouted spawns. Neither is
			// a routing decision.
			continue
		}
		st.total++
		applied := telemetry.AppliedRewrite(e.Outcome, e.Corrections)
		switch {
		case e.Verdict == "DOWNSHIFT" && applied:
			st.down++
		case e.Verdict == "UPSHIFT" && applied:
			st.up++
		case e.Verdict == "DOWNSHIFT" || e.Verdict == "UPSHIFT":
			st.held++
			e.Verdict = "HELD" // displayed as unchanged, never as a switch
		case e.Verdict == "OK":
			st.ok++
		}
		if e.Harness != "" && !seen[e.Harness] {
			seen[e.Harness] = true
			st.harnesses = append(st.harnesses, e.Harness)
		}
		recent = append(recent, e)
	}
	st.routing = telemetry.RoutingTokens(window)
	if len(recent) > maxRecent {
		recent = recent[len(recent)-maxRecent:]
	}
	return st, recent
}
