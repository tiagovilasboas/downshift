// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// Large test-runner output: multi-package go test -v, go test -json, package
// scripts that run jest or vitest, and a failed make. Fixtures are synthetic
// (testdata/big_output).

func readBig(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "big_output", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// assertKeepsLines fails when a line of in that matches want is missing from out.
func assertKeepsLines(t *testing.T, in, out []byte, want func(string) bool) {
	t.Helper()
	have := map[string]bool{}
	for _, l := range strings.Split(string(out), "\n") {
		have[l] = true
	}
	kept := 0
	for _, l := range strings.Split(string(in), "\n") {
		if want(l) {
			kept++
			if !have[l] {
				t.Errorf("line lost: %q", l)
			}
		}
	}
	if kept == 0 {
		t.Fatal("no line matched: the check proves nothing")
	}
}

func TestBigOutput_GoVerboseMultiPackage(t *testing.T) {
	in := readBig(t, "go_v_multi_clean.in")
	res := compressor.CompressExit(in, compressor.ModeSafe, 0)
	if !res.Applied || res.Format != compressor.FormatGoTest {
		t.Fatalf("multi-package go test -v must fold: applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
	}
	out := string(res.Output)
	if len(res.Output)*5 > len(in) {
		t.Errorf("saving below 80%%: %d -> %d", len(in), len(res.Output))
	}
	for _, want := range []string{"Go test -v: PASS (5 packages ok", "ok cart", "ok pricing", "ok orders", "ok catalog", "ok billing",
		"? cmd/shop [no test files]", "retry after timeout, second attempt ok"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary lacks %q:\n%s", want, out)
		}
	}
	// Only the one test whose log holds a danger word ("timeout") keeps its block.
	if strings.Count(out, "=== RUN") != 2 || strings.Contains(out, "=== PAUSE") {
		t.Errorf("passing test scaffolding must fold:\n%s", out)
	}
}

func TestBigOutput_GoVerboseFailureKeepsEveryFailLine(t *testing.T) {
	in := readBig(t, "go_v_multi_fail.in")
	signal := func(l string) bool {
		return strings.Contains(l, "FAIL") || strings.Contains(l, "got 3, want 4")
	}
	// A non-zero exit goes through the failure compressor.
	res := compressor.CompressFailed(in, compressor.FormatGoTest)
	if !res.Applied {
		t.Fatalf("failed run must compact: %q", res.Reason)
	}
	assertKeepsLines(t, in, res.Output, signal)
	// Tests that logged keep their header lines, so the saving here is modest.
	if len(res.Output)*4 > len(in)*3 {
		t.Errorf("saving below 25%%: %d -> %d", len(in), len(res.Output))
	}
	// A zero exit with a FAIL in the text is never summarised.
	if r := compressor.CompressExit(in, compressor.ModeSafe, 0); r.Applied || !bytes.Equal(r.Output, in) {
		t.Errorf("FAIL at exit 0 must stay raw: applied=%v reason=%q", r.Applied, r.Reason)
	}
}

func TestBigOutput_GoVerboseUnknownStretchStaysVerbatim(t *testing.T) {
	in := string(readBig(t, "go_v_multi_clean.in")) +
		"--- PASS: TestGhost (0.00s)\nPASS\nok  \texample.com/shop/ghost\t0.01s\n"
	res := compressor.CompressExit([]byte(in), compressor.ModeSafe, 0)
	if !res.Applied {
		t.Fatalf("other packages must still fold: %q", res.Reason)
	}
	if !strings.Contains(string(res.Output), "--- PASS: TestGhost (0.00s)") ||
		!strings.Contains(string(res.Output), "ok  \texample.com/shop/ghost\t0.01s") {
		t.Errorf("a stretch the parser cannot account for must stay verbatim:\n%s", res.Output)
	}
	if !strings.Contains(string(res.Output), "other stretches kept verbatim") {
		t.Errorf("header must not claim a full PASS:\n%s", res.Output)
	}
}

func TestBigOutput_GoJSON(t *testing.T) {
	in := readBig(t, "go_json_clean.in")
	res := compressor.CompressExit(in, compressor.ModeSafe, 0)
	if !res.Applied || res.Format != compressor.FormatGoTest {
		t.Fatalf("clean go test -json must fold: applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
	}
	out := string(res.Output)
	for _, want := range []string{"Go test -json: PASS (3 packages ok, 30 passed", "ok example.com/shop/cart 0.05s (10 passed)",
		"ok example.com/shop/orders 0.05s (12 passed)"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary lacks %q:\n%s", want, out)
		}
	}
	if len(res.Output)*10 > len(in) {
		t.Errorf("saving below 90%%: %d -> %d", len(in), len(res.Output))
	}
}

func TestBigOutput_GoJSONFailure(t *testing.T) {
	in := readBig(t, "go_json_fail.in")
	res := compressor.CompressFailed(in, compressor.FormatGoTest)
	if !res.Applied {
		t.Fatalf("failed go test -json must compact: %q", res.Reason)
	}
	assertKeepsLines(t, in, res.Output, func(l string) bool {
		return strings.Contains(l, `shop/orders","Test":"TestCase5"`) || strings.Contains(l, `"Action":"fail"`) ||
			strings.Contains(l, "got 3, want 4")
	})
	if len(res.Output)*2 > len(in) {
		t.Errorf("saving below 50%%: %d -> %d", len(in), len(res.Output))
	}
	if r := compressor.CompressExit(in, compressor.ModeSafe, 0); r.Applied {
		t.Errorf("a failed action at exit 0 must stay raw: %q", r.Reason)
	}
	// A line that is not an event keeps the stream raw.
	mixed := append(append([]byte{}, in...), []byte("panic: stray line\n")...)
	if r := compressor.CompressFailed(mixed, compressor.FormatGoTest); r.Applied {
		t.Errorf("mixed stream must stay raw: %q", r.Reason)
	}
}

func TestBigOutput_PackageScripts(t *testing.T) {
	tests := []struct {
		name, file string
		want, none []string
	}{
		{"pnpm workspace vitest", "pnpm_ws_vitest_pass.in",
			[]string{"Workspace: packages/web test", "Vitest: PASS (150 passed", "Error: invalid input (expected in this test)"},
			[]string{"packages/web test:", "✓ src/f1.test.ts"}},
		{"yarn jest", "yarn_jest_pass.in",
			[]string{"Jest: PASS (88 passed across 20 suites)", "Error: simulated network failure (expected)"},
			[]string{"yarn run", "Done in"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := readBig(t, tc.file)
			res := compressor.CompressExit(in, compressor.ModeSafe, 0)
			if !res.Applied {
				t.Fatalf("must compact: format=%s reason=%q", res.Format, res.Reason)
			}
			out := string(res.Output)
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("output lacks %q:\n%s", w, out)
				}
			}
			for _, w := range tc.none {
				if strings.Contains(out, w) {
					t.Errorf("output still has %q:\n%s", w, out)
				}
			}
		})
	}
}

