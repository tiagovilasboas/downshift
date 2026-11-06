// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"io"
)

// PrintRouting writes the routing tokens, one block per harness and never a
// total across harnesses. The below-session row comes first. Billable is
// input, output and cache write; cache read is its own figure.
func PrintRouting(w io.Writer, v RoutingView) {
	fmt.Fprintln(w, "Routing tokens, by harness (billable = input + output + cache write)")
	order := v.HarnessOrder()
	if len(order) == 0 {
		fmt.Fprintln(w, "  tokens not observed in this window")
		return
	}
	for _, h := range order {
		r := v.ByHarness[h]
		fmt.Fprintf(w, "  %s\n", h)
		for _, line := range r.Rows() {
			fmt.Fprintf(w, "    %s\n", line)
		}
	}
}

// Rows are the lines of one harness, below-session first: the session
// baseline, the requested-model fallback (a different baseline, never added),
// the unobserved spawns and the cache reads.
func (r RoutingRow) Rows() []string {
	var out []string
	if r.Observed() {
		out = append(out,
			fmt.Sprintf("ran below the session model  %s of %s billable (%s%d of %s)", formatTokens(r.BelowBillable), formatTokens(r.Billable), r.shareText(), r.BelowSpawns, spawnsText(r.Spawns)))
		out = append(out, r.RewriteRow())
	}
	if r.FallbackObserved() {
		out = append(out,
			fmt.Sprintf("below the requested model  %s of %s billable (%s, no session model; a different baseline)", formatTokens(r.FallbackBelowBillable), formatTokens(r.FallbackBillable), spawnsText(r.FallbackSpawns)))
	}
	if r.UnobservedSpawns > 0 {
		out = append(out, fmt.Sprintf("unobserved  %s (no model could be ranked)", spawnsText(r.UnobservedSpawns)))
	}
	if r.CacheRead > 0 {
		out = append(out, fmt.Sprintf("cache read  %s (apart from billable)", formatTokens(r.CacheRead)))
	}
	if len(out) == 0 {
		out = append(out, "unobserved")
	}
	return out
}

// RewriteRow is the part of the below-session figure that a Downshift rewrite
// caused: "of which by a Downshift rewrite  51 spawns (1.2M tokens)". The rest
// ran below because the caller or the session had already chosen it.
func (r RoutingRow) RewriteRow() string {
	return fmt.Sprintf("of which by a Downshift rewrite  %s (%s tokens)", spawnsText(r.RewrittenSpawns), formatTokens(r.RewrittenBillable))
}

// shareText is the routed share as "89%; ", empty with no billable tokens.
func (r RoutingRow) shareText() string {
	if r.Billable <= 0 {
		return ""
	}
	return fmt.Sprintf("%.0f%%; ", r.BelowShare())
}
