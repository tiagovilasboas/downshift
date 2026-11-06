// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"

	"github.com/tiagovilasboas/downshift/internal/accounting"
)

// efficiencyPointer is the one line that sends readers to the Compactor
// section: its tokens are printed there once, never repeated here.
const efficiencyPointer = "Compactor tokens: see the section above."

// premiumText is "75% (3 of 4 completed tasks)", or "unobserved" when no
// completed task has a known tier (never 0%). Tasks left out for an unknown
// tier are stated.
func premiumText(h accounting.HarnessEfficiency) string {
	if !h.PremiumObserved {
		s := "unobserved"
		if h.TierUnknown > 0 {
			s += fmt.Sprintf(" (%d completed with unknown tier)", h.TierUnknown)
		}
		return s
	}
	s := fmt.Sprintf("%.0f%% (%d of %d completed tasks ran below frontier)", h.PremiumPreservedPct, h.NonFrontier, h.TierKnown)
	if h.TierUnknown > 0 {
		s += fmt.Sprintf("; %d with unknown tier left out", h.TierUnknown)
	}
	return s
}

func tierCell(c accounting.TierCount) string { return fmt.Sprintf("%d (%.0f%%)", c.Count, c.Pct) }

func reworkText(h accounting.HarnessEfficiency) string {
	if h.Ended == 0 {
		return "unobserved (no spawn ended)"
	}
	return fmt.Sprintf("%.0f%% (%d of %d ended)", h.ReworkRatio*100, h.Reworked, h.Ended)
}

func completedText(h accounting.HarnessEfficiency) string {
	return fmt.Sprintf("%d of %d ended", h.Completed, h.Ended)
}

// printEfficiencyMarkdown writes the Efficiency section: per harness, never
// summed, counts and shares only.
func printEfficiencyMarkdown(w io.Writer, e accounting.Efficiency) {
	fmt.Fprint(w, "## Efficiency per harness\n\nMore work done, less premium intelligence consumed. ")
	fmt.Fprintf(w, "%s\n\n", efficiencyPointer)
	if len(e.Harnesses) == 0 {
		fmt.Fprint(w, "No routed spawns in this window.\n\n")
		return
	}
	fmt.Fprintln(w, "| harness | premium preserved | completed | rework | spawns |")
	fmt.Fprintln(w, "|---|---|---|---|--:|")
	for _, h := range e.Harnesses {
		fmt.Fprintf(w, "| %s | %s | %s | %s | %d |\n", h.Harness, premiumText(h), completedText(h), reworkText(h), h.Spawns)
	}
	fmt.Fprint(w, "\n**Spawns by tier they ran on**\n\n| harness | small | mid | frontier | unknown |\n|---|--:|--:|--:|--:|\n")
	for _, h := range e.Harnesses {
		s := h.SpawnsByTier
		fmt.Fprintf(w, "| %s | %s | %s | %s | %s |\n", h.Harness, tierCell(s.Small), tierCell(s.Mid), tierCell(s.Frontier), tierCell(s.Unknown))
	}
	fmt.Fprintln(w)
}

// printEfficiencyText is the same section for the plain-text report.
func printEfficiencyText(w io.Writer, e accounting.Efficiency) {
	fmt.Fprintf(w, "\nEFFICIENCY PER HARNESS (%s)\n", e.Scope.Label())
	fmt.Fprintf(w, "   More work done, less premium intelligence consumed. %s\n", efficiencyPointer)
	if len(e.Harnesses) == 0 {
		fmt.Fprintln(w, "   No routed spawns in this window.")
		return
	}
	for _, h := range e.Harnesses {
		s := h.SpawnsByTier
		fmt.Fprintf(w, "   %s: %d spawns\n", h.Harness, h.Spawns)
		fmt.Fprintf(w, "      premium preserved: %s\n", premiumText(h))
		fmt.Fprintf(w, "      completed: %s · rework: %s\n", completedText(h), reworkText(h))
		fmt.Fprintf(w, "      by tier: small %s · mid %s · frontier %s · unknown %s\n",
			tierCell(s.Small), tierCell(s.Mid), tierCell(s.Frontier), tierCell(s.Unknown))
	}
}
