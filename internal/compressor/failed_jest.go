// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

// Jest failed runs, read from the default and the verbose reporter. A run is
// split at its first failure block ("  ● suite › title", or "  ● Test suite
// failed to run"). Everything from there on (each failure with its message,
// code frame and stack, any console output after it, then the Test Suites,
// Tests, Snapshots and Time summary) is kept byte for byte. Before it, a closed
// grammar reads the npm banner, one PASS or FAIL line per file, the verbose
// tree of a file (describe lines, and ✓, ✕ and ○ test lines) and the indented
// output a test printed. Only the PASS lines of files that passed and the ✓
// lines of tests that passed are cut. Any other line outside the grammar keeps
// the whole run raw.

var (
	// reJestFailBlock opens the first failure block.
	reJestFailBlock = regexp.MustCompile(`^  ● \S.*$`)
	// reJestFailedSuites confirms the kept part ends in a summary that counts failed suites.
	reJestFailedSuites = regexp.MustCompile(`(?m)^Test Suites:\s.*\bfailed\b`)
	// reJestPassedFile is a PASS file line; reJestFailedFile a FAIL one (kept).
	reJestPassedFile = regexp.MustCompile(`^ ?PASS\s+\S.*$`)
	reJestFailedFile = regexp.MustCompile(`^ ?FAIL\s+\S.*$`)
	// reJestPassedTest is a verbose line of a test that passed, with its duration.
	reJestPassedTest = regexp.MustCompile(`^ +✓ \S.* \(\d+ ms\)$`)
	// reJestIndented is a describe line, a ✕, ○ or ✎ test line, or what a test printed.
	reJestIndented = regexp.MustCompile(`^ +\S.*$`)
)

// compactFailedJest cuts the passing lines of a failed jest run and prepends
// one summary line. The reason is non-empty when the run stays raw.
func compactFailedJest(text string) (string, string) {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	sep := -1
	for i, l := range lines {
		if reJestFailBlock.MatchString(l) {
			sep = i
			break
		}
	}
	if sep < 0 {
		return "", "jest_failed_run_no_failure_block"
	}
	if !reJestFailedSuites.MatchString(strings.Join(lines[sep:], "\n")) {
		return "", "jest_failed_run_missing_summary"
	}
	kept, files, tests, reason := cutPassingJest(lines[:sep])
	if reason != "" {
		return "", reason
	}
	if files == 0 && tests == 0 {
		return "", "jest_failed_run_no_gain"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Jest: %d passing files, %d passing test lines cut (failures kept verbatim)\n", files, tests)
	for _, l := range append(kept, lines[sep:]...) {
		b.WriteString(l + "\n")
	}
	return b.String(), ""
}

// cutPassingJest reads the lines before the first failure block against the
// closed grammar. It returns the lines to keep, the passing files and passing
// tests it cut, and a reason when a line is outside the grammar.
func cutPassingJest(lines []string) (kept []string, files, tests int, reason string) {
	for _, l := range lines {
		switch {
		case reJestPassedFile.MatchString(l):
			files++
			continue
		case reJestPassedTest.MatchString(l):
			tests++
			continue
		case strings.TrimSpace(l) == "", reJestFailedFile.MatchString(l), reJestIndented.MatchString(l),
			reScriptBannerLine.MatchString(l), reNpmWarnLine.MatchString(l):
		default:
			// Unknown line before the failures: keep it (when unsure, keep).
		}
		kept = append(kept, l)
	}
	return kept, files, tests, ""
}
