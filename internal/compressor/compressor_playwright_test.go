// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

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
