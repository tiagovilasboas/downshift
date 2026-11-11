// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

var (
	rawGoOK    = regexp.MustCompile(`(?m)^ok\s+\S+\s+\S+(?:\s+coverage:\s+(\S+)%)?`)
	rawGoNoTst = regexp.MustCompile(`(?m)^\?\s+\S+\s+\[no test files\]`)
)

// For a clean go test run, the kept form states the counts the raw output
// states, the coverage range of the raw figures, and every line the compressor
// did not recognise, verbatim and in order.
func TestShortenPass_GoTestKeepsCountsCoverageAndStrayLines(t *testing.T) {
	for _, name := range []string{
		"gotest_pass_coverage.txt", "gotest_no_test_files.txt", "gotest_pass_mixed_no_test_files.txt",
		"make_check_clean.txt", "git_commit_hooks_clean.txt", "gotest_pass_plain.txt", "gotest_pass_cached.txt", "gotest_ansi.txt",
	} {
		t.Run(name, func(t *testing.T) {
			raw := readFixture(t, name)
			res := compressor.CompressExit(raw, compressor.ModeSafe, 0)
			if !res.Applied {
				t.Fatalf("fixture not compacted: %s", res.Reason)
			}
			short, ok := compressor.ShortenPass(res.Output)
			if !ok {
				if string(res.Output) != string(short) || strings.Contains(string(res.Output), "\nok ") {
					t.Fatalf("rows left in a summary the kept form skipped:\n%s", res.Output)
				}
				return // header only: already at the floor
			}
			if len(short) >= len(res.Output) {
				t.Fatalf("kept form %d bytes, default %d", len(short), len(res.Output))
			}
			out := string(short)
			oks := rawGoOK.FindAllStringSubmatch(string(raw), -1)
			no := len(rawGoNoTst.FindAllString(string(raw), -1))
			want := fmt.Sprintf("Go test: PASS (%d packages ok", len(oks))
			if no > 0 {
				want += fmt.Sprintf(", %d without tests", no)
			}
			if !strings.HasPrefix(out, want) {
				t.Errorf("counts: got %q, want prefix %q", out, want)
			}
			if lo, hi, any := rawCoverRange(oks); any {
				if !strings.Contains(out, "coverage min "+lo+"% max "+hi+"%") {
					t.Errorf("coverage range %s..%s missing: %q", lo, hi, out)
				}
			} else if strings.Contains(out, "coverage") {
				t.Errorf("coverage invented: %q", out)
			}
			if strings.Contains(out, "\nok ") || strings.Contains(out, "\n? ") {
				t.Errorf("package rows kept: %q", out)
			}
			for _, l := range strings.Split(string(raw), "\n") {
				if l == "" || l == "PASS" || rawGoOK.MatchString(l) || rawGoNoTst.MatchString(l) {
					continue
				}
				if !strings.Contains(out, l+"\n") {
					t.Errorf("unrecognised line lost: %q", l)
				}
			}
		})
	}
}

func rawCoverRange(oks [][]string) (lo, hi string, any bool) {
	var loV, hiV float64
	for _, m := range oks {
		if m[1] == "" {
			continue
		}
		v, _ := strconv.ParseFloat(m[1], 64)
		if !any || v < loV {
			loV, lo = v, m[1]
		}
		if !any || v > hiV {
			hiV, hi = v, m[1]
		}
		any = true
	}
	return lo, hi, any
}

func TestShortenPass_Table(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{"rows with coverage under three packages", "Go test: PASS (2 packages ok) under x.io/a/\nok api 88.0%\nok auth 92.3%\n",
			"Go test: PASS (2 packages ok, coverage min 88.0% max 92.3%)\n"},
		{"no-test rows only", "Go test: PASS (1 packages ok, 2 without tests) under x.io/\n? a [no test files]\nok b\n? c [no test files]\n",
			"Go test: PASS (1 packages ok, 2 without tests)\n"},
		{"stray block survives", "Go test: PASS (1 packages ok) under x.io/\nOther output (kept verbatim):\nWARN deprecated flag\nok b\n",
			"Go test: PASS (1 packages ok)\nOther output (kept verbatim):\nWARN deprecated flag\n"},
		{"pytest timing", "Pytest: PASS (40 passed in 0.03s)\n", "Pytest: PASS (40 passed)\n"},
		{"pytest skipped", "Pytest: PASS (3 passed, 1 skipped in 1.07s)\n", "Pytest: PASS (3 passed, 1 skipped)\n"},
		{"vitest keeps files", "Vitest: PASS (25 passed across 5 files in 412ms)\nstdout | a\nhi\n", "Vitest: PASS (25 passed across 5 files)\nstdout | a\nhi\n"},
		{"clock timing", "dart/flutter test: PASS (3 passed, 1 skipped in 00:02)\n", "dart/flutter test: PASS (3 passed, 1 skipped)\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := compressor.ShortenPass([]byte(tc.in))
			if !ok || string(got) != tc.want {
				t.Fatalf("got %q (%v), want %q", got, ok, tc.want)
			}
			if again, ok := compressor.ShortenPass(got); ok || string(again) != tc.want {
				t.Fatalf("not idempotent: %q", again)
			}
		})
	}
}

// Anything the kept form cannot account for stays whole.
func TestShortenPass_LeavesUnknownShapesWhole(t *testing.T) {
	for name, in := range map[string]string{
		"header only":              "Go test: PASS (12 packages ok)\n",
		"row count differs":        "Go test: PASS (3 packages ok) under x/\nok a\nok b\n",
		"stray row-shaped line":    "Go test: PASS (1 packages ok) under x/\nOther output (kept verbatim):\nok build\nok b\n",
		"no final newline":         "Go test: PASS (2 packages ok) under x/\nok a 1.0%\nok b 2.0%",
		"failure header":           "Go test: FAIL (failures kept verbatim)\nok a\n",
		"not a summary":            "ok  \tx/a\t0.1s\nok  \tx/b\t0.1s\n",
		"timing not at line end":   "PHPUnit: PASS (58 tests, 143 assertions in 00:00.019, 12.00 MB)\n",
		"cypress seconds":          "Cypress: PASS (25 tests: 24 passing; 8 seconds)\n",
		"timing in a later line":   "Jest: PASS (38 passed across 4 suites)\nTime in 2s)\n",
		"empty":                    "",
		"timing without a summary": "took 3 passed in 0.4s)\n",
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := compressor.ShortenPass([]byte(in))
			if ok || string(got) != in {
				t.Fatalf("changed: %q (%v)", got, ok)
			}
		})
	}
}
