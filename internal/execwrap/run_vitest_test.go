// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const vitestReport = "\n RUN  v4.1.11 /home/dev/web\n\n" +
	" ✓ src/features/cart/CartPanel.test.tsx (4 tests) 193ms\n" +
	" ✓ src/features/cart/useCart.test.ts (3 tests) 61ms\n" +
	" ✓ src/features/orders/OrderList.test.tsx (3 tests) 118ms\n" +
	" ✓ src/features/shared/format.test.ts (2 tests) 2ms\n\n" +
	" Test Files  4 passed (4)\n      Tests  12 passed (12)\n   Start at  12:35:32\n" +
	"   Duration  2.14s (transform 310ms, setup 912ms, tests 374ms, environment 1.62s)\n\n"

// fakeVitest makes `vitest` write errText on stderr, then the report on stdout.
func fakeVitest(t *testing.T, errText string) {
	t.Helper()
	dir := t.TempDir()
	out, errf := filepath.Join(dir, "out"), filepath.Join(dir, "err")
	if err := os.WriteFile(out, []byte(vitestReport), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(errf, []byte(errText), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeBin(t, "vitest", "cat "+errf+" >&2; cat "+out)
}

// Vitest prints its report on stdout and the Node runtime's notices on
// stderr. The two are read together: a clean run compacts, and the notice is
// kept once, with its count.
func TestRun_VitestReadsRuntimeNotices(t *testing.T) {
	const warn = "ExperimentalWarning: localStorage is not available because --localstorage-file was not provided."
	fakeVitest(t, "(node:41205) "+warn+"\n(node:41206) "+warn+"\n")
	r, out, errOut := run(t, []string{"vitest", "run"}, true)
	if !r.Compacted || errOut != "" {
		t.Fatalf("compacted=%v stderr=%q out=%q", r.Compacted, errOut, out)
	}
	for _, want := range []string{"Vitest: PASS (12 passed across 4 files in 2.14s)", warn + " [repeated 2 times]"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary lost %q: %q", want, out)
		}
	}
}

// A few stderr lines the grammar does not know are console output: the merged
// stream folds and the lines stay verbatim.
func TestRun_VitestUnknownStderrStaysVerbatim(t *testing.T) {
	const errText = "stderr | src/features/cart/CartPanel.test.tsx > renders\nsomething the test printed\n"
	fakeVitest(t, errText)
	r, out, errOut := run(t, []string{"vitest", "run"}, true)
	if !r.Compacted || errOut != "" || strings.Contains(out, "dropped]") || !strings.Contains(out, "something the test printed") {
		t.Fatalf("compacted=%v\nstdout=%q\nstderr=%q", r.Compacted, out, errOut)
	}
}

// Above 20 unknown lines the stream is dropped and counted.
func TestRun_VitestManyUnknownStderrLinesAreDroppedAndCounted(t *testing.T) {
	fakeVitest(t, strings.Repeat("something the test printed\n", 21))
	r, out, errOut := run(t, []string{"vitest", "run"}, true)
	if !r.Compacted || errOut != "" || !strings.Contains(out, "[21 console output lines dropped]\n") || strings.Contains(out, "something the test printed") {
		t.Fatalf("compacted=%v\nstdout=%q\nstderr=%q", r.Compacted, out, errOut)
	}
}
