// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

var (
	rePlaywrightHeader  = regexp.MustCompile(`(?m)^\s*Running\s+(\d+)\s+tests\s+using\s+(\d+)\s+workers?`)
	rePlaywrightSummary = regexp.MustCompile(`(?m)^\s*(\d+)\s+passed\s+\(([^)]+)\)`)
	rePlaywrightError   = regexp.MustCompile(`(?i)(?:failed|flaky|timed out|Error:|AssertionError|trace(?: file| attached)?|retry(?:ing| #))`)
)

func isPlaywright(input []byte) bool {
	return rePlaywrightHeader.Match(input) ||
		(rePlaywrightSummary.Match(input) && (bytes.Contains(input, []byte("worker")) || bytes.Contains(input, []byte("Running"))))
}

// The only line shapes a clean plain Playwright run prints. Recognition is the
// rule: free text is accepted only as a test title inside a passed-test line,
// and nothing may follow the closing duration. Any other line, including an
// injected error, warning or FAIL, keeps the whole transcript raw.
var (
	rePlaywrightBlankLine   = regexp.MustCompile(`^\s*$`)
	rePlaywrightRunHeader   = regexp.MustCompile(`^Running \d+ tests? using \d+ workers?$`)
	rePlaywrightPassedTest  = regexp.MustCompile(`^\s*✓\s+\d+\s+(?:\[[^\]\s]+\]\s+›\s+)?\S+(?:\s+›\s+.+?)?\s+\(\d+(?:\.\d+)?(?:ms|s|m)\)$`)
	rePlaywrightSummaryLine = regexp.MustCompile(`^\s*\d+\s+passed\s+\(\d+(?:\.\d+)?(?:ms|s|m)\)$`)
)

// playwrightLinesRecognised reports whether every line of input is part of the
// plain Playwright success grammar, with exactly one run header and one summary.
func playwrightLinesRecognised(input []byte) bool {
	headers, summaries := 0, 0
	for _, line := range bytes.Split(input, []byte("\n")) {
		switch {
		case rePlaywrightBlankLine.Match(line), rePlaywrightPassedTest.Match(line):
		case rePlaywrightRunHeader.Match(line):
			headers++
		case rePlaywrightSummaryLine.Match(line):
			summaries++
		default:
			return false
		}
	}
	return headers == 1 && summaries == 1
}

// compressPlaywright compacts clean playwright output to a one-line summary:
// Playwright: PASS (N passed in <time>)
// If not strictly shorter than raw input, the input is returned.
func compressPlaywright(input []byte) ([]byte, bool, string) {
	if linesHaveDangerWord(input, nil) {
		return input, false, "playwright_danger_word_preserved"
	}
	if !playwrightLinesRecognised(input) {
		return input, false, "playwright_unrecognised_line_preserved"
	}
	m := rePlaywrightSummary.FindSubmatch(input)
	if m == nil {
		return input, false, "playwright_missing_summary"
	}
	passedCount := string(m[1])
	timeStr := strings.TrimSpace(string(m[2]))

	out := fmt.Sprintf("Playwright: PASS (%s passed in %s)\n", passedCount, timeStr)
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed playwright summary (%s passed in %s)", passedCount, timeStr)
}
