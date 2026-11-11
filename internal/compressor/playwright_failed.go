// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// Playwright failed runs, read from the list reporter. A run is split at its
// first failure block ("  1) path:line:col › title ─────"). Everything from
// there on (every block with its error, expected and received values, code
// frame, stack frame and retry, then the failed, flaky, skipped and passed
// summary) is kept byte for byte. Before it, a closed grammar reads the run
// header and one line per test. Only the lines of tests that passed at the
// first attempt are cut. A failed test, a skipped test and a test that passed
// on a retry keep their line. Any line outside the grammar keeps the whole
// run raw; that includes what a test printed with console.log, which shows up
// between the test lines.

const playwrightSpan = `\(\d+(?:\.\d+)?(?:ms|s|m)\)`

var (
	// rePlaywrightFailBlock opens the first failure block.
	rePlaywrightFailBlock = lazyre.New(`^ {2}1\) \S.* ─{3,}$`)
	rePlaywrightBlockHead = lazyre.New(`^ {2}(\d+)\) \S.* ─{3,}$`)
	// rePlaywrightFailedTest and rePlaywrightSkippedTest are the lines kept in place.
	rePlaywrightFailedTest  = lazyre.New(`^\s*✘\s+\d+\s+\S.* ` + playwrightSpan + `$`)
	rePlaywrightSkippedTest = lazyre.New(`^\s*-\s+\d+\s+\S.*$`)
	// rePlaywrightCount reads the summary: "  4 failed", "  1 flaky".
	rePlaywrightCount = lazyre.New(`^ {2}(\d+) (failed|flaky|skipped|did not run|interrupted|passed)(?: .*)?$`)
)

// compactFailedPlaywright cuts the passing noise of a failed playwright run
// and prepends one summary line. The reason is non-empty when the run stays raw.
func compactFailedPlaywright(text string) (string, string) {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	sep := -1
	for i, l := range lines {
		if rePlaywrightFailBlock.MatchString(l) {
			sep = i
			break
		}
	}
	if sep < 0 {
		return "", "playwright_failed_run_no_failure_block"
	}
	if reason := checkPlaywrightFailureSection(lines[sep:]); reason != "" {
		return "", reason
	}
	kept, cut, reason := cutPassingPlaywright(lines[:sep])
	if reason != "" {
		return "", reason
	}
	if cut == 0 {
		return "", "playwright_failed_run_no_gain"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Playwright: %d passing tests cut (failures kept verbatim)\n", cut)
	for _, l := range append(kept, lines[sep:]...) {
		b.WriteString(l + "\n")
	}
	return b.String(), ""
}

// checkPlaywrightFailureSection confirms the kept part is a whole report: the
// failure blocks are numbered 1 to N, and the summary counts N failed and
// flaky tests.
func checkPlaywrightFailureSection(tail []string) string {
	blocks, counted := 0, 0
	for _, l := range tail {
		if m := rePlaywrightBlockHead.FindStringSubmatch(l); m != nil {
			blocks++
			if n, _ := strconv.Atoi(m[1]); n != blocks {
				return "playwright_failed_run_blocks_out_of_order"
			}
		}
		if m := rePlaywrightCount.FindStringSubmatch(l); m != nil && (m[2] == "failed" || m[2] == "flaky") {
			n, _ := strconv.Atoi(m[1])
			counted += n
		}
	}
	if blocks == 0 || blocks != counted {
		return "playwright_failed_run_summary_mismatch"
	}
	return ""
}

// cutPassingPlaywright reads the lines before the first failure block against
// the closed grammar and returns the ones to keep.
func cutPassingPlaywright(lines []string) (kept []string, cut int, reason string) {
	headers := 0
	for _, l := range lines {
		switch {
		case rePlaywrightBlankLine.MatchString(l), rePlaywrightFailedTest.MatchString(l), rePlaywrightSkippedTest.MatchString(l):
		case rePlaywrightRunHeader.MatchString(l):
			headers++
		case rePlaywrightPassedTest.MatchString(l):
			if !strings.Contains(l, "(retry #") {
				cut++
				continue
			}
		default:
			return nil, 0, "playwright_failed_run_unrecognized_line_preserved"
		}
		kept = append(kept, l)
	}
	if headers != 1 {
		return nil, 0, "playwright_failed_run_header_preserved"
	}
	return kept, cut, ""
}
