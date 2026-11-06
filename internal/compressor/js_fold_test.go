// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// Hand-written runs shaped like npm test, vitest, jest and a JS hook runner.
const (
	vitestNoisyPass = "> app@1.0.0 test\n> vitest run\n\n RUN  v3.2.4 /work/app\n\n" +
		"stdout | src/cart.test.ts > adds an item\nadding sku 42\ncart now has 3 items\n\n" +
		"stdout | src/cart.test.ts > removes an item\nremoved sku 42\n\n" +
		" ✓ src/cart.test.ts (4 tests) 12ms\n ✓ src/price.test.ts (6 tests) 8ms\n\n" +
		" Test Files  2 passed (2)\n      Tests  10 passed (10)\n   Start at  10:00:00\n   Duration  1.20s\n"
	jestNoisyPass = "> app@1.0.0 test\n> jest\n\nPASS src/cart.test.js\n  console.log\n    loading fixtures\n\n      at Object.<anonymous> (src/cart.test.js:3:9)\n\n" +
		"PASS src/price.test.js\nDetermining test suites to run...\n\n" +
		"Test Suites: 2 passed, 2 total\nTests:       8 passed, 8 total\nSnapshots:   0 total\nTime:        1.5 s\nRan all test suites.\n"
	jestFailed = "> app@1.0.0 test\n> jest\n\nDetermining test suites to run...\nPASS src/a.test.js\nPASS src/c.test.js\nPASS src/d.test.js\nPASS src/e.test.js\n  console.log\n    hello\n\n" +
		"FAIL src/b.test.js\n  ● cart › total\n\n    expect(received).toBe(expected)\n\n    Expected: 3\n    Received: 2\n\n" +
		"Test Suites: 1 failed, 1 passed, 2 total\nTests:       1 failed, 3 passed, 4 total\nSnapshots:   0 total\nTime:        1.2 s\n"
)

func TestJSFoldPassKeepsFewConsoleLinesVerbatim(t *testing.T) {
	for _, c := range []struct {
		name, in, head string
		keep           []string
	}{
		{"vitest", vitestNoisyPass, "Vitest: PASS", []string{"adding sku 42\ncart now has 3 items", "removed sku 42"}},
		{"jest", jestNoisyPass, "Jest: PASS", []string{"loading fixtures", "Determining test suites to run..."}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := compressor.Compress([]byte(c.in), compressor.ModeSafe)
			out := string(r.Output)
			if !r.Applied || !strings.HasPrefix(out, c.head) {
				t.Fatalf("not folded: applied=%v reason=%q out=%q", r.Applied, r.Reason, out)
			}
			if strings.Contains(out, "dropped]") {
				t.Errorf("dropped a few console lines:\n%s", out)
			}
			for _, k := range c.keep {
				if !strings.Contains(out, k) {
					t.Errorf("lost %q:\n%s", k, out)
				}
			}
		})
	}
}

func TestJSFoldPassDropsManyConsoleLines(t *testing.T) {
	noise := strings.Repeat("noise line\n", 21)
	in := strings.Replace(vitestNoisyPass, "adding sku 42\n", "adding sku 42\n"+noise+"warn: deprecated cart API\n", 1)
	out := string(compressor.Compress([]byte(in), compressor.ModeSafe).Output)
	if !strings.Contains(out, "console output lines dropped]") || strings.Contains(out, "noise line") {
		t.Errorf("many console lines must fold into a count:\n%s", out)
	}
	if !strings.Contains(out, "warn: deprecated cart API") {
		t.Errorf("lost the warn line:\n%s", out)
	}
}

func TestJSFoldPassKeepsJestHandleAndCoverageLines(t *testing.T) {
	in := "PASS src/a.test.js\n" + strings.Repeat("noise\n", 21) +
		"Jest did not exit one second after the test run\nAll files | 90 | 80 | 70 | 90 |\nFile | % Stmts |\n" +
		"Test Suites: 1 passed, 1 total\nTests: 2 passed, 2 total\n"
	out := string(compressor.Compress([]byte(in), compressor.ModeSafe).Output)
	for _, k := range []string{"Jest did not exit", "File | % Stmts |", "[22 console output lines dropped]"} {
		if !strings.Contains(out, k) {
			t.Errorf("lost %q:\n%s", k, out)
		}
	}
}

