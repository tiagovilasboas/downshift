// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

const failedPlaywrightDir = "failed_playwright"

// playwrightPass is a test that passed at its first attempt: a check mark
// before the failure blocks and no retry marker. A check mark with a retry
// marker is a flaky test and stays.
var playwrightPass = regexp.MustCompile(`^\s+✓\s+\d+ `)

// playwrightNoise returns the predicate for one transcript.
func playwrightNoise(in []byte) func(string) bool {
	text := string(in)
	cut := regexp.MustCompile(`(?m)^ {2}1\) `).FindStringIndex(text)
	before := text[:cut[0]]
	passing := map[string]bool{}
	for _, l := range strings.Split(before, "\n") {
		if playwrightPass.MatchString(l) && !strings.Contains(l, "(retry #") {
			passing[l] = true
		}
	}
	return func(l string) bool { return passing[l] }
}

// Real @playwright/test 1.64 list-reporter runs: a number mismatch, an object
// diff, a thrown error and a timeout, and the same run with one retry, which
// adds a flaky test and a "Retry #1" section to every failing block.
func TestCompressFailedPlaywrightRealRuns(t *testing.T) {
	cases := []struct {
		name    string
		failure []string
	}{
		{"list_fail", []string{
			"  ✘   4 tests/billing.spec.js:7:3 › billing › applies the discount (1ms)",
			"  -  10 tests/billing.spec.js:14:8 › billing › refunds",
			"    Expected: 110",
			"    Received: 100",
			"    +   \"currency\": \"BRL\",",
			"    Error: negative amount not allowed",
			"    Test timeout of 100ms exceeded.",
			"  4 failed",
			"  1 skipped",
		}},
		{"list_fail_retries", []string{
			"  ✓   4 tests/flaky.spec.js:3:1 › settles on the second attempt (retry #1) (2ms)",
			"  ✘   5 tests/billing.spec.js:7:3 › billing › applies the discount (retry #1) (4ms)",
			"    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────",
			"  5) tests/flaky.spec.js:3:1 › settles on the second attempt ───────────────────────────────────────",
			"  1 flaky",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, r := goldenFailedRun(t, failedPlaywrightDir, compressor.FormatPlaywright, tc.name)
			assertOnlyNoiseCut(t, in, r.Output, "Playwright: ", playwrightNoise(in))
			assertContains(t, r.Output, tc.failure)
		})
	}
}

// Anything the grammar does not know, or a report that does not add up,
// returns the run unchanged.
func TestCompressFailedPlaywrightFailsOpen(t *testing.T) {
	in := readFailedFixture(t, failedPlaywrightDir, "list_fail")
	allFailed := "\nRunning 1 test using 1 worker\n\n  ✘  1 tests/a.spec.js:3:1 › a (2ms)\n\n\n" +
		"  1) tests/a.spec.js:3:1 › a ─────────────────────────────────────────\n\n    Error: boom\n\n  1 failed\n"
	cases := []struct {
		name   string
		input  []byte
		reason string
	}{
		{"what a test printed between the lines", readFailedFixture(t, failedPlaywrightDir, "list_fail_console_output"), "playwright_failed_run_unrecognized_line_preserved"},
		{"unknown line in the test list", mutate(t, in, "  ✓   2 tests/billing", "something unexpected\n  ✓   2 tests/billing"), "playwright_failed_run_unrecognized_line_preserved"},
		{"two run headers", mutate(t, in, "Running 10 tests using 2 workers", "Running 10 tests using 2 workers\nRunning 10 tests using 2 workers"), "playwright_failed_run_header_preserved"},
		{"no run header", mutate(t, in, "Running 10 tests using 2 workers", ""), "playwright_failed_run_header_preserved"},
		{"no failure block", []byte(strings.SplitN(string(in), "  1) tests/", 2)[0]), "playwright_failed_run_no_failure_block"},
		{"summary counts fewer failures", mutate(t, in, "  4 failed", "  3 failed"), "playwright_failed_run_summary_mismatch"},
		{"summary missing", mutate(t, in, "  4 failed", "  4 broken"), "playwright_failed_run_summary_mismatch"},
		{"blocks out of order", mutate(t, in, "  2) tests/billing", "  3) tests/billing"), "playwright_failed_run_blocks_out_of_order"},
		{"carriage return", mutate(t, in, "Running 10 tests using 2 workers\n", "Running 10 tests using 2 workers\r\n"), ""},
		{"colour kept by another escape", mutate(t, in, "Running 10", "\x1b[2KRunning 10"), ""},
		{"no final newline", bytes.TrimRight(in, "\n"), ""},
		{"NUL byte", mutate(t, in, "Running 10", "\x00Running 10"), ""},
		{"every test failed, nothing to cut", []byte(allFailed), "playwright_failed_run_no_gain"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { assertRaw(t, compressor.FormatPlaywright, tc.input, tc.reason) })
	}
}

// A failed run of a format without a failure compactor is declined.
func TestCompressFailedDeclinesOtherFormats(t *testing.T) {
	in := readFailedFixture(t, failedPlaywrightDir, "list_fail")
	assertRaw(t, compressor.FormatMypy, in, "failed_run_format_unsupported")
}
