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
	in := []byte("ok  pkg1 0.1s\nok  pkg2 0.2s\nok  pkg3 0.3s\n")
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

func TestCompress_GoTest_WarningPreserved(t *testing.T) {
	in := []byte("ok  github.com/foo/bar  0.1s\nok  github.com/foo/baz  0.2s\n?   	github.com/foo/qux	[no test files]\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if res.Applied {
		t.Fatal("a non-ok line must keep the original output")
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
	in := []byte("ok  github.com/foo/bar  0.1s\nok  github.com/foo/baz  0.2s\n")
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
