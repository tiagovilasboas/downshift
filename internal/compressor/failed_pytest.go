// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// Pytest failed runs, read from the default dot progress and from -v. A run is
// split at its first section that is kept: "=== FAILURES ===", "=== ERRORS ===",
// "=== warnings summary ===" or "=== short test summary info ===".
// A run with none of them (--tb=no without -r, for instance) is split at its
// final summary line. In failure blocks, non-error body lines are stripped while
// preserving the test header, error lines, and file:line locations. Duplicate
// FAILED lines in short test summary info are stripped when already shown in
// FAILURES. Before the split, passing dots (including on mixed progress lines)
// and -v PASSED lines are counted and cut.

var (
	// rePytestSplitHeading opens the first section kept.
	rePytestSplitHeading = lazyre.New(`^=+ (?:FAILURES|ERRORS|warnings summary|short test summary info) =+$`)
	// rePytestSectionHeading is any "=== title ===" line.
	rePytestSectionHeading = lazyre.New(`^=+ (.+?) =+$`)
	// rePytestFinalLine is the closing summary: "=== 1 failed, 24 passed in 0.48s ===" or "-q" form "1 failed, 24 passed in 0.48s".
	rePytestFinalLine = lazyre.New(`^(?:=+ )?.*\b(?:failed|errors?)\b.* in .+`)
	// rePytestProgressLine: the outcome letters of a progress line, with its path and percentage.
	rePytestProgressLine = lazyre.New(`^(?:` + pytestPathChars + `\.py )?([.sFExX]+)(?: +\[ *\d+%\])?$`)
	// rePytestVerboseLine is a -v line: nodeid, outcome, percentage.
	rePytestVerboseLine = lazyre.New(`^\S+::\S.* (PASSED|FAILED|ERROR|SKIPPED|XFAIL|XPASS)(?: +\[ *\d+%\])?$`)
	// rePytestSummaryPassed is a PASSED line of the short test summary (-rA or -rP).
	rePytestSummaryPassed = lazyre.New(`^PASSED \S.*$`)
	// rePytestSummaryFailed matches a FAILED line in short test summary info.
	rePytestSummaryFailed = lazyre.New(`^FAILED\s+(\S+?)(?:\s+-\s+.*)?$`)
	// rePytestTestHeader matches the test header inside FAILURES: "___ test_name ___".
	rePytestTestHeader = lazyre.New(`^_{3,}\s*(.*?)\s*_{3,}$`)
	// rePytestErrorLine matches an error or assertion line inside a failure block.
	rePytestErrorLine = lazyre.New(`^E(?:\s.*)?$`)
	// rePytestFileLine matches a file:line: message inside a failure block.
	rePytestFileLine = lazyre.New(`^\S+:\d+: .*$`)
	// rePytestCapturedHeader matches standard pytest captured output section delimiters.
	rePytestCapturedHeader = lazyre.New(`^--+\s+Captured\s+(?:stdout|stderr|log)\s+.*?--+$`)
)

// compactFailedPytest cuts passing tests and noise from a failed pytest run and prepends
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
	tailKept, summaryPassed := compactPytestTail(tail)
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

// pytestFailureSplit returns the index of the first section kept, or of
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
// grammar. It counts passing dots (including on mixed progress lines) and
// cuts all-passing progress or verbose lines.
func cutPassingPytest(lines []string) (kept []string, passed int, reason string) {
	for _, l := range lines {
		if m := rePytestProgressLine.FindStringSubmatch(l); m != nil {
			dots := strings.Count(m[1], ".")
			passed += dots
			if strings.Trim(m[1], ".") == "" {
				continue
			}
		} else if m := rePytestVerboseLine.FindStringSubmatch(l); m != nil {
			if m[1] == "PASSED" {
				passed++
				continue
			}
		} else if strings.TrimSpace(l) != "" && !rePytestPlainLine.MatchString(l) {
			return nil, 0, "pytest_failed_run_unrecognized_line_preserved"
		}
		kept = append(kept, l)
	}
	return kept, passed, ""
}

// compactPytestTail processes the tail of a pytest failed run. It strips non-error
// body lines from failure blocks and drops duplicate FAILED lines in short test summary info.
func compactPytestTail(tail []string) (kept []string, passed int) {
	failedTests := map[string]bool{}
	for _, l := range tail {
		if m := rePytestTestHeader.FindStringSubmatch(l); m != nil {
			name := strings.TrimSpace(m[1])
			failedTests[name] = true
			if idx := strings.Index(name, "["); idx != -1 {
				failedTests[name[:idx]] = true
			}
			if parts := strings.Split(name, "."); len(parts) > 1 {
				failedTests[parts[len(parts)-1]] = true
			}
		}
	}

	inSection := ""
	inFailBlock := false
	inCaptured := false
	var summaryLines []string

	flushSummary := func() {
		if len(summaryLines) > 0 {
			kept = append(kept, "=========================== short test summary info ============================")
			kept = append(kept, summaryLines...)
			summaryLines = nil
		}
	}

	for _, l := range tail {
		if rePytestFinalLine.MatchString(l) {
			flushSummary()
			inSection = ""
			inFailBlock = false
			kept = append(kept, l)
			continue
		}
		if m := rePytestSectionHeading.FindStringSubmatch(l); m != nil {
			flushSummary()
			inSection = m[1]
			inFailBlock = false
			inCaptured = false
			if inSection != "short test summary info" {
				kept = append(kept, l)
			}
			continue
		}

		if inSection == "FAILURES" || inSection == "ERRORS" {
			if rePytestTestHeader.MatchString(l) {
				inFailBlock = true
				inCaptured = false
				kept = append(kept, l)
				continue
			}
			if inFailBlock {
				if rePytestCapturedHeader.MatchString(l) {
					inCaptured = true
					kept = append(kept, l)
					continue
				}
				if inCaptured {
					kept = append(kept, l)
					continue
				}
				if rePytestErrorLine.MatchString(l) || rePytestFileLine.MatchString(l) {
					kept = append(kept, l)
					continue
				}
				continue
			}
			kept = append(kept, l)
			continue
		}

		if inSection == "short test summary info" {
			if rePytestSummaryPassed.MatchString(l) {
				passed++
				continue
			}
			if m := rePytestSummaryFailed.FindStringSubmatch(l); m != nil {
				nodeid := m[1]
				if isPytestDuplicateFailure(nodeid, failedTests) {
					continue
				}
			}
			summaryLines = append(summaryLines, l)
			continue
		}

		kept = append(kept, l)
	}
	flushSummary()
	return kept, passed
}

// isPytestDuplicateFailure reports whether nodeid refers to a test already shown in FAILURES.
func isPytestDuplicateFailure(nodeid string, failedTests map[string]bool) bool {
	if len(failedTests) == 0 {
		return false
	}
	if failedTests[nodeid] {
		return true
	}
	parts := strings.Split(nodeid, "::")
	last := parts[len(parts)-1]
	if failedTests[last] {
		return true
	}
	if idx := strings.Index(last, "["); idx != -1 && failedTests[last[:idx]] {
		return true
	}
	return false
}
