// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

const embeddedFixture = "make_test_embedded_clean.txt"

// embeddedLog reads the make log fixture. It lives apart from the bench corpus:
// the bench detects a format from the text, while the wrapper folds a make
// transcript with FoldTestRuns.
func embeddedLog(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "embedded_runs", embeddedFixture))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Each complete passing block becomes the runner's own summary; every other
// line of the make log survives verbatim and in order.
func TestFoldEmbeddedRunsFoldsEachBlockKeepsTheRest(t *testing.T) {
	in := embeddedLog(t)
	r := compressor.FoldEmbeddedRuns(in)
	if !r.Applied || r.Format != compressor.FormatGoTestRuns || len(r.Output) >= len(in) {
		t.Fatalf("applied=%v format=%q %d -> %d (%s)", r.Applied, r.Format, len(in), len(r.Output), r.Reason)
	}
	t.Logf("%s: %d -> %d bytes", embeddedFixture, len(in), len(r.Output))
	out := string(r.Output)
	order := []string{
		"go build -o bin/widget ./cmd/widget", "go vet ./...", "go test -v -count=1 ./internal/parse",
		"Go test -v: PASS (", "cd scripts && python -m pytest tests", "Pytest: PASS (18 passed in 0.42s)",
		"make[1]: Leaving directory '/work/widget/scripts'", "cd web && npx vitest run", "Vitest: PASS (",
		"make: Leaving directory '/work/widget'",
	}
	last := -1
	for _, want := range order {
		i := strings.Index(out, want)
		if i < 0 || i < last {
			t.Fatalf("%q lost or out of order:\n%s", want, out)
		}
		last = i
	}
	for _, gone := range []string{"=== RUN", "test session starts", "RUN  v3", "src/lib/cart.test.ts"} {
		if strings.Contains(out, gone) {
			t.Errorf("block line %q was not folded:\n%s", gone, out)
		}
	}
}

