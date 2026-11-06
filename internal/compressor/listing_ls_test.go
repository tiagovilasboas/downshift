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
			name:        "long form stays raw: size, mtime and type are what the agent asked for",
			input:       lsLongListing,
			wantApplied: false,
			wantReason:  "ls_long_form_kept_raw",
		},
		{
			name:        "long row without a total line stays raw",
			input:       "-rw-r--r--  1 tiago.boas  staff  1518 Nov  4 10:00 AGENTS.md\n",
			wantApplied: false,
			wantReason:  "ls_long_form_kept_raw",
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
			wantReason:  "ls_long_form_kept_raw",
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
