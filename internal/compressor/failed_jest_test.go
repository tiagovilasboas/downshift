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

// jestSuiteMark is the reporter's mark of a suite that passed, and
// jestTestMark the verbose line of a test that passed, with its duration.
var (
	jestSuiteMark = regexp.MustCompile(`^ ?PASS\s+\S`)
	jestTestMark  = regexp.MustCompile(`^ +✓ \S.* \(\d+ ms\)$`)
)

func jestNoise(l string) bool { return jestSuiteMark.MatchString(l) || jestTestMark.MatchString(l) }

// Real-looking jest runs (testdata/corpus). Each keeps every failure line and
// cuts only the PASS lines and the ✓ lines; the summary states the cut counts.
func TestCompressFailedJestCutsOnlyPassingLines(t *testing.T) {
	cases := []struct {
		file    string
		summary string
		failure []string
	}{
		{"jest_fail_many_suites.txt", "Jest: 6 passing files, 0 passing test lines cut", []string{
			" FAIL  src/billing/invoice.test.ts",
			"  ● invoice › applies the regional tax",
			"    Received: 100",
			"Test Suites: 1 failed, 6 passed, 7 total",
			"Tests:       1 failed, 41 passed, 42 total",
		}},
		{"jest_fail_verbose_colour.txt", "Jest: 1 passing files, 4 passing test lines cut", []string{
			"    ✕ applies the regional tax (8 ms)",
			"    ○ skipped applies the surcharge",
			"Tests:       1 failed, 1 skipped, 4 passed, 6 total",
		}},
		{"jest_fail_npm_console.txt", "Jest: 3 passing files, 0 passing test lines cut", []string{
			"> shop@2.4.0 test",
			"  console.log",
			"  console.warn",
			"Test Suites: 1 failed, 3 passed, 4 total",
		}},
		{"jest_fail_suite_error.txt", "Jest: 4 passing files, 0 passing test lines cut", []string{
			"  ● Test suite failed to run",
			"Cannot find module './tax' from 'src/checkout/broken.test.ts'",
		}},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			in := readFixture(t, c.file)
			r := compressor.CompressFailed(in, compressor.FormatJest)
			if !r.Applied || r.Format != compressor.FormatJest || len(r.Output) >= len(in) {
				t.Fatalf("not compacted: applied=%v format=%s reason=%q %d -> %d", r.Applied, r.Format, r.Reason, len(in), len(r.Output))
			}
			assertOnlyNoiseCut(t, in, r.Output, c.summary, jestNoise)
			assertContains(t, r.Output, c.failure)
		})
	}
}

// A run where every suite failed has nothing to cut: it stays raw.
func TestCompressFailedJestAllFailingStaysRaw(t *testing.T) {
	assertRaw(t, compressor.FormatJest, readFixture(t, "jest_fail_all.txt"), "jest_failed_run_no_gain")
}

// Each input is a run the grammar cannot read in full, or one with no failure
// signal: the output must come back byte for byte.
func TestCompressFailedJestFailsOpen(t *testing.T) {
	in := readFixture(t, "jest_fail_many_suites.txt")
	cases := []struct {
		name   string
		input  []byte
		reason string
	}{
		{"CRLF", []byte(strings.ReplaceAll(string(in), "\n", "\r\n")), "jest_failed_run_raw_preserved"},
		{"no final newline", bytes.TrimSuffix(in, []byte("\n")), "jest_failed_run_raw_preserved"},
		{"cursor escape", mutate(t, in, " PASS  src/cart/totals", "\x1b[2K PASS  src/cart/totals"), "jest_failed_run_raw_preserved"},
		{"no failure block", mutate(t, in, "  ● invoice", "  x invoice"), "jest_failed_run_no_failure_block"},
		{"no failed-suites summary", mutate(t, in, "Test Suites: 1 failed", "Suites: 1 failed"), "jest_failed_run_missing_summary"},
		{"no failure signal", []byte(" PASS  src/a.test.ts\n\nTest Suites: 1 passed, 1 total\n"), "jest_failed_run_no_failure_block"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRaw(t, compressor.FormatJest, c.input, c.reason)
		})
	}
}
