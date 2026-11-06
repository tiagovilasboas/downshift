// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

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

// TestCompress_Vitest_InjectedLineIsNeverLost injects lines into an otherwise
// clean vitest transcript. A line with a danger word, "warn" or a failure mark
// either keeps the transcript raw or survives in the summary: a hidden failure
// or warning cannot be compacted away.
func TestCompress_Vitest_InjectedLineIsNeverLost(t *testing.T) {
	clean := "\n RUN  v1.6.0 /web\n\n ✓ a.test.ts (5)\n\n Test Files  1 passed (1)\n      Tests  5 passed (5)\n   Start at  12:00:00\n   Duration  350ms (transform 50ms, tests 35ms)\n"
	injected := map[string]string{
		"error":   "stderr | error: connection reset\n",
		"warning": "Warning: deprecated option used\n",
		"panic":   "panic: runtime boom\n",
		"FAIL":    " FAIL  a.test.ts > case\n",
		"failed":  " 1 failed\n",
		// Success-looking lines with free text after the grammar must not vanish.
		"run-free-text":      " RUN  v1.6.0 × a.test.ts > case\n",
		"duration-free-text": "   Duration  1ms (× case)\n",
	}
	for name, line := range injected {
		t.Run(name, func(t *testing.T) {
			in := []byte(strings.Replace(clean, "\n Test Files", "\n"+line+"\n Test Files", 1))
			res := compressor.Compress(in, compressor.ModeSafe)
			if !res.Applied {
				if !bytes.Equal(res.Output, in) {
					t.Fatalf("vitest with injected %s line came back changed without applying", name)
				}
				return
			}
			if !strings.Contains(string(res.Output), strings.TrimRight(line, "\n")) {
				t.Fatalf("vitest with injected %s line lost it: %q", name, res.Output)
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
