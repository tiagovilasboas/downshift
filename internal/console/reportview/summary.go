// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package reportview

import (
	"fmt"
	"strings"
)

const labelCells = 22

// summaryHeader is the title line of the summary tab.
func (m Model) summaryHeader() string {
	h := m.data.Header
	title := h.Label
	if title == "" {
		title = "Downshift"
	}
	window := "all time"
	if h.Days > 0 {
		window = fmt.Sprintf("last %d days", h.Days)
	}
	n := len(m.data.Harnesses)
	unit := "harnesses"
	if n == 1 {
		unit = "harness"
	}
	return fmt.Sprintf("%s · %s · %d %s", title, window, n, unit)
}

// summaryLines draws one block per harness: what the Compactor kept out of
// the context, then how the subagents ran. Nothing is summed across harnesses,
// and the Compactor and routing figures are never added.
func (m Model) summaryLines() []string {
	if len(m.data.Harnesses) == 0 {
		return []string{m.compose(m.width, seg{"No harness activity in this window.", m.th.Muted})}
	}
	bars := barsFit(m.width)
	var out []string
	for i, r := range m.data.Harnesses {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, m.rule(r.Harness))
		out = append(out, m.compactorLines(r, bars)...)
		out = append(out, m.routingLines(r, bars)...)
		if r.Quota != "" {
			out = append(out, m.textRow("Quota used", r.Quota, "5h · week"))
		}
	}
	return append(out, "",
		m.compose(m.width, seg{"* estimate: saved bytes / 4, net of what retrieve read back.", m.th.Muted}),
		m.compose(m.width, seg{"Per harness, never summed. Tokens on a cheaper model still ran:", m.th.Muted}),
		m.compose(m.width, seg{"they are never added to tokens saved.", m.th.Muted}))
}

// compactorLines is the Compactor block of one harness.
func (m Model) compactorLines(r Row, bars bool) []string {
	if r.Commands == 0 {
		return []string{m.textRow("Compactor", "no runs in this window", "")}
	}
	out := []string{
		m.section("COMPACTOR"),
		m.textRow("Tokens kept out", "~"+Human(r.TokensSaved)+"*", ""),
		m.metric(bars, "Output cut", r.RawBytes > 0, r.CutPct, pct(r.CutPct),
			Bytes(r.SavedBytes)+" of "+Bytes(r.RawBytes), m.th.OK),
		m.textRow("Runs", fmt.Sprintf("%d · %d compacted (%s)", r.Commands, r.Compacted, pct(share(r.Compacted, r.Commands))),
			fmt.Sprintf("%d via hook · %d direct", r.HookRuns, r.DirectRuns)),
	}
	if r.RetrievedBytes > 0 {
		out = append(out, m.textRow("Read back", Bytes(r.RetrievedBytes), "with retrieve, already subtracted"))
	}
	if len(r.Families) > 0 {
		parts := make([]string, 0, len(r.Families))
		for _, f := range r.Families {
			parts = append(parts, fmt.Sprintf("%s %s (%s)", f.Name, Human(f.AvoidedTokens), pct(f.CutPct)))
		}
		out = append(out, m.textRow("Top families", strings.Join(parts, " · "), ""))
	}
	return out
}

// routingLines is the routing block of one harness, or one line when no
// subagent was measured.
func (m Model) routingLines(r Row, bars bool) []string {
	if !r.RoutingObserved && !r.PremiumObserved && r.Ended == 0 && r.Spawns == 0 {
		return []string{m.textRow("Routing", "no subagent spawns measured", "")}
	}
	out := []string{
		m.section("ROUTING"),
		m.textRow("Subagents", fmt.Sprintf("%d routed", r.Spawns),
			fmt.Sprintf("model changed %d · effort changed %d · held %d", r.ModelSwitched, r.EffortChanged, r.Held)),
		m.metric(bars, "On a cheaper model", r.RoutingObserved, r.RoutedSharePct, pct(r.RoutedSharePct),
			"of "+Human(r.Billable)+" subagent tokens", m.th.Info),
	}
	if r.RoutingObserved && r.RewrittenSpawns > 0 {
		out = append(out, m.textRow("  moved by Downshift", Human(r.RewrittenBillable)+" tokens", fmt.Sprintf("%d spawns", r.RewrittenSpawns)))
	}
	out = append(out, m.metric(bars, "Premium preserved", r.PremiumObserved, r.PremiumPct, pct(r.PremiumPct),
		fmt.Sprintf("%d of %d tasks below frontier", r.NonFrontier, r.TierKnown), m.th.OK))
	if r.Ended == 0 {
		return append(out, m.textRow("Rework", "unobserved", ""))
	}
	return append(out, m.textRow("Rework", pct(r.ReworkPct), fmt.Sprintf("%d of %d ended", r.Reworked, r.Ended)))
}

// share is part of whole in percent; 0 when whole is 0.
func share(part, whole int64) float64 {
	if whole <= 0 {
		return 0
	}
	return 100 * float64(part) / float64(whole)
}
