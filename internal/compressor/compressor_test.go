// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

func TestCompress_EmptyInput(t *testing.T) {
	res := compressor.Compress([]byte{}, compressor.ModeSafe)
	if res.Applied {
		t.Errorf("expected applied=false for empty input")
	}
	if res.Reason != "empty_input" {
		t.Errorf("expected reason empty_input, got %s", res.Reason)
	}
}

func TestCompress_ModeOff(t *testing.T) {
	in := []byte("ok  pkg1 0.1s\nok  pkg2 0.2s\n")
	res := compressor.Compress(in, compressor.ModeOff)
	if res.Applied {
		t.Errorf("expected applied=false in ModeOff")
	}
	if !bytes.Equal(res.Output, in) {
		t.Errorf("expected output identical to input in ModeOff")
	}
}

func TestCompress_ModeObserve(t *testing.T) {
	in := []byte("ok  github.com/foo/bar/pkg1  0.1s\nok  github.com/foo/bar/pkg2  0.2s\nok  github.com/foo/bar/pkg3  0.3s\n")
	res := compressor.Compress(in, compressor.ModeObserve)
	if !res.Applied {
		t.Fatalf("expected applied=true (detected gain) in ModeObserve")
	}
	// But output MUST remain untouched in observe mode!
	if !bytes.Equal(res.Output, in) {
		t.Errorf("observe mode MUST NOT modify output")
	}
	if !strings.Contains(res.Reason, "observe_mode") {
		t.Errorf("expected reason to mention observe_mode, got %s", res.Reason)
	}
}

func TestCompress_GoTest_PassCompaction(t *testing.T) {
	in := []byte("ok  github.com/foo/bar  0.1s  coverage: 80%\nok  github.com/foo/baz  0.2s  coverage: 90%\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for passing go test")
	}
	if !strings.Contains(string(res.Output), "Go test: PASS (2 packages ok)") {
		t.Errorf("expected compacted summary, got %s", string(res.Output))
	}
	if res.ReducedBytes >= res.OriginalBytes {
		t.Errorf("expected reduction in bytes")
	}
}

func TestCompress_GoTest_FailPreservesRaw(t *testing.T) {
	in := []byte("ok  github.com/foo/bar  0.1s\n--- FAIL: TestSomething (0.01s)\n    panic: runtime error\nFAIL\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if res.Applied {
		t.Errorf("must NOT compress failing tests")
	}
	if !bytes.Equal(res.Output, in) {
		t.Errorf("failing test must be preserved byte-identical")
	}
	if !strings.Contains(res.Reason, "fail_open") {
		t.Errorf("expected fail open reason, got %s", res.Reason)
	}
}

func TestCompress_GitStatus_CleanTree(t *testing.T) {
	in := []byte("On branch main\nYour branch is up to date.\nnothing to commit, working tree clean\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for clean git status")
	}
	if !strings.Contains(string(res.Output), "working tree clean") {
		t.Errorf("expected clean working tree summary, got %s", string(res.Output))
	}
}

func TestCompress_GitStatus_WithChanges_Preserved(t *testing.T) {
	in := []byte("On branch main\nChanges not staged for commit:\n  modified: foo.go\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if res.Applied {
		t.Errorf("status with actual changes should be preserved")
	}
	if !bytes.Equal(res.Output, in) {
		t.Errorf("output must be preserved")
	}
}

func TestCompress_BinaryData_Preserved(t *testing.T) {
	in := []byte{0x00, 0x12, 0xFF, 0xFE}
	res := compressor.Compress(in, compressor.ModeSafe)
	if res.Applied {
		t.Errorf("binary data must never be compressed")
	}
	if res.Reason != "binary_or_invalid_utf8" {
		t.Errorf("expected binary_or_invalid_utf8 reason, got %s", res.Reason)
	}
}

