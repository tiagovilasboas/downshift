// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// scriptRun reads one npm or pnpm script transcript of the compressor.
func scriptRun(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", "script_runs", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// npmJestFailure is a failed `npm test` that runs jest: npm prints the script
// banner on stdout, jest its whole report on stderr.
const (
	npmJestBanner  = "\n> web@1.0.0 test\n> jest\n\n"
	npmJestFailure = "PASS src/auth.test.js\nPASS src/billing.test.js\nPASS src/cart.test.js\n" +
		"PASS src/catalog.test.js\nPASS src/checkout.test.js\nPASS src/config.test.js\n" +
		"FAIL src/price.test.js\n  ● price › applies the coupon discount\n\n" +
		"    expect(received).toBe(expected) // Object.is equality\n\n    Expected: 81\n    Received: 90\n\n" +
		"Test Suites: 1 failed, 6 passed, 7 total\nTests:       1 failed, 41 passed, 42 total\n" +
		"Snapshots:   0 total\nTime:        2.104 s\nRan all test suites.\n"
)

// `npm test` and `pnpm test` run vitest or jest. A passing run of either
// compacts; a failing vitest run compacts and keeps every failure; a failing
// jest run, which the failure compressor does not read, stays raw.
func TestRun_NpmTestRunsVitestOrJest(t *testing.T) {
	t.Run("passing vitest", func(t *testing.T) {
		raw := scriptRun(t, "npm_vitest_v3_pass.in")
		fakeBin(t, "npm", emit(raw, false))
		r, out, errOut := run(t, []string{"npm", "test"}, true)
		if !r.Compacted || r.ExitCode != 0 || errOut != "" || out != compactedWant(raw) {
			t.Fatalf("result=%+v stderr=%q out=%q", r, errOut, out)
		}
	})
	t.Run("failing vitest", func(t *testing.T) {
		raw := scriptRun(t, "npm_vitest_v3_fail.in")
		body := compressor.CompressFailed([]byte(raw), compressor.FormatVitest).Output
		cut := strings.Index(raw, "⎯⎯⎯⎯⎯⎯⎯ Failed Tests")
		fakeBin(t, "npm", emit(raw[:cut], false)+gap+emit(raw[cut:], true)+"exit 1")
		r, out, errOut := run(t, []string{"npm", "test"}, true)
		if !r.Compacted || r.ExitCode != 1 || errOut != "" || out != string(body)+Marker(len(raw), len(body)) {
			t.Fatalf("result=%+v stderr=%q out=%q", r, errOut, out)
		}
		for _, l := range []string{" FAIL  src/lib/price.test.ts > price > applies the coupon discount", "AssertionError: expected 90 to be 81 // Object.is equality"} {
			if !strings.Contains(out, l+"\n") {
				t.Errorf("lost failure line %q", l)
			}
		}
	})
	t.Run("passing jest on pnpm", func(t *testing.T) {
		raw := scriptRun(t, "npm_jest_pass.in")
		cut := strings.Index(raw, "PASS ")
		fakeBin(t, "pnpm", emit(raw[:cut], false)+gap+emit(raw[cut:], true))
		r, out, errOut := run(t, []string{"pnpm", "test"}, true)
		if !r.Compacted || r.ExitCode != 0 || errOut != "" || !strings.HasPrefix(out, "Jest: PASS (214 passed across 30 suites)\n") {
			t.Fatalf("result=%+v stderr=%q out=%q", r, errOut, out)
		}
	})
	t.Run("failing jest compacts, keeping every failure", func(t *testing.T) {
		// The failure compressor cuts the six PASS lines and keeps the rest.
		raw := npmJestBanner + npmJestFailure
		body := compressor.CompressFailed([]byte(raw), compressor.FormatJest).Output
		fakeBin(t, "npm", emit(npmJestBanner, false)+gap+emit(npmJestFailure, true)+"exit 1")
		r, out, errOut := run(t, []string{"npm", "test"}, true)
		if !r.Compacted || r.ExitCode != 1 || errOut != "" || out != string(body)+Marker(len(raw), len(body)) {
			t.Fatalf("result=%+v stderr=%q out=%q", r, errOut, out)
		}
		if !strings.Contains(out, "● price › applies the coupon discount\n") {
			t.Errorf("lost failure line in %q", out)
		}
	})
	t.Run("a failure whose runner cannot be told stays raw", func(t *testing.T) {
		// The vitest failure grammar keeps everything after the failure
		// heading verbatim, so it would compact this; a jest summary line
		// means the runner is unknown, and the run stays whole.
		raw := strings.Replace(scriptRun(t, "npm_vitest_v3_fail.in"), " Test Files ", "Test Suites: 1 failed, 1 total\n Test Files ", 1)
		fakeBin(t, "npm", emit(raw, false)+"exit 1")
		r, out, _ := run(t, []string{"npm", "test"}, true)
		if r.Compacted || r.ExitCode != 1 || r.Passthrough != PassthroughNonZeroExit || out != raw {
			t.Fatalf("result=%+v", r)
		}
	})
}
