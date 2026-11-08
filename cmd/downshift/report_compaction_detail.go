// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"

	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// detailFamilies is how many families a harness shows; --json carries all.
const detailFamilies = 8

// notCompactedLabel words a passthrough reason code for people.
var notCompactedLabel = map[string]string{
	"unrecognized_output": "output shape not recognized",
	"small_output":        "output already short",
	"nonzero_exit":        "command failed, output kept whole",
	"refused":             "not allowlisted, did not run",
	"not_recorded":        "reason not recorded",
}

// skippedLabel words a skip reason code for people.
var skippedLabel = map[string]string{
	"metachar":        "pipe, list, quote or redirect",
	"too_long":        "command too long",
	"already_wrapped": "already wrapped",
	"not_allowlisted": "family not allowlisted",
	"denied":          "a deny or ask rule covers it",
	"disabled":        "Compactor off or nested",
	"abs_path":        "binary given by absolute path",
	"empty":           "empty command",
}

// detailLabel words a passthrough_detail code for people.
var detailLabel = map[string]string{
	"danger_word":      "failure or secret word in the output",
	"ansi":             "terminal escape in the output",
	"cr":               "carriage return (progress redraw)",
	"no_final_newline": "output has no final newline",
	"unknown_shape":    "no known line shape",
	"no_compressor":    "family has no compressor",
	"compressor_panic": "compressor failed, output kept",
}

// metacharLabel words a metachar class for people.
var metacharLabel = map[string]string{
	"semicolon": "; command list", "and": "&& chain", "or": "|| chain", "pipe": "| pipe",
	"background": "& background", "subst": "$ substitution", "backtick": "backtick substitution",
	"redirect": "redirect", "quote": "quote", "newline": "newline", "glob": "glob or brace",
	"backslash": "backslash", "paren": "parenthesis", "other": "other character",
}

func reasonLabel(labels map[string]string, code string) string {
	if l, ok := labels[code]; ok {
		return l
	}
	return code
}

// printCompactionDetailMarkdown writes, per harness, the families, why runs
// were not compacted and why the hook left commands unwrapped.
func printCompactionDetailMarkdown(w io.Writer, d accounting.CompactionDetail) {
	for _, h := range d.Harnesses {
		fmt.Fprintf(w, "### %s: %d commands, %d compacted\n\n", h.Harness, h.Commands, h.Compacted)
		if len(h.Families) > 0 {
			fmt.Fprint(w, "**By family** (largest output first; tokens are bytes / 4, an estimate)\n\n")
			fmt.Fprintln(w, "| family | commands | compacted | output tokens | tokens avoided |")
			fmt.Fprintln(w, "|---|--:|--:|--:|--:|")
			for _, f := range firstFamilies(h.Families) {
				fmt.Fprintf(w, "| %s | %d | %d | %s | %s |\n", f.Family, f.Commands, f.Compacted,
					telemetry.ShortTokens(f.RawTokens), telemetry.ShortTokens(f.AvoidedTokens))
			}
			if more := len(h.Families) - detailFamilies; more > 0 {
				fmt.Fprintf(w, "\n_%d more families in `--json`._\n", more)
			}
			fmt.Fprintln(w)
		}
		reasonTableMarkdown(w, "**Not compacted**", "commands", h.NotCompacted, notCompactedLabel)
		reasonTableMarkdown(w, "**Left unwrapped by the hook**", "calls", h.Skipped, skippedLabel)
		reasonTableMarkdown(w, "**Not compacted, detail** (output shape not recognized)", "commands", h.PassthroughDetails, detailLabel)
		reasonTableMarkdown(w, "**Left unwrapped, metacharacter**", "calls", h.SkipMetachars, metacharLabel)
	}
}

func reasonTableMarkdown(w io.Writer, title, unit string, rs []accounting.ReasonCount, labels map[string]string) {
	if len(rs) == 0 {
		return
	}
	fmt.Fprintf(w, "%s\n\n| reason | %s |\n|---|--:|\n", title, unit)
	for _, r := range rs {
		fmt.Fprintf(w, "| %s | %d |\n", reasonLabel(labels, r.Reason), r.Count)
	}
	fmt.Fprintln(w)
}

// printCompactionDetailText is the same detail for the plain-text report.
func printCompactionDetailText(w io.Writer, d accounting.CompactionDetail) {
	if len(d.Harnesses) == 0 {
		return
	}
	fmt.Fprintln(w, "\nCOMPACTION DETAIL PER HARNESS (tokens are bytes / 4, an estimate)")
	for _, h := range d.Harnesses {
		fmt.Fprintf(w, "   %s: %d commands, %d compacted\n", h.Harness, h.Commands, h.Compacted)
		if len(h.Families) > 0 {
			fmt.Fprintf(w, "      %-18s %8s %9s %13s %14s\n", "family", "commands", "compacted", "output tokens", "tokens avoided")
			for _, f := range firstFamilies(h.Families) {
				fmt.Fprintf(w, "      %-18s %8d %9d %13s %14s\n", f.Family, f.Commands, f.Compacted,
					telemetry.ShortTokens(f.RawTokens), telemetry.ShortTokens(f.AvoidedTokens))
			}
			if more := len(h.Families) - detailFamilies; more > 0 {
				fmt.Fprintf(w, "      %d more families in --json\n", more)
			}
		}
		reasonLinesText(w, "not compacted", h.NotCompacted, notCompactedLabel)
		reasonLinesText(w, "left unwrapped by the hook", h.Skipped, skippedLabel)
		reasonLinesText(w, "not compacted, detail", h.PassthroughDetails, detailLabel)
		reasonLinesText(w, "left unwrapped, metacharacter", h.SkipMetachars, metacharLabel)
	}
}

func reasonLinesText(w io.Writer, title string, rs []accounting.ReasonCount, labels map[string]string) {
	if len(rs) == 0 {
		return
	}
	fmt.Fprintf(w, "      %s:\n", title)
	for _, r := range rs {
		fmt.Fprintf(w, "         %-34s %d\n", reasonLabel(labels, r.Reason), r.Count)
	}
}

func firstFamilies(fs []accounting.FamilyCompaction) []accounting.FamilyCompaction {
	if len(fs) > detailFamilies {
		return fs[:detailFamilies]
	}
	return fs
}
