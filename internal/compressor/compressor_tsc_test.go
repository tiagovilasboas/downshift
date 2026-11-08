// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

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

// TestCompress_TSC_UnrecognisedLinePreservesRaw covers injected lines that the
// plain tsc success grammar does not recognise: each must return the input
// byte-identical, even when the run otherwise ends with the 0-errors summary.
func TestCompress_TSC_UnrecognisedLinePreservesRaw(t *testing.T) {
	const head = "[19:20:00] Projects in this build:\n    * tsconfig.json\n"
	const tail = "[19:20:02] Found 0 errors. Watching for file changes.\n"
	cases := []struct {
		name string
		in   string
	}{
		{"injected error word", head + "unexpected error in output\n" + tail},
		{"injected warning word", head + "unexpected warning in output\n" + tail},
		{"injected FAIL word", head + "unexpected FAIL in output\n" + tail},
		{"injected panic word", head + "unexpected panic in output\n" + tail},
		{"injected unknown line", head + "zz-unrecognised-line 42\n" + tail},
		{"project line with spaces", "[19:20:00] Projects in this build:\n    * unexpected error in output\n" + tail},
		// Colour alone no longer keeps a transcript raw (colour.go); any other
		// terminal escape still does.
		{"line erase before the summary", head + "[19:20:02] \x1b[2KFound 0 errors. Watching for file changes.\n"},
		{"no summary line", head},
		{"project named error.json", "[19:20:00] Projects in this build:\n    * error.json\n" + tail},
		{"building panic project", "[19:20:00] Projects in this build:\n    * tsconfig.json\n[19:20:01] Building project 'panic'...\n" + tail},
		{"project tsconfig.panic.json", "[19:20:00] Projects in this build:\n    * tsconfig.panic.json\n" + tail},
		{"project tsconfig.error.json", "[19:20:00] Projects in this build:\n    * tsconfig.error.json\n" + tail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("must not compact tsc with unrecognised line: %s", res.Reason)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("must preserve raw output byte-identical, got %q", res.Output)
			}
		})
	}
}
