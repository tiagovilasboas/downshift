// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

// Lines copied from real vitest 3, 4 and 5 runs (colour removed, paths
// generic). Each transcript must fold to exactly the summary given.
func TestCompressVitestRealShapes(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"vitest 3, per-phase durations",
			"\n RUN  v3.2.4 /home/dev/web\n\n ✓ src/App.test.tsx (1 test) 39ms\n\n Test Files  1 passed (1)\n      Tests  1 passed (1)\n" +
				"   Start at  12:35:30\n   Duration  789ms (transform 31ms, setup 80ms, collect 317ms, tests 39ms, environment 233ms, prepare 37ms)\n\n",
			"Vitest: PASS (1 passed across 1 files in 789ms)\n"},
		{"vitest 4, seconds",
			"\n RUN  v4.1.11 /home/dev/web\n\n ✓ src/a.test.tsx (4 tests) 193ms\n ✓ src/b.test.ts (27 tests) 1747ms\n\n" +
				" Test Files  2 passed (2)\n      Tests  31 passed (31)\n   Start at  12:35:32\n" +
				"   Duration  9.14s (transform 3.10s, setup 9.12s, import 9.98s, tests 11.43s, environment 46.62s)\n\n",
			"Vitest: PASS (31 passed across 2 files in 9.14s)\n"},
		{"vitest 5, percent breakdown, skipped file and todo",
			"\n RUN  v5.0.5 /home/dev/web\n\n ↓ src/b.test.ts (2 tests | 2 skipped)\n ✓ src/c.test.ts (2 tests | 1 todo) 1ms\n" +
				" ✓ src/a.test.ts (2 tests | 1 skipped) 1ms\n\n Test Files  2 passed | 1 skipped (3)\n      Tests  2 passed | 3 skipped | 1 todo (6)\n" +
				"   Start at  12:40:55\n   Duration  98ms (transform 66%, import 15%, worker 13%, tests 5%)\n\n",
			"Vitest: PASS (2 passed, 3 skipped, 1 todo across 3 files in 98ms)\n"},
		{"workspace project prefix",
			"\n RUN  v3.2.4 /home/dev/web\n\n ✓ |api| src/a.test.ts (3 tests) 5ms\n ✓ |web| src/b.test.ts (3 tests) 8ms\n\n" +
				" Test Files  2 passed (2)\n      Tests  6 passed (6)\n   Start at  10:00:00\n   Duration  1.20s (transform 200ms, tests 13ms)\n",
			"Vitest: PASS (6 passed across 2 files in 1.20s)\n"},
	} {
		out, applied, reason := compressVitest([]byte(tc.in))
		if !applied || string(out) != tc.want {
			t.Errorf("%s: applied=%v reason=%q\n got %q\nwant %q", tc.name, applied, reason, out, tc.want)
		}
	}
}

// The Node and jsdom notices a clean run prints are kept: each distinct
// notice once, in the order first seen, with its count. The process id does
// not make a notice distinct.
func TestCompressVitestKeepsEveryNoticeOnce(t *testing.T) {
	const warn = " ExperimentalWarning: localStorage is not available because --localstorage-file was not provided."
	const hint = "(Use `node --trace-warnings ...` to show where the warning was created)"
	var b strings.Builder
	b.WriteString("(node:11) [DEP0205] DeprecationWarning: `module.register()` is deprecated.\n(Use `node --trace-deprecation ...` to show where the warning was created)\n")
	b.WriteString("\n RUN  v4.1.11 /home/dev/web\n\n")
	for i, pid := range []string{"(node:21)", "(node:22)", "(node:23)"} {
		b.WriteString(pid + warn + "\n" + hint + "\n")
		b.WriteString(" ✓ src/f" + string(rune('a'+i)) + ".test.ts (2 tests) 3ms\n")
	}
	b.WriteString("Not implemented: navigation to another Document\n")
	b.WriteString("\n Test Files  3 passed (3)\n      Tests  6 passed (6)\n   Start at  12:00:00\n   Duration  1.10s (transform 300ms, tests 9ms)\n")
	out, applied, reason := compressVitest([]byte(b.String()))
	want := "Vitest: PASS (6 passed across 3 files in 1.10s)\n" +
		"(node:11) [DEP0205] DeprecationWarning: `module.register()` is deprecated.\n" +
		"(Use `node --trace-deprecation ...` to show where the warning was created)\n" +
		"(node:21)" + warn + " [repeated 3 times]\n" +
		hint + " [repeated 3 times]\n" +
		"Not implemented: navigation to another Document\n"
	if !applied || string(out) != want {
		t.Fatalf("applied=%v reason=%q\n got %q\nwant %q", applied, reason, out, want)
	}
}

// Anything outside the grammar keeps the transcript raw: console output a
// test printed, a warning that is not the runtime's, a second run, a failed
// count, free text after a report line.
// Unknown lines no longer keep a run raw (see js_fold_test.go); a second run,
// a doubled summary or a missing Tests summary still do.
func TestCompressVitestKeepsUnknownShapesRaw(t *testing.T) {
	const head = "\n RUN  v5.0.5 /home/dev/web\n\n ✓ src/a.test.ts (2 tests) 1ms\n"
	const tail = "\n Test Files  1 passed (1)\n      Tests  2 passed (2)\n   Start at  12:00:00\n   Duration  98ms (transform 66%, tests 5%)\n"
	for name, in := range map[string]string{
		"second run":       head + tail + head + tail,
		"no tests summary": head + "\n Test Files  1 passed (1)\n",
	} {
		out, applied, _ := compressVitest([]byte(in))
		if applied || string(out) != in {
			t.Errorf("%s: applied=%v, want the input back unchanged", name, applied)
		}
	}
}
