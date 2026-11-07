// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

// Pytest failed runs, read from the default dot progress and from -v. A run is
// split at its first section that is kept whole: "=== FAILURES ===",
// "=== ERRORS ===", "=== warnings summary ===" or "=== short test summary info ===".
// A run with none of them (--tb=no without -r, for instance) is split at its
// final summary line. From the split on, everything is kept byte for byte, but
// the PASSED lines of the short test summary. Before it, a closed grammar reads
// the session header (rePytestPlainLine), the progress lines (dots and F, E, s,
// x letters, with an optional path and percentage) and the -v lines
// "nodeid OUTCOME [ pct]". Only the passed tests are cut: the dots of a progress
// line that has no other outcome, a -v PASSED line and a summary PASSED line.
// --tb=line prints a traceback line the grammar does not know, so that run
// stays raw.

var (
	// rePytestSplitHeading opens the first section kept whole.
	rePytestSplitHeading = regexp.MustCompile(`^=+ (?:FAILURES|ERRORS|warnings summary|short test summary info) =+$`)
	// rePytestSectionHeading is any "=== title ===" line.
	rePytestSectionHeading = regexp.MustCompile(`^=+ .+ =+$`)
	rePytestShortSummary   = regexp.MustCompile(`^=+ short test summary info =+$`)
	// rePytestFinalLine is the closing summary: "=== 1 failed, 24 passed in 0.48s ===".
	rePytestFinalLine = regexp.MustCompile(`^=+ .*\b(?:failed|errors?)\b.* in .+ =+$`)
	// rePytestProgressLine: the outcome letters of a progress line, with its path and percentage.
	rePytestProgressLine = regexp.MustCompile(`^(?:` + pytestPathChars + `\.py )?([.sFExX]+)(?: +\[ *\d+%\])?$`)
	// rePytestVerboseLine is a -v line: nodeid, outcome, percentage.
	rePytestVerboseLine = regexp.MustCompile(`^\S+::\S.* (PASSED|FAILED|ERROR|SKIPPED|XFAIL|XPASS)(?: +\[ *\d+%\])?$`)
	// rePytestSummaryPassed is a PASSED line of the short test summary (-rA or -rP).
	rePytestSummaryPassed = regexp.MustCompile(`^PASSED \S.*$`)
)

// compactFailedPytest cuts the passed tests of a failed pytest run and prepends
// one summary line. The reason is non-empty when the run stays raw.
func compactFailedPytest(text string) (string, string) {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	sep := pytestFailureSplit(lines)
	if sep < 0 {
		return "", "pytest_failed_run_no_failure_section"
	}
	tail := lines[sep:]
	if !hasPytestFinalLine(tail) {
		return "", "pytest_failed_run_missing_summary"
	}
	kept, passed, reason := cutPassingPytest(lines[:sep])
	if reason != "" {
		return "", reason
	}
	tailKept, summaryPassed := cutPytestSummaryPassed(tail)
	passed += summaryPassed
	if passed == 0 {
		return "", "pytest_failed_run_no_gain"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Pytest: %d passing tests cut (failures kept verbatim)\n", passed)
	for _, l := range append(kept, tailKept...) {
		b.WriteString(l + "\n")
	}
	return b.String(), ""
}

// pytestFailureSplit returns the index of the first section kept whole, or of
// the final summary line when the run printed no such section, or -1.
func pytestFailureSplit(lines []string) int {
	for i, l := range lines {
		if rePytestSplitHeading.MatchString(l) {
			return i
		}
	}
	for i, l := range lines {
		if rePytestFinalLine.MatchString(l) {
			return i
		}
	}
	return -1
}

// hasPytestFinalLine reports whether the kept part ends in a summary that
// counts failures or errors.
func hasPytestFinalLine(tail []string) bool {
	for _, l := range tail {
		if rePytestFinalLine.MatchString(l) {
			return true
		}
	}
	return false
}

// cutPassingPytest reads the lines before the split against the closed
// grammar. It returns the lines to keep, the passed tests it cut, and a reason
// when a line is outside the grammar.
func cutPassingPytest(lines []string) (kept []string, passed int, reason string) {
	for _, l := range lines {
		if m := rePytestProgressLine.FindStringSubmatch(l); m != nil {
			if strings.Trim(m[1], ".") == "" {
				passed += len(m[1])
				continue
			}
		} else if m := rePytestVerboseLine.FindStringSubmatch(l); m != nil {
			if m[1] == "PASSED" {
				passed++
				continue
			}
		} else if strings.TrimSpace(l) != "" && !isPytestPlainLine(l) {
			return nil, 0, "pytest_failed_run_unrecognized_line_preserved"
		}
		kept = append(kept, l)
	}
	return kept, passed, ""
}

// cutPytestSummaryPassed drops the PASSED lines of the short test summary from
// the kept part and counts them.
func cutPytestSummaryPassed(tail []string) (kept []string, passed int) {
	inSummary := false
	for _, l := range tail {
		if rePytestSectionHeading.MatchString(l) {
			inSummary = rePytestShortSummary.MatchString(l)
		} else if inSummary && rePytestSummaryPassed.MatchString(l) {
			passed++
			continue
		}
		kept = append(kept, l)
	}
	return kept, passed
}
