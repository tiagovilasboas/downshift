// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// printReportMarkdown writes the default report: the sections of the text
// report, from the same reportOut the --json output encodes, as headings and
// tables a chat renders. Per harness, never summed; no money.
func printReportMarkdown(w io.Writer, out reportOut, scope string) {
	fmt.Fprintf(w, "# Downshift report (%s)\n\n", scope)

	fmt.Fprint(w, "## Compaction savings per harness\n\nTokens kept out of the context by the Compactor.\n\n")
	fmt.Fprintln(w, "| harness | tokens avoided |")
	fmt.Fprintln(w, "|---|--:|")
	have := map[string]bool{}
	for _, r := range out.TokensByHarness {
		have[r.Harness] = true
		if r.CompactorAvoided > 0 {
			fmt.Fprintf(w, "| %s | %s |\n", r.Harness, telemetry.ShortTokens(r.CompactorAvoided))
		}
	}
	fmt.Fprintf(w, "\n%s\n", noEventsLine(have))
	fmt.Fprintln(w)
	printCompactionDetailMarkdown(w, out.CompactionDetail)
	telemetry.PrintRoutingSectionMarkdown(w, out.Report.Routing)
	printEfficiencyMarkdown(w, out.Efficiency)

	telemetry.PrintSegmentedReportMarkdown(w, out.Report, out.ByHarness)

	fmt.Fprint(w, "## Why spawns changed\n\n")
	if out.WhySpawnsChanged.Decisions == 0 {
		fmt.Fprint(w, "No routing events in this window.\n\n")
	} else {
		fmt.Fprintln(w, "| harness | spawns | by verdict | top reasons |")
		fmt.Fprintln(w, "|---|--:|---|---|")
		for _, h := range out.WhySpawnsChanged.Harnesses {
			fmt.Fprintf(w, "| %s | %d | %s | %s |\n", h.Harness, h.Decisions, joinCounts(h.ByVerdict), joinReasons(h.TopReasons))
		}
		fmt.Fprintln(w)
	}

	fmt.Fprint(w, "## Main session mechanical work\n\nRead, search and shell calls a small-tier subagent could take.\n\n")
	fmt.Fprintln(w, "| harness | calls |")
	fmt.Fprintln(w, "|---|---|")
	for _, r := range out.MainSessionWork {
		fmt.Fprintf(w, "| %s | %s |\n", r.Harness, mainWorkText(r))
	}

	fmt.Fprint(w, "\n## Quota used per harness\n\n")
	fmt.Fprintln(w, "| harness | 5h · week |")
	fmt.Fprintln(w, "|---|---|")
	for _, r := range out.TokensByHarness {
		fmt.Fprintf(w, "| %s | %s |\n", r.Harness, quotaText(r.Quota))
	}
	fmt.Fprintf(w, "\n## Planner and workers\n\n%s\n", accounting.AdoptionLine(out.Adoption))
}

// noEventsLine names the known harnesses with nothing in the window.
func noEventsLine(have map[string]bool) string {
	rest := harnessesWithoutEvents(have)
	if len(rest) == 0 {
		return ""
	}
	return "_No events: " + strings.Join(rest, ", ") + "._\n"
}