func TestCompress_GoTest_NoTestFilesCompacted(t *testing.T) {
	in := []byte(
		"ok  \tgithub.com/org/repo/pkg/alpha\t0.123s\tcoverage: 85.0% of statements\n" +
			"ok  \tgithub.com/org/repo/pkg/beta\t0.234s\tcoverage: 90.0% of statements\n" +
			"?   \tgithub.com/org/repo/pkg/gamma\t[no test files]\n",
	)
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for suite with packages without tests, got reason=%s", res.Reason)
	}
	out := string(res.Output)
	if !strings.Contains(out, "Go test: PASS (2 packages ok, 1 without tests)") {
		t.Errorf("expected summary with 2 ok and 1 without tests, got %s", out)
	}
	if !strings.Contains(out, "under github.com/org/repo/pkg/") {
		t.Errorf("expected common prefix, got %s", out)
	}
	if !strings.Contains(out, "ok alpha 85.0%") || !strings.Contains(out, "ok beta 90.0%") {
		t.Errorf("expected ok packages with coverage, got %s", out)
	}
	if !strings.Contains(out, "? gamma [no test files]") {
		t.Errorf("expected ? gamma [no test files], got %s", out)
	}
	if res.ReducedBytes >= res.OriginalBytes {
		t.Errorf("expected reduction in bytes: reduced=%d, original=%d", res.ReducedBytes, res.OriginalBytes)
	}
}

func TestCompress_GoTest_NoTestFilesWithFailPreservesRaw(t *testing.T) {
	in := []byte(
		"ok  \tgithub.com/org/repo/pkg/alpha\t0.123s\n" +
			"?   \tgithub.com/org/repo/pkg/gamma\t[no test files]\n" +
			"--- FAIL: TestSomething (0.01s)\n" +
			"FAIL\n",
	)
	res := compressor.Compress(in, compressor.ModeSafe)
	if res.Applied {
		t.Fatal("failing suite with no test files packages must not be compressed")
	}
	if !bytes.Equal(res.Output, in) {
		t.Fatalf("failing test must be preserved byte-identical")
	}
}

func TestCompress_GoTest_WarningPreserved(t *testing.T) {
	in := []byte("ok  github.com/foo/bar  0.1s\nok  github.com/foo/baz  0.2s\nwarning: unexpected compiler warning\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if res.Applied {
		t.Fatal("a non-ok, non-no-test line must keep the original output")
	}
	if !bytes.Equal(res.Output, in) {
		t.Fatalf("output = %q", res.Output)
	}
}

func TestCompress_SearchResults_Preserved(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "file.go:%d:match\n", i)
	}
	in := []byte(b.String())
	res := compressor.Compress(in, compressor.ModeSafe)
	if res.Applied || !bytes.Equal(res.Output, in) {
		t.Fatalf("search output was compacted: applied=%v len=%d", res.Applied, len(res.Output))
	}
}

func TestCompressExit_NonZeroPreservesPassingText(t *testing.T) {
	in := []byte("ok  github.com/foo/bar  0.1s  coverage: 80.0% of statements\nok  github.com/foo/baz  0.2s  coverage: 90.0% of statements\n")
	res := compressor.CompressExit(in, compressor.ModeSafe, 1)
	if res.Applied || !bytes.Equal(res.Output, in) {
		t.Fatalf("nonzero exit was compacted: %+v", res)
	}
	if res.Reason != "nonzero_exit_preserved" {
		t.Fatalf("reason = %s", res.Reason)
	}
	ok := compressor.CompressExit(in, compressor.ModeSafe, 0)
	if !ok.Applied {
		t.Fatal("exit 0 may compact an all-ok suite")
	}
}

func TestCompress_RepetitiveLogs(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 20; i++ {
		b.WriteString("2026-10-08 17:00:00 INFO connection retrying...\n")
	}
	in := []byte(b.String())
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for repetitive logs")
	}
	if !strings.Contains(string(res.Output), "[repeated") {
		t.Errorf("expected repeated indicator in output: %s", string(res.Output))
	}
}