func TestBigOutput_PackageScriptsStayRaw(t *testing.T) {
	failedSummary := "> web@1.0.0 test\n> vitest run\n\n RUN  v2.1.4 /w\n\n ✓ src/a.test.ts (3 tests) 2ms\n ❯ src/b.test.ts (2 tests | 1 failed) 4ms\n\n" +
		" Test Files  1 failed | 1 passed (2)\n      Tests  1 failed | 4 passed (5)\n   Duration  1.00s\n"
	twoWorkspaces := "packages/a test: > vitest run\npackages/b test: > vitest run\npackages/a test:  Tests  5 passed (5)\npackages/b test:  Tests  3 passed (3)\n"
	for name, in := range map[string]string{"failed summary at exit 0": failedSummary, "two workspaces": twoWorkspaces} {
		res := compressor.CompressExit([]byte(in), compressor.ModeSafe, 0)
		if res.Applied || string(res.Output) != in {
			t.Errorf("%s must stay raw: applied=%v reason=%q", name, res.Applied, res.Reason)
		}
	}
}

func TestBigOutput_MakeFailure(t *testing.T) {
	in := readBig(t, "make_go_test_fail.in")
	res := compressor.CompressFailed(in, compressor.FormatGoTestRuns)
	if !res.Applied {
		t.Fatalf("failed make with a go test failure must compact: %q", res.Reason)
	}
	assertKeepsLines(t, in, res.Output, func(l string) bool {
		return strings.Contains(l, "FAIL") || strings.HasPrefix(l, "make:") || strings.Contains(l, "got 3, want 4") ||
			strings.HasPrefix(l, "go ")
	})
	if len(res.Output) >= len(in) {
		t.Errorf("not shorter: %d >= %d", len(res.Output), len(in))
	}
	// A failed make with no go test failure has nothing provably noise: raw.
	cc := readBig(t, "make_compile_fail.in")
	if r := compressor.CompressFailed(cc, compressor.FormatGoTestRuns); r.Applied || !bytes.Equal(r.Output, cc) {
		t.Errorf("compile failure must stay raw: %q", r.Reason)
	}
	// No make failure line: not a transcript this compactor knows.
	if r := compressor.CompressFailed([]byte("FAIL\nx\n"), compressor.FormatGoTestRuns); r.Applied {
		t.Errorf("unknown shape must stay raw")
	}
}
