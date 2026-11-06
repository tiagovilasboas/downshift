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

const failedVitestDir = "failed_vitest"

// vitestPassMark is how every vitest version marks a passed file, suite or
// test: a check mark after the indent. It counts only before the failure
// report, where a ✓ can only be a reporter line.
var vitestPassMark = regexp.MustCompile(`^ +✓ `)

// vitestNoise returns the predicate for one transcript: the check-mark lines
// before the first failure section heading.
func vitestNoise(in []byte) func(string) bool {
	plain := sgrColour.ReplaceAllString(string(in), "")
	heading := regexp.MustCompile(`(?m)^⎯{3,} (?:Failed Tests|Failed Suites|Unhandled Errors)`).FindStringIndex(plain)
	before := plain[:heading[0]]
	passing := map[string]int{}
	for _, l := range strings.Split(before, "\n") {
		if vitestPassMark.MatchString(l) {
			passing[l]++
		}
	}
	return func(l string) bool {
		if passing[l] > 0 {
			passing[l]--
			return true
		}
		return false
	}
}

// Real vitest 3, 4 and 5 runs: three failing assertions (a number, an object
// diff, a thrown error), a timeout, a skipped and a todo test, a console.log, a
// passing suite nested in a failing file, and a run with a file that cannot be
// imported and an unhandled rejection.
func TestCompressFailedVitestRealRuns(t *testing.T) {
	cases := []struct {
		name    string
		failure []string
	}{
		{"v3_fail", []string{
			" FAIL  test/billing.test.js > billing > applies the discount",
			"AssertionError: expected 100 to be 110 // Object.is equality",
			"     → negative amount not allowed",
			"Error: Test timed out in 50ms.",
			"      Tests  4 failed | 9 passed | 1 skipped | 1 todo (15)",
		}},
		{"v4_fail", []string{
			" FAIL  test/billing.test.js > billing > builds the receipt",
			"-   \"currency\": \"USD\",",
			"+   \"currency\": \"BRL\",",
			"Error: negative amount not allowed",
			"     × rejects a negative amount 0ms",
			"      Tests  4 failed | 9 passed | 1 skipped | 1 todo (15)",
		}},
		{"v5_fail", []string{
			"   ❯ billing (6)",
			" FAIL  test/slow.test.js > finishes in time",
			"If this is a long-running test, pass a timeout value as the last argument or configure it globally with \"testTimeout\".",
			"⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯[4/4]⎯",
		}},
		{"v3_suite_errors", []string{
			" FAIL  test/broken.test.js [ test/broken.test.js ]",
			"⎯⎯⎯⎯⎯⎯ Unhandled Errors ⎯⎯⎯⎯⎯⎯",
			"Error: background job failed",
			"     Errors  1 error",
		}},
		{"v4_suite_errors", []string{
			" FAIL  test/broken.test.js [ test/broken.test.js ]",
			"⎯⎯⎯⎯⎯⎯ Unhandled Errors ⎯⎯⎯⎯⎯⎯",
			"Error: background job failed",
			"     Errors  1 error",
		}},
		{"v5_suite_errors", []string{
			" FAIL  test/broken.test.js [ test/broken.test.js ]",
			"⎯⎯⎯⎯⎯⎯ Unhandled Errors ⎯⎯⎯⎯⎯⎯",
			"Error: background job failed",
			"     Errors  1 error",
		}},
		{"v4_fail_colour", []string{
			" FAIL  test/billing.test.js > billing > applies the discount",
			"AssertionError: expected 100 to be 110 // Object.is equality",
			"      Tests  4 failed | 9 passed | 1 skipped | 1 todo (15)",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, r := goldenFailedRun(t, failedVitestDir, compressor.FormatVitest, tc.name)
			assertOnlyNoiseCut(t, in, r.Output, "Vitest: ", vitestNoise(in))
			assertContains(t, r.Output, tc.failure)
		})
	}
}

// Anything the grammar does not know, or a failure section that is not whole,
// returns the run unchanged.
func TestCompressFailedVitestFailsOpen(t *testing.T) {
	in := readFailedFixture(t, failedVitestDir, "v4_fail")
	cases := []struct {
		name   string
		input  []byte
		reason string
	}{
		{"two runs in one transcript", mutate(t, in, " RUN  v4.1.12", " RUN  v4.1.12 /a\n\n RUN  v4.1.12"), "vitest_failed_run_header_preserved"},
		{"no run header", mutate(t, in, " RUN  v4.1.12 /work/app", ""), "vitest_failed_run_header_preserved"},
		{"no failure section", []byte(strings.SplitN(string(in), "⎯⎯⎯⎯⎯⎯⎯ Failed Tests", 2)[0]), "vitest_failed_run_no_failure_section"},
		{"last failure footer missing", mutate(t, in, "[4/4]⎯", "[3/4]⎯"), "vitest_failed_run_truncated_failure_section"},
		{"summary missing", mutate(t, mutate(t, in, "Test Files  2 failed", "Test Files  2 broken"), "Tests  4 failed", "Tests  4 broken"), "vitest_failed_run_missing_summary"},
		{"carriage return", mutate(t, in, " RUN  v4.1.12 /work/app\n", " RUN  v4.1.12 /work/app\r\n"), ""},
		{"no final newline", bytes.TrimRight(in, "\n"), ""},
		{"cursor movement escape", mutate(t, in, " RUN  v4.1.12", "\x1b[2K RUN  v4.1.12"), ""},
		{"NUL byte", mutate(t, in, " RUN  v4.1.12", "\x00 RUN  v4.1.12"), ""},
		{"invalid UTF-8", mutate(t, in, " RUN  v4.1.12", "\xff RUN  v4.1.12"), ""},
		{"only failures, nothing to cut", []byte(strings.Join(failureOnlyVitest(), "\n") + "\n"), "vitest_failed_run_no_gain"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { assertRaw(t, compressor.FormatVitest, tc.input, tc.reason) })
	}
}

// What a test printed is kept whole, even a line shaped like a passed test.
func TestCompressFailedVitestKeepsWhatATestPrinted(t *testing.T) {
	in := mutate(t, readFailedFixture(t, failedVitestDir, "v4_fail"), "waiting for the gateway\n",
		"waiting for the gateway\n   ✓ looks like a passed test 1ms\n")
	r := compressor.CompressFailed(in, compressor.FormatVitest)
	if !r.Applied {
		t.Fatalf("not compacted: %q", r.Reason)
	}
	assertContains(t, r.Output, []string{"waiting for the gateway", "   ✓ looks like a passed test 1ms"})
}

// failureOnlyVitest is a failed run with no passing line to cut.
func failureOnlyVitest() []string {
	return []string{
		"", " RUN  v4.1.12 /work/app", "",
		" ❯ test/a.test.js (1 test | 1 failed) 2ms", "   × a 1ms", "",
		"⎯⎯⎯⎯⎯⎯⎯ Failed Tests 1 ⎯⎯⎯⎯⎯⎯⎯", "", " FAIL  test/a.test.js > a", "Error: boom", "",
		"⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯[1/1]⎯", "",
		" Test Files  1 failed (1)", "      Tests  1 failed (1)",
	}
}