func TestCompress_GitStatus_KeepsTrackingAndDetached(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		applied bool
	}{
		{"up to date", "On branch main\nYour branch is up to date with 'origin/main'.\n\nnothing to commit, working tree clean\n", true},
		{"no upstream", "On branch main\nnothing to commit, working tree clean\n", true},
		{"ahead", "On branch main\nYour branch is ahead of 'origin/main' by 3 commits.\n  (use \"git push\" to publish your local commits)\n\nnothing to commit, working tree clean\n", false},
		{"behind", "On branch main\nYour branch is behind 'origin/main' by 1 commit, and can be fast-forwarded.\n\nnothing to commit, working tree clean\n", false},
		{"detached", "HEAD detached at abc1234\nnothing to commit, working tree clean\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.CompressExit(in, compressor.ModeSafe, 0)
			if res.Applied != tc.applied {
				t.Fatalf("applied = %v, want %v (%s)", res.Applied, tc.applied, res.Reason)
			}
			if !tc.applied && !bytes.Equal(res.Output, in) {
				t.Fatal("output must be byte-identical when not applied")
			}
		})
	}
}

func TestCompress_Pytest_PassCompaction(t *testing.T) {
	in := []byte("============================= test session starts ==============================\nrootdir: /tmp\ncollected 10 items\n\ntest_a.py ..........\n\n============================== 10 passed in 0.42s ==============================\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for passing pytest: %s", res.Reason)
	}
	want := "Pytest: PASS (10 passed in 0.42s)\n"
	if string(res.Output) != want {
		t.Errorf("got %q, want %q", string(res.Output), want)
	}
	if res.Format != compressor.FormatPytest {
		t.Errorf("expected format pytest, got %v", res.Format)
	}
}

func TestCompress_Pytest_PassWithSkipped(t *testing.T) {
	in := []byte("============================= test session starts ==============================\ncollected 12 items\n\ntest_a.py ..........ss\n\n======================== 10 passed, 2 skipped in 1.15s =========================\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for pytest with skipped: %s", res.Reason)
	}
	want := "Pytest: PASS (10 passed, 2 skipped in 1.15s)\n"
	if string(res.Output) != want {
		t.Errorf("got %q, want %q", string(res.Output), want)
	}
}

func TestCompress_Pytest_ErrorsPreserveRaw(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"FAILED line", "=== 5 passed in 0.1s ===\nFAILED test_x.py::test_1\n"},
		{"ERROR line", "=== 5 passed in 0.1s ===\nERROR: fixture error\n"},
		{"Traceback line", "=== 5 passed in 0.1s ===\nTraceback (most recent call last):\n"},
		{"AssertionError line", "=== 5 passed in 0.1s ===\nAssertionError: assert 1 == 2\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("must not compress pytest with error: %s", tc.name)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("must preserve raw output byte-identical")
			}
		})
	}
}

func TestCompress_Vitest_PassCompaction(t *testing.T) {
	in := []byte("\n RUN  v1.6.0 /web\n\n ✓ a.test.ts (2)\n ✓ b.test.ts (3)\n\n Test Files  2 passed (2)\n      Tests  5 passed (5)\n   Start at  12:00:00\n   Duration  350ms (transform 50ms, tests 35ms)\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for passing vitest: %s", res.Reason)
	}
	want := "Vitest: PASS (5 passed across 2 files in 350ms)\n"
	if string(res.Output) != want {
		t.Errorf("got %q, want %q", string(res.Output), want)
	}
	if res.Format != compressor.FormatVitest {
		t.Errorf("expected format vitest, got %v", res.Format)
	}
}

func TestCompress_Vitest_NoDuration(t *testing.T) {
	in := []byte("\n RUN  v1.6.0 /web\n\n ✓ a.test.ts (5)\n\n Test Files  1 passed (1)\n      Tests  5 passed (5)\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for vitest without duration: %s", res.Reason)
	}
	want := "Vitest: PASS (5 passed across 1 files)\n"
	if string(res.Output) != want {
		t.Errorf("got %q, want %q", string(res.Output), want)
	}
}

