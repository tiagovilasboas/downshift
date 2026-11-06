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

// pytestDotsMark is a progress line of passed tests only, with or without its
// path and percentage. pytestVerboseMark is a -v line of a passed test, and
// pytestSummaryMark a PASSED line of the short test summary (-rA or -rP).
var (
	pytestDotsMark    = regexp.MustCompile(`^(?:\S+\.py )?\.+(?: +\[ *\d+%\])?$`)
	pytestVerboseMark = regexp.MustCompile(`^\S+::\S.* PASSED(?: +\[ *\d+%\])?$`)
	pytestSummaryMark = regexp.MustCompile(`^PASSED \S`)
)

func pytestNoise(l string) bool {
	return pytestDotsMark.MatchString(l) || pytestVerboseMark.MatchString(l) || pytestSummaryMark.MatchString(l)
}

// Real-looking pytest runs (testdata/corpus): the default dot progress, -v with
// colour, -rA with a warnings summary, and --tb=no. Each keeps every failure
// line and cuts only the lines of passed tests; the summary states the count.
func TestCompressFailedPytestCutsOnlyPassedTests(t *testing.T) {
	cases := []struct {
		file    string
		summary string
		failure []string
	}{
		{"pytest_fail.txt", "Pytest: 20 passing tests cut", []string{
			"tests/test_billing.py:42: AssertionError",
			"E       assert 100 == 110",
			"FAILED tests/test_billing.py::test_charge - assert 100 == 110",
		}},
		{"pytest_fail_verbose_colour.txt", "Pytest: 5 passing tests cut", []string{
			"FAILED tests/test_orders.py::test_refund - assert 0 == 30",
			"E       assert 0 == 30",
			"tests/test_orders.py:21: AssertionError",
		}},
		{"pytest_fail_warnings_rA.txt", "Pytest: 17 passing tests cut", []string{
			"tests/test_cart.py::test_totals",
			"  /home/ci/shop/shop/tax.py:12: DeprecationWarning: round_half_even is deprecated",
			"-- Docs: https://docs.pytest.org/en/stable/how-to/capture-warnings.html",
			"FAILED tests/test_checkout.py::test_pay_declines_bad_card - AssertionError: assert 'approved' == 'declined'",
		}},
		{"pytest_fail_tb_no.txt", "Pytest: 4 passing tests cut", []string{
			"FAILED tests/test_billing.py::test_charge - assert 100 == 110",
		}},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			in := readFixture(t, c.file)
			r := compressor.CompressFailed(in, compressor.FormatPytest)
			if !r.Applied || r.Format != compressor.FormatPytest || len(r.Output) >= len(in) {
				t.Fatalf("not compacted: applied=%v format=%s reason=%q %d -> %d", r.Applied, r.Format, r.Reason, len(in), len(r.Output))
			}
			assertOnlyNoiseCut(t, in, r.Output, c.summary, pytestNoise)
			assertContains(t, r.Output, c.failure)
		})
	}
}

// A run where every test failed has nothing to cut, and a --tb=line run prints
// a traceback line the grammar does not know: both stay raw.
func TestCompressFailedPytestStaysRawWhenNothingCutOrUnknown(t *testing.T) {
	assertRaw(t, compressor.FormatPytest, readFixture(t, "pytest_fail_all.txt"), "pytest_failed_run_no_gain")
	assertRaw(t, compressor.FormatPytest, readFixture(t, "pytest_fail_tb_line.txt"), "pytest_failed_run_unrecognized_line_preserved")
}

// Each input is a run the grammar cannot read in full, or one with no failure
// signal: the output must come back byte for byte.
func TestCompressFailedPytestFailsOpen(t *testing.T) {
	in := readFixture(t, "pytest_fail.txt")
	cases := []struct {
		name   string
		input  []byte
		reason string
	}{
		{"CRLF", []byte(strings.ReplaceAll(string(in), "\n", "\r\n")), "pytest_failed_run_raw_preserved"},
		{"no final newline", bytes.TrimSuffix(in, []byte("\n")), "pytest_failed_run_raw_preserved"},
		{"cursor escape", mutate(t, in, "tests/test_auth.py", "\x1b[2Ktests/test_auth.py"), "pytest_failed_run_raw_preserved"},
		{"unknown line in the progress", mutate(t, in, "collected 25 items\n", "collected 25 items\nsomething odd\n"), "pytest_failed_run_unrecognized_line_preserved"},
		{"no final failure summary", mutate(t, in, "1 failed, 24 passed", "1 passed, 24 passed"), "pytest_failed_run_missing_summary"},
		{"no failure signal", []byte("=========== 3 passed in 0.01s ===========\n"), "pytest_failed_run_no_failure_section"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRaw(t, compressor.FormatPytest, c.input, c.reason)
		})
	}
}
