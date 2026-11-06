// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

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
