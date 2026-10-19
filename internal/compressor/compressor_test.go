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
