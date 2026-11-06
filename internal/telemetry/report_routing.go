// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"io"
)

// PrintRoutingSectionMarkdown writes the top-level routing section: per
// harness, how many subagent tokens ran on a model lighter than the session's
// and how many of those a Downshift rewrite moved. Routing does not make
// tokens fewer; it moves them to a lighter model and spares the top model's
// quota. Harnesses are never summed, tokens only, no money.
func PrintRoutingSectionMarkdown(w io.Writer, v RoutingView) {
	fmt.Fprint(w, "## Routing: tokens on a lighter model, per harness\n\n")
	order := v.HarnessOrder()
	if len(order) == 0 {
		fmt.Fprint(w, "No subagent tokens observed in this window.\n\n")
		return
	}
	fmt.Fprint(w, "Same work, run on a model lighter than the session's: it spares the top model's quota. "+
		"Tokens are not fewer; they ran lighter. Never added to compaction tokens.\n\n")
	fmt.Fprintln(w, "| harness | subagent tokens | on a lighter model | share | moved by Downshift |")
	fmt.Fprintln(w, "|---|--:|--:|--:|--:|")
	for _, h := range order {
		fmt.Fprintf(w, "| %s | %s |\n", h, v.ByHarness[h].routingCells())
	}
	fmt.Fprintln(w)
	notes := false
	for _, h := range order {
		for _, note := range v.ByHarness[h].routingNotes() {
			fmt.Fprintf(w, "- **%s**: %s\n", h, note)
			notes = true
		}
	}
	if notes {
		fmt.Fprintln(w)
	}
	fmt.Fprint(w, "_Tokens = input, output and cache write of the same tasks; cache reads are left out. Prompt-free._\n\n")
}

// routingCells are the four table cells after the harness name. A harness with
// no session-placed spawn reads "unobserved", never 0.
func (r RoutingRow) routingCells() string {
	if !r.Observed() {
		return "unobserved | - | - | -"
	}
	moved := "-"
	if r.RewrittenSpawns > 0 {
		moved = fmt.Sprintf("%s (%s)", ShortTokens(r.RewrittenBillable), agentsText(r.RewrittenSpawns))
	}
	share := "-"
	if r.Billable > 0 {
		share = fmt.Sprintf("%.0f%%", r.BelowShare())
	}
	return fmt.Sprintf("%s | %s (%d of %s) | %s | %s",
		ShortTokens(r.Billable), ShortTokens(r.BelowBillable), r.BelowSpawns, agentsText(r.Spawns), share, moved)
}

// routingNotes are the lines under the table that apply to this harness: the
// requested-model fallback (a different baseline), unobserved spawns, cache read.
func (r RoutingRow) routingNotes() []string {
	var out []string
	if r.FallbackObserved() {
		out = append(out, fmt.Sprintf("%s had no session model: %s of %s ran on a lighter model than the one requested",
			agentsText(r.FallbackSpawns), ShortTokens(r.FallbackBelowBillable), ShortTokens(r.FallbackBillable)))
	}
	if r.UnobservedSpawns > 0 {
		out = append(out, fmt.Sprintf("%s unobserved (no model could be ranked)", agentsText(r.UnobservedSpawns)))
	}
	if r.CacheRead > 0 {
		out = append(out, fmt.Sprintf("cache read %s, apart from the tokens above", ShortTokens(r.CacheRead)))
	}
	return out
}

// agentsText is "1 agent" or "3 agents".
func agentsText(n int) string {
	if n == 1 {
		return "1 agent"
	}
	return fmt.Sprintf("%d agents", n)
}
