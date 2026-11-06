// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// A passing line whose title carries a danger word is not a plain success
// line: the whole transcript must come back byte-identical with a stable reason.
func TestCompress_Playwright_DangerWordInTitlePreservesRaw(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"bare panic title", "Running 1 test using 1 worker\n\n  ✓  1 panic (1.0s)\n\n  1 passed (1.0s)\n"},
		{"title with warning banner", "Running 1 test using 1 worker\n\n  ✓  1 e2e/a.spec.ts › warning banner shown (1.0s)\n\n  1 passed (1.0s)\n"},
		{"title with error word", "Running 1 test using 1 worker\n\n  ✓  1 e2e/a.spec.ts › error page renders (1.0s)\n\n  1 passed (1.0s)\n"},
		{"title with exception word", "Running 1 test using 1 worker\n\n  ✓  1 e2e/a.spec.ts › exception toast (1.0s)\n\n  1 passed (1.0s)\n"},
		{"title with fatal word", "Running 1 test using 1 worker\n\n  ✓  1 e2e/a.spec.ts › fatal banner hidden (1.0s)\n\n  1 passed (1.0s)\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("must not compact a playwright title with a danger word: reason=%q", res.Reason)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("must preserve raw output byte-identical, got %q", res.Output)
			}
			if res.Reason != "playwright_danger_word_preserved" {
				t.Errorf("reason = %q, want playwright_danger_word_preserved", res.Reason)
			}
		})
	}
}

// A tsc project path whose directory carries a danger word must not compact.
func TestCompress_TSC_DangerWordInDirectoryPreservesRaw(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"error directory project", "[19:20:00] Projects in this build:\n    * error/tsconfig.json\n[19:20:02] Found 0 errors. Watching for file changes.\n"},
		{"panic directory project", "[19:20:00] Projects in this build:\n    * panic/tsconfig.json\n[19:20:02] Found 0 errors. Watching for file changes.\n"},
		{"building error directory", "[19:20:00] Projects in this build:\n    * tsconfig.json\n[19:20:01] Building project '/work/error/tsconfig.json'...\n[19:20:02] Found 0 errors. Watching for file changes.\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("must not compact a tsc path with a danger word: reason=%q", res.Reason)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("must preserve raw output byte-identical, got %q", res.Output)
			}
			if res.Reason != "tsc_danger_word_preserved" {
				t.Errorf("reason = %q, want tsc_danger_word_preserved", res.Reason)
			}
		})
	}
}

// The exact residue inputs named in the plan stay byte-identical.
func TestCompress_PlaywrightTSC_ResidueInputsByteIdentical(t *testing.T) {
	pw := []byte("Running 1 test using 1 worker\n\n  ✓  1 panic (1.0s)\n  ✓  1 e2e/a.spec.ts › warning banner shown (1.0s)\n\n  2 passed (2.0s)\n")
	if res := compressor.Compress(pw, compressor.ModeSafe); res.Applied || !bytes.Equal(res.Output, pw) {
		t.Fatalf("playwright residue input must stay byte-identical: applied=%v reason=%q", res.Applied, res.Reason)
	}
	tsc := []byte("[19:20:00] Projects in this build:\n    * error/tsconfig.json\n[19:20:02] Found 0 errors. Watching for file changes.\n")
	if res := compressor.Compress(tsc, compressor.ModeSafe); res.Applied || !bytes.Equal(res.Output, tsc) {
		t.Fatalf("tsc residue input must stay byte-identical: applied=%v reason=%q", res.Applied, res.Reason)
	}
}