// TestCompress_Vitest_UnrecognisedLinePreservesRaw injects lines into an
// otherwise clean vitest transcript. Any unrecognised line must keep the whole
// transcript byte-identical, so a hidden failure or warning cannot be compacted.
func TestCompress_Vitest_UnrecognisedLinePreservesRaw(t *testing.T) {
	clean := "\n RUN  v1.6.0 /web\n\n ✓ a.test.ts (5)\n\n Test Files  1 passed (1)\n      Tests  5 passed (5)\n   Start at  12:00:00\n   Duration  350ms (transform 50ms, tests 35ms)\n"
	injected := map[string]string{
		"error":   "stderr | error: connection reset\n",
		"warning": "Warning: deprecated option used\n",
		"panic":   "panic: runtime boom\n",
		"FAIL":    " FAIL  a.test.ts > case\n",
		"failed":  " 1 failed\n",
		// Success-looking lines with free text after the grammar must not pass.
		"run-free-text":      " RUN  v1.6.0 × a.test.ts > case\n",
		"duration-free-text": "   Duration  1ms (× case)\n",
	}
	for name, line := range injected {
		t.Run(name, func(t *testing.T) {
			in := []byte(strings.Replace(clean, "\n Test Files", "\n"+line+"\n Test Files", 1))
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("vitest with injected %s line must not be compacted: reason=%q", name, res.Reason)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("vitest with injected %s line must come back byte-identical", name)
			}
		})
	}
}

func TestCompress_Vitest_ErrorsPreserveRaw(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"FAIL indicator", " RUN  v1.0.0\n FAIL  a.test.ts\n Test Files  1 passed (1)\n      Tests  5 passed (5)\n"},
		{"Error indicator", " RUN  v1.0.0\n Error: connection reset\n Test Files  1 passed (1)\n      Tests  5 passed (5)\n"},
		{"AssertionError indicator", " RUN  v1.0.0\n AssertionError: expected true\n Test Files  1 passed (1)\n      Tests  5 passed (5)\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("must not compress vitest with error: %s", tc.name)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("must preserve raw output byte-identical")
			}
		})
	}
}

func TestCompress_Playwright_PassCompaction(t *testing.T) {
	in := []byte("Running 10 tests using 4 workers\n\n  ✓  1 e2e/a.spec.ts (1.0s)\n  ✓  2 e2e/b.spec.ts (1.2s)\n\n  10 passed (2.2s)\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for passing playwright: %s", res.Reason)
	}
	want := "Playwright: PASS (10 passed in 2.2s)\n"
	if string(res.Output) != want {
		t.Errorf("got %q, want %q", string(res.Output), want)
	}
	if res.Format != compressor.FormatPlaywright {
		t.Errorf("expected format playwright, got %v", res.Format)
	}
}

func TestCompress_Playwright_ErrorsPreserveRaw(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"failed indicator", "Running 10 tests using 4 workers\n  1 failed\n  9 passed (2.5s)\n"},
		{"timeout indicator", "Running 10 tests using 4 workers\n  Error: test timed out\n  10 passed (2.5s)\n"},
		{"retry indicator", "Running 10 tests using 4 workers\n  Retrying test (retry #1)\n  10 passed (2.5s)\n"},
		{"trace attachment", "Running 10 tests using 4 workers\n  trace: test-results/trace.zip\n  10 passed (2.5s)\n"},
		{"flaky indicator", "Running 10 tests using 4 workers\n  1 flaky\n  9 passed (2.5s)\n"},
		{"Error indicator", "Running 10 tests using 4 workers\n  Error: target closed\n  10 passed (2.5s)\n"},
		{"AssertionError indicator", "Running 10 tests using 4 workers\n  AssertionError: expected true\n  10 passed (2.5s)\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("must not compress playwright with error: %s", tc.name)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("must preserve raw output byte-identical")
			}
			if res.Format != compressor.FormatPlaywright {
				t.Errorf("expected format playwright, got %v", res.Format)
			}
		})
	}
}