func TestJSFoldPassKeepsWarnAndDangerLines(t *testing.T) {
	in := strings.Replace(vitestNoisyPass, "adding sku 42\n", "adding sku 42\nwarn: deprecated cart API\nretry after error code 7\n", 1)
	out := string(compressor.Compress([]byte(in), compressor.ModeSafe).Output)
	for _, k := range []string{"warn: deprecated cart API", "retry after error code 7", "Vitest: PASS"} {
		if !strings.Contains(out, k) {
			t.Errorf("lost %q:\n%s", k, out)
		}
	}
}

func TestJSFoldPassWithoutSummaryStaysRaw(t *testing.T) {
	in := "> app@1.0.0 test\n> vitest run\n\n RUN  v3.2.4 /work/app\n\nstdout | a.test.ts > x\nhello\n\n ✓ a.test.ts (1 test) 2ms\n"
	if r := compressor.Compress([]byte(in), compressor.ModeSafe); r.Applied {
		t.Errorf("folded a run with no Tests summary: %q", r.Output)
	}
}

func TestJSFoldFailedRunKeepsFailuresAndUnknownLines(t *testing.T) {
	r := compressor.CompressFailed([]byte(jestFailed), compressor.FormatJest)
	out := string(r.Output)
	if !r.Applied {
		t.Fatalf("not folded: reason=%q", r.Reason)
	}
	for _, k := range []string{"● cart › total", "Expected: 3", "Received: 2", "Determining test suites to run...", "FAIL src/b.test.js", "Tests:       1 failed"} {
		if !strings.Contains(out, k) {
			t.Errorf("lost %q:\n%s", k, out)
		}
	}
	if strings.Contains(out, "PASS src/a.test.js") {
		t.Errorf("kept a passing file line:\n%s", out)
	}
}

func TestJSFoldVitestFailedRunKeepsUnknownLine(t *testing.T) {
	in := " RUN  v3.2.4 /work/app\n\nsome stray runner line\n ✓ src/a.test.ts (2 tests) 3ms\n ✓ src/c.test.ts (2 tests) 3ms\n ✓ src/d.test.ts (2 tests) 3ms\n ✓ src/e.test.ts (2 tests) 3ms\n ❯ src/b.test.ts (1 test | 1 failed) 5ms\n     × adds 2ms\n\n" +
		"⎯⎯⎯⎯⎯⎯⎯ Failed Tests 1 ⎯⎯⎯⎯⎯⎯⎯\n\n FAIL  src/b.test.ts > adds\nAssertionError: expected 1 to be 2\n\n⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯[1/1]⎯\n\n" +
		" Test Files  1 failed | 1 passed (2)\n      Tests  1 failed | 2 passed (3)\n"
	r := compressor.CompressFailed([]byte(in), compressor.FormatVitest)
	out := string(r.Output)
	if !r.Applied {
		t.Fatalf("not folded: reason=%q", r.Reason)
	}
	for _, k := range []string{"some stray runner line", "AssertionError: expected 1 to be 2", "FAIL  src/b.test.ts > adds"} {
		if !strings.Contains(out, k) {
			t.Errorf("lost %q:\n%s", k, out)
		}
	}
}

func TestRemoteRunKeepsWarnLinesInHookBox(t *testing.T) {
	in := "Enumerating objects: 5, done.\nCounting objects: 100% (5/5), done.\n" +
		"╭──────────────────────────────╮\n│ 🥊 lefthook v1.7.0  hook: pre-push │\n│ WARN: eslint cache is stale        │\n│ 3 problems (0 errors)              │\n│ build ok                           │\n╰──────────────────────────────╯\n" +
		"  ────────────────────────────────────\nsummary: (done in 4.00 seconds)\n✔️ lint (3.10 seconds)\n" +
		"To example.com:acme/widget.git\n   1a2b3c4..5d6e7f8  feat -> feat\n"
	r := compressor.CompressGitRemote([]byte(in), 0)
	out := string(r.Output)
	if !r.Applied {
		t.Fatalf("not compacted: %q", r.Reason)
	}
	for _, k := range []string{"│ WARN: eslint cache is stale", "│ 3 problems (0 errors)", "feat -> feat"} {
		if !strings.Contains(out, k) {
			t.Errorf("lost %q:\n%s", k, out)
		}
	}
	for _, d := range []string{"│ build ok", "lefthook v1.7.0", "╭"} {
		if strings.Contains(out, d) {
			t.Errorf("kept noise %q:\n%s", d, out)
		}
	}
}
