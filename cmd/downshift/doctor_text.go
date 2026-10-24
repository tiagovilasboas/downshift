// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// defaultTextWidth is the line width when no terminal is attached (a pipe, a
// file, a test buffer).
const defaultTextWidth = 80

// minFitWidth keeps a truncated text readable even in a very narrow window.
const minFitWidth = 20

// writeText prints the checklist as text. Paths under the home directory print
// as ~, and a detail or fix line never runs past the terminal width: the middle
// is cut with an ellipsis. quiet keeps only the failed and warning checks; with
// none, it says how many checks passed. The JSON report is not affected.
func (r *doctorReport) writeText(w io.Writer, verbose, quiet bool) {
	fmt.Fprintf(w, "downshift doctor\n")
	section := "\x00"
	for _, c := range r.visibleChecks(quiet) {
		if c.Harness != section {
			section = c.Harness
			if section != "" {
				fmt.Fprintf(w, "\n%s\n", section)
			} else {
				fmt.Fprintln(w)
			}
		}
		mark := map[string]string{doctorOK: "✓", doctorWarn: "!", doctorFail: "✗"}[c.Status]
		lead := fmt.Sprintf("  %s %-15s ", mark, c.ID)
		fmt.Fprintf(w, "%s%s\n", lead, r.fit(c.Detail, lead))
		if c.Fix != "" && c.Status != doctorOK {
			for i, line := range strings.Split(c.Fix, "\n") {
				prefix := "      fix: "
				if i > 0 {
					prefix = "           "
				}
				fmt.Fprintf(w, "%s%s\n", prefix, r.fit(line, prefix))
			}
		}
	}
	if !quiet && len(r.NotDetected) > 0 {
		fmt.Fprintf(w, "\nnot detected (no hook file, skipped): %s\n", strings.Join(r.NotDetected, ", "))
		if !r.anyHarnessChecked() {
			fmt.Fprintf(w, "next: downshift setup --dry-run%s\n", pluginInstallHint())
		}
	}
	if !quiet && verbose && len(r.EffortUnknownModels) > 0 {
		fmt.Fprintf(w, "\nmodels without a verified efforts list:\n")
		for _, m := range r.EffortUnknownModels {
			fmt.Fprintf(w, "  %s\n", m)
		}
	}
	switch {
	case r.Failed > 0:
		fmt.Fprintf(w, "\n%d failed, %d warnings\n", r.Failed, r.Warnings)
	case r.Warnings > 0:
		fmt.Fprintf(w, "\nno failures, %d warnings\n", r.Warnings)
	case quiet:
		fmt.Fprintf(w, "\nall %d checks passed\n", len(r.Checks))
	default:
		fmt.Fprintf(w, "\nall good\n")
	}
}

// visibleChecks is every check, or with quiet only the failed and warning ones.
func (r *doctorReport) visibleChecks(quiet bool) []doctorCheck {
	if !quiet {
		return r.Checks
	}
	var out []doctorCheck
	for _, c := range r.Checks {
		if c.Status != doctorOK {
			out = append(out, c)
		}
	}
	return out
}

// fit makes text fit on one line after lead: home-relative first, then the
// longest paths are cut in the middle, and only then the whole text.
func (r *doctorReport) fit(text, lead string) string {
	width := r.width
	if width <= 0 {
		width = defaultTextWidth
	}
	avail := max(width-utf8.RuneCountInString(lead), minFitWidth)
	return fitWords(r.tidy(text), avail)
}

// minPathKeep is the fewest runes a path keeps when its middle is cut.
const minPathKeep = 12

// fitWords cuts the middle of the longest path-like words of s, one at a time,
// until s is at most avail runes; prose words stay whole while a path can give.
func fitWords(s string, avail int) string {
	n := utf8.RuneCountInString
	if n(s) <= avail {
		return s
	}
	words := strings.Split(s, " ")
	for excess := n(s) - avail; excess > 0; excess = n(strings.Join(words, " ")) - avail {
		idx := -1
		for i, w := range words {
			if strings.Contains(w, "/") && n(w) > minPathKeep && (idx < 0 || n(w) > n(words[idx])) {
				idx = i
			}
		}
		if idx < 0 {
			break
		}
		words[idx] = middleTruncate(words[idx], max(n(words[idx])-excess, minPathKeep))
	}
	return middleTruncate(strings.Join(words, " "), avail)
}

// tidy writes the home directory as ~ wherever a path begins with it.
func (r *doctorReport) tidy(s string) string {
	if r.home == "" || r.home == "/" {
		return s
	}
	return strings.ReplaceAll(s, r.home+"/", "~/")
}

// middleTruncate keeps the head and the tail of s and joins them with an
// ellipsis when s is longer than max runes. The tail keeps the file name.
func middleTruncate(s string, max int) string {
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	keep := max - 1
	head := (keep + 1) / 2
	tail := keep - head
	return string(rs[:head]) + "…" + string(rs[len(rs)-tail:])
}

// terminalWidth is the column count of w when it is a terminal, else the default.
func terminalWidth(w io.Writer) int {
	if f, ok := w.(*os.File); ok {
		if c := ttyColumns(f); c > 0 {
			return c
		}
	}
	return defaultTextWidth
}