// TestCompress_Playwright_UnrecognisedLinePreservesRaw injects lines that the
// plain success grammar does not recognise into a clean run. Each transcript
// must come back byte-identical, whatever the injected words are.
func TestCompress_Playwright_UnrecognisedLinePreservesRaw(t *testing.T) {
	const clean = "Running 2 tests using 1 worker\n\n  ✓  1 e2e/a.spec.ts:10:5 › login flow (1.0s)\n  ✓  2 e2e/b.spec.ts (1.2s)\n\n  2 passed (2.2s)\n"
	cases := []struct {
		name string
		in   string
	}{
		{"injected error line", "Running 2 tests using 1 worker\nerror\n\n  ✓  1 e2e/a.spec.ts:10:5 › login flow (1.0s)\n  2 passed (2.2s)\n"},
		{"injected warning line", "Running 2 tests using 1 worker\n\n  ✓  1 e2e/a.spec.ts:10:5 › login flow (1.0s)\nwarning: slow browser\n  2 passed (2.2s)\n"},
		{"injected FAIL line", "Running 2 tests using 1 worker\n\n  ✓  1 e2e/a.spec.ts:10:5 › login flow (1.0s)\nFAIL\n  2 passed (2.2s)\n"},
		{"free text after a passed-test line", "Running 2 tests using 1 worker\n\n  ✓  1 e2e/a.spec.ts:10:5 › login flow (1.0s) retried by hand\n  ✓  2 e2e/b.spec.ts (1.2s)\n\n  2 passed (2.2s)\n"},
		{"free text after the summary line", "Running 2 tests using 1 worker\n\n  ✓  1 e2e/a.spec.ts:10:5 › login flow (1.0s)\n  ✓  2 e2e/b.spec.ts (1.2s)\n\n  2 passed (2.2s) with a note\n"},
		{"free text in a second summary line", clean + "  2 passed (2.2s)\n"},
		{"unrecognised skipped line", "Running 2 tests using 1 worker\n\n  ✓  1 e2e/a.spec.ts:10:5 › login flow (1.0s)\n  -  2 e2e/b.spec.ts (skipped)\n  1 passed (2.2s)\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("must not compact an unrecognised playwright line: reason=%q", res.Reason)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("must preserve raw output byte-identical, got %q", res.Output)
			}
		})
	}
}

func TestCompress_CargoTest_PassCompaction(t *testing.T) {
	in := []byte("   Compiling my_crate v0.1.0 (/home/dev/my_crate)\n    Finished `test` profile [unoptimized + debuginfo] target(s) in 0.5s\n     Running unittests src/lib.rs (target/debug/deps/my_crate-1a2b3c)\n\nrunning 5 tests\ntest tests::test_1 ... ok\n\ntest result: ok. 5 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.15s\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for passing cargo test: %s", res.Reason)
	}
	want := "Cargo test: PASS (5 passed in 0.15s)\n"
	if string(res.Output) != want {
		t.Errorf("got %q, want %q", string(res.Output), want)
	}
	if res.Format != compressor.FormatCargoTest {
		t.Errorf("expected format cargo_test, got %v", res.Format)
	}
}

