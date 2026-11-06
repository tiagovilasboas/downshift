// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

func TestCompressGrepTable(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantApplied bool
		wantOut     string
		wantReason  string
	}{
		{
			name: "matches are grouped by file, line numbers kept, leading whitespace trimmed",
			input: lines(
				"internal/compressor/apply.go:12:\tfunc applyFormatCompression(input []byte, f Format) {",
				"internal/compressor/apply.go:40:    return compressLs(input)",
				"internal/compressor/types.go:7:type Format string",
			),
			wantApplied: true,
			wantOut: lines(
				"internal/compressor/apply.go",
				" 12: func applyFormatCompression(input []byte, f Format) {",
				" 40: return compressLs(input)",
				"internal/compressor/types.go",
				" 7: type Format string",
			),
			wantReason: "grep matches grouped by file",
		},
		{
			name: "a match that names a danger word is kept",
			input: lines(
				"internal/compressor/errors_test.go:3:\tpanic(\"boom\")",
				"internal/compressor/errors_test.go:9:\tt.Fatal(\"error: fail\")",
			),
			wantApplied: true,
			wantOut: lines(
				"internal/compressor/errors_test.go",
				" 3: panic(\"boom\")",
				" 9: t.Fatal(\"error: fail\")",
			),
			wantReason: "grep matches grouped by file",
		},
		{
			name:        "context row with a dash separator stays raw",
			input:       "internal/compressor/apply.go:12:func a() {}\ninternal/compressor/apply.go-13-\n",
			wantApplied: false,
			wantReason:  "grep_unrecognized_structure_preserved",
		},
		{
			name:        "binary-file notice stays raw",
			input:       "internal/compressor/apply.go:12:func a() {}\nBinary file internal/x.bin matches\n",
			wantApplied: false,
			wantReason:  "grep_unrecognized_structure_preserved",
		},
		{
			name:        "single short match has no gain",
			input:       "a:1:x\n",
			wantApplied: false,
			wantReason:  "no_compression_gain",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, applied, reason := compressGrep([]byte(tc.input))
			if applied != tc.wantApplied || reason != tc.wantReason {
				t.Fatalf("applied=%v reason=%q, want applied=%v reason=%q", applied, reason, tc.wantApplied, tc.wantReason)
			}
			if !tc.wantApplied {
				if string(out) != tc.input {
					t.Fatalf("raw case changed the input: %q", out)
				}
				return
			}
			if string(out) != tc.wantOut {
				t.Fatalf("out:\n%s\nwant:\n%s", out, tc.wantOut)
			}
			// Every match survives: its file, line number and text minus leading whitespace.
			for _, row := range strings.Split(strings.TrimSuffix(tc.input, "\n"), "\n") {
				parts := strings.SplitN(row, ":", 3)
				want := parts[0] + ":" + parts[1] + ":" + strings.TrimLeft(parts[2], " \t")
				file := parts[0]
				got := false
				for _, l := range strings.Split(string(out), "\n") {
					if l == file {
						got = true
					}
				}
				if !got || !strings.Contains(string(out), parts[1]+": "+strings.TrimLeft(parts[2], " \t")) {
					t.Fatalf("match %q not kept as %q", row, want)
				}
			}
		})
	}
}
