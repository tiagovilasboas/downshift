// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
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
