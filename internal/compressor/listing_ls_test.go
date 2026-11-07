// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

const lsLongListing = `total 24
drwxr-xr-x  12 tiago.boas  staff   384 Nov  4 10:00 .
drwxr-xr-x   5 tiago.boas  staff   160 Nov  4 09:12 ..
-rw-r--r--@  1 tiago.boas  staff  1518 Nov  4 10:00 AGENTS.md
drwxr-xr-x   9 tiago.boas  staff   288 Nov  4 10:00 cmd
lrwxr-xr-x   1 tiago.boas  staff     7 Nov  4 10:00 current -> cmd/app
-rw-r--r--   1 tiago.boas  staff  2290 Mar  2  2025 go.sum
-rw-r--r--   1 tiago.boas  staff  4102 2026-11-04 10:00 notes.md
`

func TestCompressLsTable(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantApplied bool
		wantOut     string
		wantReason  string
	}{
		{
			name:        "long form compacts: permissions, owner, group and total line dropped",
			input:       lsLongListing,
			wantApplied: true,
			wantOut:     "1518 Nov  4 10:00 AGENTS.md\n 288 Nov  4 10:00 cmd/\n   7 Nov  4 10:00 current -> cmd/app\n2290 Mar  2  2025 go.sum\n4102 2026-11-04 10:00 notes.md\n",
			wantReason:  "ls long form compacted",
		},
		{
			name:        "long row without a total line compacts",
			input:       "-rw-r--r--  1 tiago.boas  staff  1518 Nov  4 10:00 AGENTS.md\n",
			wantApplied: true,
			wantOut:     "1518 Nov  4 10:00 AGENTS.md\n",
			wantReason:  "ls long form compacted",
		},
		{
			name: "long form with human-readable sizes and dot entries",
			input: "total 32\n" +
				"drwxr-xr-x  12 user  staff  4.0K Nov  4 10:00 .\n" +
				"drwxr-xr-x   5 user  staff   160 Nov  4 09:12 ..\n" +
				"-rw-r--r--   1 user  staff  1.5K Nov  4 10:00 app.go\n" +
				"drwxr-xr-x   3 user  staff  512B Nov  4 10:00 pkg\n",
			wantApplied: true,
			wantOut:     "1.5K Nov  4 10:00 app.go\n512B Nov  4 10:00 pkg/\n",
			wantReason:  "ls long form compacted",
		},
		{
			name:        "long listing with only dot entries has no entry to keep",
			input:       "total 16\ndrwxr-xr-x  12 user  staff  384 Nov  4 10:00 .\ndrwxr-xr-x   5 user  staff  160 Nov  4 09:12 ..\n",
			wantApplied: false,
			wantReason:  "ls_empty_listing_preserved",
		},
		{
			name:        "plain form keeps names and drops . and ..",
			input:       ".\n..\n.github/\nREADME.md\ninternal/\n",
			wantApplied: true,
			wantOut:     ".github/\nREADME.md\ninternal/\n",
			wantReason:  "ls names only",
		},
		{
			name:        "danger word in a name keeps the listing raw",
			input:       strings.Replace(lsLongListing, "go.sum", "errors.go", 1),
			wantApplied: false,
			wantReason:  "ls_danger_word_preserved",
		},
		{
			name:        "several operands print name headers and stay raw",
			input:       "internal:\ncompressor\nexecwrap\n\ncmd:\nmain.go\n",
			wantApplied: false,
			wantReason:  "ls_unrecognized_structure_preserved",
		},
		{
			name:        "blank line in the listing stays raw",
			input:       "AGENTS.md\n\nREADME.md\n",
			wantApplied: false,
			wantReason:  "ls_unrecognized_structure_preserved",
		},
		{
			name:        "plain listing that holds a long-form row stays raw",
			input:       "AGENTS.md\n-rw-r--r-- 1 tiago staff 12 Nov 4 10:00 x\n",
			wantApplied: false,
			wantReason:  "ls_unrecognized_structure_preserved",
		},
		{
			name:        "only dot entries has no entry to keep",
			input:       ".\n..\n",
			wantApplied: false,
			wantReason:  "ls_empty_listing_preserved",
		},
		{
			name:        "a listing with no gain stays raw",
			input:       "a\n",
			wantApplied: false,
			wantReason:  "no_compression_gain",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, applied, reason := compressLs([]byte(tc.input))
			if applied != tc.wantApplied || reason != tc.wantReason {
				t.Fatalf("applied=%v reason=%q, want applied=%v reason=%q", applied, reason, tc.wantApplied, tc.wantReason)
			}
			if tc.wantApplied && string(out) != tc.wantOut {
				t.Fatalf("out=%q, want %q", out, tc.wantOut)
			}
			if !tc.wantApplied && string(out) != tc.input {
				t.Fatalf("raw case changed the input: %q", out)
			}
		})
	}
}
