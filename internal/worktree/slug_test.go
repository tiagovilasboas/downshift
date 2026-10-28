// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package worktree

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		wantErr bool
	}{
		// Accepted: the grammar holds and no extra rule applies.
		{"letters", "parser", false},
		{"one character", "a", false},
		{"digit first", "0x", false},
		{"hyphen, dot and underscore inside", "w-1.task_v2", false},
		{"64 characters", strings.Repeat("a", 64), false},
		{"console is not con", "console", false},
		{"con with hyphen is not con", "con-1", false},
		{"com0 is not reserved", "com0", false},
		{"com10 is not reserved", "com10", false},
		{"lpt0 is not reserved", "lpt0", false},
		{"lock inside the name", "lock.a", false},

		// Refused by the R54 grammar.
		{"empty", "", true},
		{"65 characters", strings.Repeat("a", 65), true},
		{"leading hyphen", "-lead", true},
		{"leading dot", ".hidden", true},
		{"uppercase", "Parser", true},
		{"space inside", "has space", true},
		{"trailing space", "trail ", true},
		{"forward slash", "a/b", true},
		{"backslash", `a\b`, true},
		{"control byte", "a\x00b", true},
		{"newline at the end", "abc\n", true},
		{"at-brace", "a@{0}", true},
		{"colon, as in a drive", "c:d", true},
		{"dot dot alone", "..", true},

		// Refused by the rules the grammar cannot express.
		{"dot dot inside", "a..b", true},
		{"lock suffix", "x.lock", true},
		{"trailing dot", "trail.", true},

		// Windows device names, bare or with an extension.
		{"con", "con", true},
		{"prn", "prn", true},
		{"aux", "aux", true},
		{"nul", "nul", true},
		{"com1", "com1", true},
		{"com9 with extension", "com9.log", true},
		{"lpt1", "lpt1", true},
		{"lpt5 with extension", "lpt5.txt", true},
		{"nul with two extensions", "nul.tar.gz", true},
		{"upper case device, refused by grammar", "CON", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSlug(tc.slug)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateSlug(%q) error = %v, wantErr %v", tc.slug, err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidSlug) {
				t.Errorf("ValidateSlug(%q) error %v does not wrap ErrInvalidSlug", tc.slug, err)
			}
		})
	}
}
