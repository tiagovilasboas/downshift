// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// npm test, pnpm test and vitest runs, passing and failing. The fixtures under
// testdata/script_runs are synthetic reproductions of the shapes (vitest 1, 2
// and 3, npm and pnpm banners) plus two real vitest 3 runs through npm test
// with their paths made relative. Each must shrink by a floor and keep every
// failure line and the summary.

const scriptRunsDir = "script_runs"

func reduction(in, out []byte) float64 {
	return float64(len(in)-len(out)) / float64(len(in)) * 100
}

func TestScriptRunsCleanShrink(t *testing.T) {
	cases := []struct {
		name  string
		floor float64
		keep  []string
	}{
		{"npm_vitest_v3_real_pass", 70, []string{"Vitest: PASS (2 passed across 1 files in "}},
		{"npm_vitest_v3_pass", 90, []string{"Vitest: PASS (", "passed across 40 files in 3.41s)"}},
		{"pnpm_vitest_v3_pass", 90, []string{"passed, 2 skipped across 33 files in 2.87s)"}},
		{"npm_vitest_v1_pass", 85, []string{"across 36 files in 2.12s)", "npm warn Unknown project config"}},
		{"vitest_v2_pass", 85, []string{"across 28 files in 1.94s)"}},
		{"npm_jest_pass", 70, []string{"Jest: PASS (214 passed across 30 suites)"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := readFailedFixture(t, scriptRunsDir, tc.name)
			r := compressor.CompressExit(in, compressor.ModeSafe, 0)
			if !r.Applied {
				t.Fatalf("not compacted: format=%s reason=%q", r.Format, r.Reason)
			}
			if got := reduction(in, r.Output); got < tc.floor {
				t.Errorf("reduction %.1f%% (%d -> %d), want at least %.0f%%", got, len(in), len(r.Output), tc.floor)
			}
			for _, k := range tc.keep {
				if !strings.Contains(string(r.Output), k) {
					t.Errorf("lost %q in %q", k, r.Output)
				}
			}
			if strings.Contains(string(r.Output), "\n> ") || strings.HasPrefix(string(r.Output), "> ") {
				t.Errorf("script banner survived: %q", r.Output)
			}
			t.Logf("%s: %d -> %d bytes (%.1f%%)", tc.name, len(in), len(r.Output), reduction(in, r.Output))
		})
	}
}

// scriptRunNoise marks what a failed run may lose: the script banner ahead of
// the RUN header and the check-mark lines before the failure report.
func scriptRunNoise(in []byte) func(string) bool {
	passing := vitestNoise(in)
	started := false
	return func(l string) bool {
		if strings.Contains(l, " RUN  v") {
			started = true
		}
		return (!started && strings.HasPrefix(l, "> ")) || passing(l)
	}
}

func TestScriptRunsFailedShrinkKeepsFailures(t *testing.T) {
	summary := func(files, tests string) []string {
		return []string{" Test Files  " + files, "      Tests  " + tests}
	}
	cases := []struct {
		name  string
		floor float64
		keep  []string
	}{
		{"npm_vitest_v3_real_fail", 5, append(summary("1 failed | 1 passed (2)", "2 failed | 3 passed (5)"), []string{
			" FAIL  test/bad.test.js > price > discount",
			"AssertionError: expected 90 to be 81 // Object.is equality",
			" ❯ test/bad.test.js:4:44", " ❯ test/bad.test.js:5:30",
			"⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯[2/2]⎯",
		}...)},
		{"npm_vitest_v3_fail", 35, append(summary("1 failed | 34 passed (35)", "2 failed |"), []string{
			" FAIL  src/lib/price.test.ts > price > applies the coupon discount",
			" ❯ src/lib/price.test.ts:18", " ❯ src/lib/price.test.ts:31",
			"-     3,", "+     2,", "- 81", "+ 90",
			"      at /work/web/src/lib/price.test.ts:18:20",
		}...)},
		{"pnpm_vitest_v3_fail", 35, append(summary("1 failed | 34 passed (35)", "2 failed |"), []string{
			" FAIL  src/lib/price.test.ts > price > sums the cart",
			"     → expected 90 to be 81 // Object.is equality",
			"      at processTicksAndRejections (node:internal/process/task_queues:95:5)",
			" ELIFECYCLE  Test failed. See above for more details.",
		}...)},
		{"vitest_v2_fail", 35, append(summary("1 failed | 34 passed (35)", "2 failed |"), []string{
			" FAIL  test/price.test.ts > price > applies the coupon discount",
			"   × price > sums the cart 3ms",
		}...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, r := goldenFailedRun(t, scriptRunsDir, compressor.FormatVitest, tc.name)
			assertOnlyNoiseCut(t, in, r.Output, "Vitest: ", scriptRunNoise(in))
			for _, k := range tc.keep {
				if !strings.Contains(string(r.Output), k) {
					t.Errorf("lost %q", k)
				}
			}
			if got := reduction(in, r.Output); got < tc.floor {
				t.Errorf("reduction %.1f%% (%d -> %d), want at least %.0f%%", got, len(in), len(r.Output), tc.floor)
			}
			t.Logf("%s: %d -> %d bytes (%.1f%%)", tc.name, len(in), len(r.Output), reduction(in, r.Output))
		})
	}
}

// A "> " line after the runner started is test output, not a banner, and a
// failed run keeps the exit code's verdict: CompressExit never touches it.
func TestScriptRunsFailOpen(t *testing.T) {
	pass := readFailedFixture(t, scriptRunsDir, "npm_vitest_v3_pass")
	late := mutate(t, pass, " ✓ src/lib/auth", "> late banner\n ✓ src/lib/auth")
	if r := compressor.CompressExit(late, compressor.ModeSafe, 0); !r.Applied || !strings.Contains(string(r.Output), "> late banner") {
		t.Errorf("banner line after the run header is test output, kept verbatim: %q", r.Reason)
	}
	fail := readFailedFixture(t, scriptRunsDir, "npm_vitest_v3_fail")
	lateFail := mutate(t, fail, " ✓ src/lib/cart", "> late banner\n ✓ src/lib/cart")
	// An unknown line before the failures is kept, never a reason to stay raw.
	if r := compressor.CompressFailed(lateFail, compressor.FormatVitest); !r.Applied || !strings.Contains(string(r.Output), "> late banner") {
		t.Errorf("late banner line must be kept in a folded failed run: applied=%v", r.Applied)
	}
	if r := compressor.CompressExit(fail, compressor.ModeSafe, 1); r.Applied || !bytes.Equal(r.Output, fail) {
		t.Error("a non-zero exit must stay raw through CompressExit")
	}
}