// TestCompress_CargoTest_UnrecognisedLinePreservesRaw injects lines that a
// clean run never emits into an otherwise passing transcript. Each must keep
// the whole transcript byte-identical, at the start, middle and end.
func TestCompress_CargoTest_UnrecognisedLinePreservesRaw(t *testing.T) {
	clean := []string{
		"   Compiling my_crate v0.1.0 (/home/dev/my_crate)",
		"    Finished `test` profile [unoptimized + debuginfo] target(s) in 0.5s",
		"     Running unittests src/lib.rs (target/debug/deps/my_crate-1a2b3c)",
		"",
		"running 2 tests",
		"test a::t1 ... ok",
		"test a::t2 ... ok",
		"",
		"test result: ok. 2 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.05s",
		"",
	}
	if res := compressor.Compress([]byte(strings.Join(clean, "\n")), compressor.ModeSafe); !res.Applied {
		t.Fatalf("clean transcript must still compact: %s", res.Reason)
	}
	injected := []string{
		"unexpected error in output",
		"unexpected warning in output",
		"unexpected panic in output",
		"unexpected FAIL in output",
	}
	for _, line := range injected {
		for pos := 0; pos <= len(clean); pos++ {
			lines := append(append(append([]string{}, clean[:pos]...), line), clean[pos:]...)
			in := []byte(strings.Join(lines, "\n"))
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("injected %q at %d must not compact: reason=%q", line, pos, res.Reason)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("injected %q at %d must come back byte-identical", line, pos)
			}
		}
	}
}

// TestCompress_CargoTest_ErrorTextInCargoShapedLinePreservesRaw covers lines
// whose shape fits a cargo success pattern but whose free text carries an error
// word. Each must keep the transcript byte-identical.
func TestCompress_CargoTest_ErrorTextInCargoShapedLinePreservesRaw(t *testing.T) {
	clean := []string{
		"   Compiling my_crate v0.1.0 (/home/dev/my_crate)",
		"    Finished `test` profile [unoptimized + debuginfo] target(s) in 0.5s",
		"     Running unittests src/lib.rs (target/debug/deps/my_crate-1a2b3c)",
		"",
		"running 2 tests",
		"test a::t1 ... ok",
		"test a::t2 ... ok",
		"",
		"test result: ok. 2 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.05s",
		"",
	}
	injected := []string{
		"   Compiling x v1 (error: x)",
		"     Running unittests x (error: boom)",
		"test x ... ok ... error: y ... ok",
	}
	for _, line := range injected {
		for pos := 0; pos <= len(clean); pos++ {
			lines := append(append(append([]string{}, clean[:pos]...), line), clean[pos:]...)
			in := []byte(strings.Join(lines, "\n"))
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("line %q at %d must not compact: reason=%q", line, pos, res.Reason)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("line %q at %d must come back byte-identical", line, pos)
			}
		}
	}
}

func TestCompress_CargoTest_ErrorsPreserveRaw(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"FAILED summary", "running 2 tests\ntest t1 ... ok\ntest t2 ... FAILED\n\ntest result: FAILED. 1 passed; 1 failed; finished in 0.05s\n"},
		{"compiler error E", "error[E0425]: cannot find value `foo` in this scope\n --> src/lib.rs:10:5\n"},
		{"compiler warning", "running 2 tests\ntest t1 ... ok\ntest t2 ... ok\nwarning: unused variable: `x`\n\ntest result: ok. 2 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.05s\n"},
		{"panicked at", "thread 'test_panics' panicked at src/lib.rs:20:5:\nassertion failed\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("must not compress cargo test with error: %s", tc.name)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("must preserve raw output byte-identical")
			}
			if res.Format != compressor.FormatCargoTest {
				t.Errorf("expected format cargo_test, got %v", res.Format)
			}
		})
	}
}

func TestCompress_TSC_PassCompaction(t *testing.T) {
	in := []byte("[19:20:00] Projects in this build:\n    * tsconfig.json\n[19:20:01] Building project...\n[19:20:02] Found 0 errors. Watching for file changes.\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for passing tsc: %s", res.Reason)
	}
	want := "TSC: PASS (0 errors)\n"
	if string(res.Output) != want {
		t.Errorf("got %q, want %q", string(res.Output), want)
	}
	if res.Format != compressor.FormatTSC {
		t.Errorf("expected format tsc, got %v", res.Format)
	}
}