// Counts in a summary are the ones the output states.
func TestFoldEmbeddedRunsCopiesCountsFromTheOutput(t *testing.T) {
	out := string(compressor.FoldEmbeddedRuns(embeddedLog(t)).Output)
	for _, want := range []string{"Go test -v: PASS (6 passed, 0 failed, 0 skipped) in 0.021s", "Pytest: PASS (18 passed in 0.42s)",
		"Vitest: PASS (35 passed across 4 files in 1.41s)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// A failure anywhere leaves the whole output as it is.
func TestFoldEmbeddedRunsStaysWhole(t *testing.T) {
	good := string(embeddedLog(t))
	for name, in := range map[string]string{
		"go failure in another package": good + "--- FAIL: TestOther (0.00s)\nFAIL\tgithub.com/acme/widget/internal/other\t0.01s\n",
		"go panic":                      good + "panic: boom\n",
		"pytest failure line":           good + "=========== 1 failed, 17 passed in 0.50s ===========\n",
		"vitest failure summary":        good + " Test Files  1 failed | 3 passed (4)\n",
	} {
		r := compressor.FoldEmbeddedRuns([]byte(in))
		if r.Applied || string(r.Output) != in {
			t.Errorf("%s: applied=%v, output changed (%s)", name, r.Applied, r.Reason)
		}
	}
}

// A truncated block is kept verbatim line by line.
func TestFoldEmbeddedRunsKeepsTruncatedBlockVerbatim(t *testing.T) {
	good := string(embeddedLog(t))
	ok := "ok  \tgithub.com/acme/widget/internal/parse\t0.021s\n"
	in := strings.Replace(good, ok, "", 1)
	r := compressor.FoldEmbeddedRuns([]byte(in))
	if !r.Applied || !strings.Contains(string(r.Output), "=== RUN   TestParseEmpty\n--- PASS: TestParseEmpty (0.00s)\n") {
		t.Fatalf("truncated go block not kept verbatim (applied=%v):\n%s", r.Applied, r.Output)
	}
	if !strings.Contains(string(r.Output), "Pytest: PASS (") {
		t.Errorf("other blocks should still fold:\n%s", r.Output)
	}
	for name, drop := range map[string]string{
		"pytest": "============================== 18 passed in 0.42s ==============================\n",
		"vitest": "   Duration  1.41s (transform 210ms, setup 0ms, collect 0.60s, tests 188ms, environment 2ms, prepare 0.10s)\n",
	} {
		cutIn := strings.Replace(good, drop, "", 1)
		out := string(compressor.FoldEmbeddedRuns([]byte(cutIn)).Output)
		want := map[string]string{"pytest": "tests/test_report.py ....", "vitest": " ✓ src/lib/tax.test.ts (8 tests) 3ms"}[name]
		if !strings.Contains(out, want) {
			t.Errorf("truncated %s block not kept verbatim:\n%s", name, out)
		}
	}
}

// Output of another process inside a block (an unindented line, a pytest
// warning, vitest console output) keeps that block raw and loses nothing.
func TestFoldEmbeddedRunsInterleavedOutputKeepsBlock(t *testing.T) {
	good := string(embeddedLog(t))
	for name, c := range map[string]struct{ after, add, kept string }{
		"go":     {"--- PASS: TestParseHeader (0.00s)\n", "building docs...\n", "building docs..."},
		"go log": {"=== RUN   TestParseHeader\n", "    parse_test.go:12: loaded header\n", "parse_test.go:12: loaded header"},
		"pytest": {"collected 18 items\n", "warning: cache dir missing\n", "warning: cache dir missing"},
		"vitest": {" RUN  v3.2.4 /work/widget/web\n\n", "stdout | src/lib/cart.test.ts\nhello\n\n", "stdout | src/lib/cart.test.ts"},
	} {
		in := strings.Replace(good, c.after, c.after+c.add, 1)
		r := compressor.FoldEmbeddedRuns([]byte(in))
		if !strings.Contains(string(r.Output), c.kept) {
			t.Errorf("%s: interleaved line lost:\n%s", name, r.Output)
		}
		if !r.Applied {
			continue
		}
		if len(r.Output) >= len(in) {
			t.Errorf("%s: longer than raw", name)
		}
	}
}

// No prefix of the log, cut at a line boundary, ever grows.
func TestFoldEmbeddedRunsNeverLongerThanRaw(t *testing.T) {
	lines := strings.SplitAfter(string(embeddedLog(t)), "\n")
	for n := 1; n <= len(lines); n++ {
		in := strings.Join(lines[:n], "")
		if r := compressor.FoldEmbeddedRuns([]byte(in)); len(r.Output) > len(in) {
			t.Fatalf("prefix of %d lines grew: %d -> %d", n, len(in), len(r.Output))
		}
		if r := compressor.FoldTestRuns([]byte(in)); len(r.Output) > len(in) {
			t.Fatalf("FoldTestRuns prefix of %d lines grew", n)
		}
	}
}

// FoldTestRuns still folds plain package rows, and both kinds together.
func TestFoldTestRunsCombinesBlocksAndRows(t *testing.T) {
	rows := string(readFixture(t, "make_check_clean.txt"))
	in := []byte(string(embeddedLog(t)) + rows)
	r := compressor.FoldTestRuns(in)
	if !r.Applied || !strings.Contains(string(r.Output), "Go test -v: PASS (") || !strings.Contains(string(r.Output), "Go test: PASS (") {
		t.Fatalf("applied=%v:\n%s", r.Applied, r.Output)
	}
	if r.OriginalBytes != len(in) || r.ReducedBytes != len(r.Output) {
		t.Errorf("byte counts %d/%d, want %d/%d", r.OriginalBytes, r.ReducedBytes, len(in), len(r.Output))
	}
	plain := compressor.FoldTestRuns([]byte(rows))
	if !plain.Applied {
		t.Errorf("plain rows no longer fold: %s", plain.Reason)
	}
}
