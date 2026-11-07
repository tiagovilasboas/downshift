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

var (
	pytestDotsMark    = regexp.MustCompile(`^(?:\S+\.py )?\.+(?: +\[ *\d+%\])?$`)
	pytestVerboseMark = regexp.MustCompile(`^\S+::\S.* PASSED(?: +\[ *\d+%\])?$`)
	pytestSummaryMark = regexp.MustCompile(`^PASSED \S`)
	pytestTestHeader  = regexp.MustCompile(`^_{3,}\s*(.*?)\s*_{3,}$`)
	pytestErrorMark   = regexp.MustCompile(`^E(?:\s.*)?$`)
	pytestFileMark    = regexp.MustCompile(`^\S+:\d+: .*$`)
	pytestSectionMark = regexp.MustCompile(`^=+ (.+?) =+$`)
	pytestFinalMark   = regexp.MustCompile(`^(?:=+ )?.*\b(?:failed|errors?)\b.* in .+`)
	pytestSumFailed   = regexp.MustCompile(`^FAILED\s+(\S+?)(?:\s+-\s+.*)?$`)
	pytestColorEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// pytestNoise builds the predicate of lines cut by the compressor for this input.
func pytestNoise(in []byte) func(string) bool {
	plain := pytestColorEscape.ReplaceAllString(string(in), "")
	lines := strings.Split(strings.TrimSuffix(plain, "\n"), "\n")

	hasFailures := false
	failedTests := map[string]bool{}
	for _, l := range lines {
		if strings.Contains(l, "FAILURES") || strings.Contains(l, "ERRORS") {
			hasFailures = true
		}
		if m := pytestTestHeader.FindStringSubmatch(l); m != nil {
			name := strings.TrimSpace(m[1])
			failedTests[name] = true
			if parts := strings.Split(name, "."); len(parts) > 1 {
				failedTests[parts[len(parts)-1]] = true
			}
		}
	}

	inSummaryScan := false
	hasKeptSummaryLine := false
	for _, l := range lines {
		if inSummaryScan {
			if pytestFinalMark.MatchString(l) || pytestSectionMark.MatchString(l) {
				break
			}
			if pytestSummaryMark.MatchString(l) {
				continue
			}
			if m := pytestSumFailed.FindStringSubmatch(l); m != nil && hasFailures {
				nodeid := m[1]
				parts := strings.Split(nodeid, "::")
				last := parts[len(parts)-1]
				if failedTests[nodeid] || failedTests[last] {
					continue
				}
			}
			hasKeptSummaryLine = true
		} else if m := pytestSectionMark.FindStringSubmatch(l); m != nil && m[1] == "short test summary info" {
			inSummaryScan = true
		}
	}

	inFailBlock := false
	inCaptured := false
	inSummary := false

	return func(l string) bool {
		if m := pytestDotsMark.FindStringSubmatch(l); m != nil {
			return true
		}
		if m := pytestVerboseMark.FindStringSubmatch(l); m != nil {
			return true
		}
		if pytestFinalMark.MatchString(l) {
			inFailBlock = false
			inSummary = false
			return false
		}
		if m := pytestSectionMark.FindStringSubmatch(l); m != nil {
			inFailBlock = false
			inCaptured = false
			if m[1] == "short test summary info" {
				inSummary = true
				return !hasKeptSummaryLine
			}
			return false
		}
		if strings.HasPrefix(l, "___") && strings.HasSuffix(l, "___") {
			inFailBlock = true
			inCaptured = false
			return false
		}
		if inFailBlock {
			if strings.HasPrefix(l, "--") && strings.Contains(l, "Captured ") {
				inCaptured = true
				return false
			}
			if inCaptured {
				return false
			}
			if pytestErrorMark.MatchString(l) || pytestFileMark.MatchString(l) {
				return false
			}
			return true
		}
		if inSummary {
			if pytestSummaryMark.MatchString(l) {
				return true
			}
			if m := pytestSumFailed.FindStringSubmatch(l); m != nil && hasFailures {
				nodeid := m[1]
				parts := strings.Split(nodeid, "::")
				last := parts[len(parts)-1]
				if failedTests[nodeid] || failedTests[last] {
					return true
				}
			}
			return false
		}
		return false
	}
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
		{"pytest_fail.txt", "Pytest: 24 passing tests cut", []string{
			"tests/test_billing.py:42: AssertionError",
			"E       assert 100 == 110",
		}},
		{"pytest_fail_verbose_colour.txt", "Pytest: 5 passing tests cut", []string{
			"E       assert 0 == 30",
			"tests/test_orders.py:21: AssertionError",
		}},
		{"pytest_fail_warnings_rA.txt", "Pytest: 22 passing tests cut", []string{
			"tests/test_cart.py::test_totals",
			"  /home/ci/shop/shop/tax.py:12: DeprecationWarning: round_half_even is deprecated",
			"-- Docs: https://docs.pytest.org/en/stable/how-to/capture-warnings.html",
			"tests/test_checkout.py:31: AssertionError",
		}},
		{"pytest_fail_tb_no.txt", "Pytest: 7 passing tests cut", []string{
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
			assertOnlyNoiseCut(t, in, r.Output, c.summary, pytestNoise(in))
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

func TestCompressFailedPytestQuietMode(t *testing.T) {
	in := []byte("....................                                                     [ 80%]\n..F..                                                                    [100%]\n=================================== FAILURES ===================================\n_________________________________ test_charge __________________________________\nE       assert 100 == 110\n=========================== short test summary info ============================\nFAILED tests/test_billing.py::test_charge - assert 100 == 110\n1 failed, 24 passed in 0.05s\n")
	r := compressor.CompressFailed(in, compressor.FormatPytest)
	if !r.Applied || r.Format != compressor.FormatPytest || len(r.Output) >= len(in) {
		t.Fatalf("not compacted: applied=%v format=%s reason=%q %d -> %d", r.Applied, r.Format, r.Reason, len(in), len(r.Output))
	}
	assertOnlyNoiseCut(t, in, r.Output, "Pytest: 24 passing tests cut", pytestNoise(in))
}

func TestCompressFailedPytestBenchmarkFactPreservation(t *testing.T) {
	raw := "..............................FFFF                                       [100%]\n" +
		"=================================== FAILURES ===================================\n" +
		"________________________________ test_broken_1 _________________________________\n" +
		"\n" +
		"    def test_broken_1():\n" +
		"        values = [calc.add(x, 1) for x in range(5)]\n" +
		">       assert values == [2, 3, 4, 5, 11]\n" +
		"E       assert [1, 2, 3, 4, 5] == [2, 3, 4, 5, 11]\n" +
		"E         At index 0 diff: 1 != 2\n" +
		"E         Use -v to get more diff\n" +
		"\n" +
		"test_fail.py:94: AssertionError\n" +
		"________________________________ test_broken_2 _________________________________\n" +
		"\n" +
		"    def test_broken_2():\n" +
		"        values = [calc.add(x, 1) for x in range(5)]\n" +
		">       assert values == [2, 3, 4, 5, 12]\n" +
		"E       assert [1, 2, 3, 4, 5] == [2, 3, 4, 5, 12]\n" +
		"E         At index 0 diff: 1 != 2\n" +
		"E         Use -v to get more diff\n" +
		"\n" +
		"test_fail.py:98: AssertionError\n" +
		"________________________________ test_broken_3 _________________________________\n" +
		"\n" +
		"    def test_broken_3():\n" +
		"        values = [calc.add(x, 1) for x in range(5)]\n" +
		">       assert values == [2, 3, 4, 5, 13]\n" +
		"E       assert [1, 2, 3, 4, 5] == [2, 3, 4, 5, 13]\n" +
		"E         At index 0 diff: 1 != 2\n" +
		"E         Use -v to get more diff\n" +
		"\n" +
		"test_fail.py:102: AssertionError\n" +
		"_________________________________ test_raises __________________________________\n" +
		"\n" +
		"    def test_raises():\n" +
		">       raise ValueError(\"config missing: db_url\")\n" +
		"E       ValueError: config missing: db_url\n" +
		"\n" +
		"test_fail.py:105: ValueError\n" +
		"=========================== short test summary info ============================\n" +
		"FAILED test_fail.py::test_broken_1 - assert [1, 2, 3, 4, 5] == [2, 3, 4, 5, 11]\n" +
		"FAILED test_fail.py::test_broken_2 - assert [1, 2, 3, 4, 5] == [2, 3, 4, 5, 12]\n" +
		"FAILED test_fail.py::test_broken_3 - assert [1, 2, 3, 4, 5] == [2, 3, 4, 5, 13]\n" +
		"FAILED test_fail.py::test_raises - ValueError: config missing: db_url\n" +
		"======================== 4 failed, 30 passed in 0.05s ==========================\n"

	r := compressor.CompressFailed([]byte(raw), compressor.FormatPytest)
	if !r.Applied {
		t.Fatalf("expected applied=true, reason=%q", r.Reason)
	}
	out := string(r.Output)

	facts := []string{
		"test_broken_1", "test_broken_2", "test_broken_3", "test_raises",
		"[2, 3, 4, 5, 11]", "[2, 3, 4, 5, 12]", "[2, 3, 4, 5, 13]",
		"ValueError: config missing: db_url",
		"test_fail.py:94", "test_fail.py:98", "test_fail.py:102", "test_fail.py:105",
	}
	for _, f := range facts {
		if !strings.Contains(out, f) {
			t.Errorf("missing fact %q in output:\n%s", f, out)
		}
	}

	reduction := float64(len(raw)-len(r.Output)) / float64(len(raw))
	if reduction < 0.40 {
		t.Errorf("expected at least 40%% reduction, got %.1f%% (%d -> %d)", reduction*100, len(raw), len(r.Output))
	}
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
