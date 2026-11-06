// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

// Vitest failed runs. A run is split at the first failure section heading
// ("Failed Tests N", "Failed Suites N" or "Unhandled Errors"). Everything from
// that heading on (every FAIL block with its assertion, diff, code frame and
// stack frames, then the summary) is kept byte for byte. Before it, a closed
// grammar reads the RUN header, one line per test file, the test tree of a
// file that failed, and the stdout and stderr blocks of the tests. Only the
// lines of files, suites and tests that passed are cut; any line outside the
// grammar keeps the whole run raw. The shapes come from real vitest 3, 4 and 5
// runs (testdata/failed_vitest).

var (
	// reVitestFailSection starts the part of the report that is kept whole.
	reVitestFailSection = regexp.MustCompile(`^⎯{3,} (?:Failed Tests|Failed Suites|Unhandled Errors) ?\d* ⎯{3,}$`)
	reVitestFailHeading = regexp.MustCompile(`^⎯{3,} Failed (?:Tests|Suites) (\d+) ⎯{3,}$`)
	reVitestFailFooter  = regexp.MustCompile(`^⎯+\[(\d+)/(\d+)\]⎯$`)
	reVitestFailSummary = regexp.MustCompile(`(?m)^\s*(?:Test Files|Tests|Errors)\s+.*\b(?:failed|errors?)\b`)

	// A file that failed or was skipped keeps its line: "❯ path (8 tests | 3 failed | 1 skipped) 10ms".
	reVitestKeptFileLine = regexp.MustCompile(`^ {1,2}[❯↓] (?:\|[\w@./-]+\| +)?\S+ \(\d+(?: tests?)?(?: \| \d+ [a-z]+)*\)(?: ` + vitestSpan + `)?$`)
	// Lines of the test tree that are not a pass: failed (×), skipped (↓),
	// todo (□) and a suite with a failure (❯, vitest 5), plus the "→ reason"
	// line vitest 3 prints under a failed test.
	reVitestKeptTreeLine = regexp.MustCompile(`^ {3,}[×↓□❯] \S.*$`)
	reVitestReasonLine   = regexp.MustCompile(`^ {4,}→ .*$`)
	// A passed test, or a passed suite, in the tree of a file that failed.
	reVitestPassedTreeLine = regexp.MustCompile(`^ {3,}✓ \S.*(?: \d+(?:\.\d+)?(?:ms|s)| \(\d+\))$`)
	// reVitestStreamHeader opens what a test wrote: "stdout | path > test name".
	reVitestStreamHeader = regexp.MustCompile(`^(?:stdout|stderr) \| \S.*$`)
)

// vitestCut counts what a failed run dropped.
type vitestCut struct{ files, lines int }

// compactFailedVitest cuts the passing noise of a failed vitest run and
// prepends one summary line. The reason is non-empty when the run stays raw.
func compactFailedVitest(text string) (string, string) {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	sep := -1
	for i, l := range lines {
		if reVitestFailSection.MatchString(l) {
			sep = i
			break
		}
	}
	if sep < 0 {
		return "", "vitest_failed_run_no_failure_section"
	}
	if reason := checkVitestFailureSection(lines[sep:]); reason != "" {
		return "", reason
	}
	kept, cut, reason := cutPassingVitest(lines[:sep])
	if reason != "" {
		return "", reason
	}
	if cut.files == 0 && cut.lines == 0 {
		return "", "vitest_failed_run_no_gain"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Vitest: %d passing files, %d passing test lines cut (failures kept verbatim)\n", cut.files, cut.lines)
	for _, l := range append(kept, lines[sep:]...) {
		b.WriteString(l + "\n")
	}
	return b.String(), ""
}

// checkVitestFailureSection confirms the kept part is a whole report: it ends
// in a summary that counts failures, and every "Failed Tests N" or "Failed
// Suites N" heading reached its last footer "[N/N]".
func checkVitestFailureSection(tail []string) string {
	if !reVitestFailSummary.MatchString(strings.Join(tail, "\n")) {
		return "vitest_failed_run_missing_summary"
	}
	last := map[string]bool{}
	for _, l := range tail {
		if m := reVitestFailFooter.FindStringSubmatch(l); m != nil && m[1] == m[2] {
			last[m[2]] = true
		}
	}
	for _, l := range tail {
		if m := reVitestFailHeading.FindStringSubmatch(l); m != nil && !last[m[1]] {
			return "vitest_failed_run_truncated_failure_section"
		}
	}
	return ""
}

// cutPassingVitest reads the lines before the failure section against the
// closed grammar and returns the ones to keep.
func cutPassingVitest(lines []string) (kept []string, cut vitestCut, reason string) {
	headers := 0
	inStream, inReason := false, false
	for _, l := range lines {
		switch {
		case inStream:
			// Whatever a test printed, up to the blank line that ends the block.
			inStream = strings.TrimSpace(l) != ""
		case strings.TrimSpace(l) == "":
			inReason = false
		case headers == 0 && reScriptBannerLine.MatchString(l):
			// "> pkg@1.0.0 test" and "> vitest run" from npm or pnpm.
			cut.lines++
			continue
		case reVitestHeaderLine.MatchString(l):
			headers++
			inReason = false
		case isPassedVitestFile(l):
			cut.files++
			inReason = false
			continue
		case reVitestPassedTreeLine.MatchString(l):
			cut.lines++
			inReason = false
			continue
		case reVitestKeptFileLine.MatchString(l), reVitestKeptTreeLine.MatchString(l), reVitestNotice.MatchString(l), reNpmWarnLine.MatchString(l):
			inReason = false
		case reVitestReasonLine.MatchString(l):
			inReason = true
		case reVitestStreamHeader.MatchString(l):
			inStream, inReason = true, false
		case inReason:
			// A reason of more than one line continues at the start of the line.
		default:
			// Unknown line before the failures: keep it (when unsure, keep).
			inReason = false
		}
		kept = append(kept, l)
	}
	if headers != 1 {
		return nil, vitestCut{}, "vitest_failed_run_header_preserved"
	}
	return kept, cut, ""
}

// isPassedVitestFile reports a "✓ path (N tests) 3ms" line of a file whose
// tests all passed. The file lines sit one or two spaces in; a passed suite in
// the tree sits deeper and is a passed test line.
func isPassedVitestFile(l string) bool {
	trimmed := strings.TrimLeft(l, " ")
	return len(l)-len(trimmed) <= 2 && strings.HasPrefix(trimmed, "✓") && reVitestFileLine.MatchString(l)
}
