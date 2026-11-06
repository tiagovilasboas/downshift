// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"io"
	"strings"
)

// PrintReportMarkdown writes the routed report as Markdown: the same figures
// and wording as PrintReport, as headings and tables a chat can render.
func PrintReportMarkdown(w io.Writer, r Report) {
	PrintSegmentedReportMarkdown(w, r, []HarnessReport{{Report: r}})
}

// PrintSegmentedReportMarkdown is PrintReportMarkdown with the routed row and
// the rework per verdict split by harness when the window has more than one
// segment. whole carries what is not a per-harness figure (routing view,
// completion, excluded count). Routed figures are never summed across harnesses.
func PrintSegmentedReportMarkdown(w io.Writer, whole Report, segments []HarnessReport) {
	r := whole
	multi := len(segments) > 1
	fmt.Fprintf(w, "## Routed spawns (%s)\n\n", windowLabel(r.Days))
	printRoutedMarkdown(w, segments, multi)
	printVerdictsMarkdown(w, segments, multi)
	if len(r.Completion) > 0 {
		fmt.Fprint(w, "### Completion measurable, by harness\n\n")
		fmt.Fprintln(w, "| harness | completion |")
		fmt.Fprintln(w, "|---|---|")
		for _, c := range r.Completion {
			fmt.Fprintf(w, "| %s | %s |\n", c.Harness, c.Describe())
		}
		fmt.Fprintln(w)
	}
	if r.Excluded > 0 {
		fmt.Fprintf(w, "_%d spawn(s) left unrouted by a session pause, or logged as baseline by an older version, are not part of these figures._\n\n", r.Excluded)
	}
	fmt.Fprint(w, "_Billable tokens = input, output and cache write tokens of the same tasks; cache reads are left out. "+
		"The figures count tokens that ran, per harness, never summed. Prompt-free._\n\n")
}

func windowLabel(days int) string {
	if days <= 0 {
		return "all time"
	}
	return fmt.Sprintf("last %d days", days)
}

// codeCell wraps a policy key in backticks; plain text stays plain.
func codeCell(s string, isKey bool) string {
	if !isKey {
		return s
	}
	return "`" + s + "`"
}

// printRoutedMarkdown writes the routed table: one row per harness when multi,
// else the single group row.
func printRoutedMarkdown(w io.Writer, segments []HarnessReport, multi bool) {
	var rows []string
	for _, seg := range segments {
		g := seg.Report.Routed
		if g.Spawns == 0 {
			continue
		}
		tokens := "-"
		if g.Completed > 0 {
			tokens = formatTokens(int64(g.BillablePerTask + 0.5))
		}
		name := "routed"
		if multi {
			name = seg.Harness
		}
		rows = append(rows, fmt.Sprintf("| %s | %d | %d | %d | %s | %s |", name, g.Spawns, g.Ended, g.Completed, tokens, reworkCell(g.Ended, g.ReworkRate)))
	}
	if len(rows) == 0 {
		fmt.Fprint(w, "No routed spawn in this window.\n\n")
		return
	}
	first := "group"
	if multi {
		first = "harness"
	}
	fmt.Fprintf(w, "| %s | spawns | ended | completed | billable/completed | rework |\n", first)
	fmt.Fprintln(w, "|---|--:|--:|--:|--:|--:|")
	fmt.Fprintln(w, strings.Join(rows, "\n")+"\n")
}

// printVerdictsMarkdown writes the rework per verdict, under one heading per
// harness when multi.
func printVerdictsMarkdown(w io.Writer, segments []HarnessReport, multi bool) {
	any := false
	for _, seg := range segments {
		if len(seg.Report.ByVerdict) == 0 {
			continue
		}
		if !any {
			fmt.Fprint(w, "### Rework by routing verdict\n\n")
			any = true
		}
		if multi {
			fmt.Fprintf(w, "**%s**\n\n", seg.Harness)
		}
		fmt.Fprintln(w, "| verdict | spawns | ended | completed | rework | policy key that turns it off |")
		fmt.Fprintln(w, "|---|--:|--:|--:|--:|---|")
		for _, v := range seg.Report.ByVerdict {
			fmt.Fprintf(w, "| %s | %d | %d | %d | %s | %s |\n", v.Verdict, v.Spawns, v.Ended, v.Completed,
				reworkCell(v.Ended, v.ReworkRate), codeCell(turnOffCell(v), v.TurnOff != ""))
		}
		fmt.Fprintln(w)
	}
	if any {
		fmt.Fprint(w, "Downshift does not change the policy: the key is in `policy.json`.\n\n")
	}
}