func TestCompress_TSC_ErrorsPreserveRaw(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"error TS code", "src/index.ts:1:1 - error TS2322: Type 'string' is not assignable to type 'number'.\nFound 1 error in src/index.ts:1\n"},
		{"Found N errors", "src/utils.ts:5:5 - error TS2304: Cannot find name 'x'.\nFound 2 errors in 1 file.\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("must not compress tsc with error: %s", tc.name)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("must preserve raw output byte-identical")
			}
			if res.Format != compressor.FormatTSC {
				t.Errorf("expected format tsc, got %v", res.Format)
			}
		})
	}
}

func TestCompress_Jest_PassCompaction(t *testing.T) {
	in := []byte("PASS src/auth.test.ts\nPASS src/billing.test.ts\n\nTest Suites: 1 skipped, 2 passed, 3 total\nTests:       1 skipped, 1 todo, 25 passed, 27 total\nSnapshots:   0 total\nTime:        1.234 s\nRan all test suites.\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for passing Jest: %s", res.Reason)
	}
	want := "Jest: PASS (1 skipped, 1 todo, 25 passed across 3 suites, 1 suite skipped)\n"
	if string(res.Output) != want {
		t.Errorf("got %q, want %q", string(res.Output), want)
	}
	if res.Format != compressor.FormatJest {
		t.Errorf("expected format jest, got %v", res.Format)
	}
}

func TestCompress_Jest_ErrorsPreserveRaw(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"failing suite", "FAIL src/auth.test.ts\n\nTest Suites: 1 failed, 1 passed, 2 total\nTests: 1 failed, 25 passed, 26 total\n"},
		{"failing summary", "Test Suites: 1 passed, 1 total\nTests: 1 failed, 24 passed, 25 total\n"},
		{"error output", "PASS src/auth.test.ts\nError: worker crashed\nTest Suites: 1 passed, 1 total\nTests: 25 passed, 25 total\n"},
		{"incomplete summary", "PASS src/auth.test.ts\nTest Suites: 1 passed, 1 total\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("must not compress unsafe Jest output: %s", tc.name)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatal("must preserve raw output byte-identical")
			}
			if res.Format != compressor.FormatJest {
				t.Errorf("expected format jest, got %v", res.Format)
			}
		})
	}
}

func TestCompress_Jest_PreservesUnknownOutput(t *testing.T) {
	in := []byte("PASS src/auth.test.ts\nconsole.log CUSTOMER_ID=12345\nTest Suites: 1 passed, 1 total\nTests: 25 passed, 25 total\n")
	res := compressor.CompressExit(in, compressor.ModeSafe, 0)
	if res.Applied || !bytes.Equal(res.Output, in) {
		t.Fatalf("unknown Jest output must be preserved byte-identically: applied=%v output=%q", res.Applied, res.Output)
	}
}

func TestCompress_PrecedenceOverSearch(t *testing.T) {
	// e2e/auth.spec.ts:10:5 › login flow looks like rg match (file:line:col:)
	playwrightIn := []byte("Running 5 tests using 2 workers\n  ✓ 1 e2e/auth.spec.ts:10:5 › login flow (1.0s)\n  5 passed (2.0s)\n")
	resP := compressor.Compress(playwrightIn, compressor.ModeSafe)
	if resP.Format != compressor.FormatPlaywright {
		t.Fatalf("playwright output with file:line:col must have precedence over search, got %v", resP.Format)
	}

	// [19:20:00] timestamp looks like prefix match
	tscIn := []byte("[19:20:00] Projects in this build:\n    * tsconfig.json\n[19:20:02] Found 0 errors. Watching for file changes.\n")
	resT := compressor.Compress(tscIn, compressor.ModeSafe)
	if resT.Format != compressor.FormatTSC {
		t.Fatalf("tsc output must have precedence over search, got %v", resT.Format)
	}
}
