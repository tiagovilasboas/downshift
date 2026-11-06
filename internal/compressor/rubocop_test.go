// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"os"
	"strings"
	"testing"
)

func rubocopFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/corpus/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCompressRubocop(t *testing.T) {
	offenses := rubocopFixture(t, "rubocop_offenses.txt")
	cases := []struct {
		name     string
		input    string
		applied  bool
		contains []string
		absent   []string
	}{
		{
			name:     "clean run keeps the summary line",
			input:    rubocopFixture(t, "rubocop_clean.txt"),
			applied:  true,
			contains: []string{"120 files inspected, no offenses detected"},
			absent:   []string{"Inspecting", "...."},
		},
		{
			name:    "offences grouped by file and cop, error kept verbatim",
			input:   offenses,
			applied: true,
			contains: []string{
				"app/a.rb: Lint/UselessAssignment 1, Style/FrozenStringLiteralComment 1",
				"app/b.rb: Lint/Syntax 1, Style/FrozenStringLiteralComment 1",
				"cops: Style/FrozenStringLiteralComment 2, Lint/Syntax 1, Lint/UselessAssignment 1",
				"app/b.rb:12:7: E: Lint/Syntax: unexpected token tRPAREN\n  foo(1))\n       ^\n",
				"3 files inspected, 4 offenses detected, 2 offenses autocorrectable\n",
			},
			absent: []string{"puts 'z'", "y = 1"},
		},
		{
			name:    "count disagreeing with the summary stays raw",
			input:   strings.Replace(offenses, "4 offenses detected", "5 offenses detected", 1),
			applied: false,
		},
		{
			name:    "trailing line after the summary stays raw",
			input:   rubocopFixture(t, "rubocop_clean.txt") + "Tip: something\n",
			applied: false,
		},
		{
			name:    "unknown line before the summary stays raw",
			input:   "warning: obsolete config\n" + rubocopFixture(t, "rubocop_clean.txt"),
			applied: false,
		},
		{name: "no final newline stays raw", input: strings.TrimSuffix(offenses, "\n"), applied: false},
		{name: "colour escapes stay raw", input: "\x1b[32m" + offenses, applied: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, applied, _ := compressRubocop([]byte(tc.input))
			if applied != tc.applied {
				t.Fatalf("applied = %v, want %v\n%s", applied, tc.applied, out)
			}
			if !applied {
				if string(out) != tc.input {
					t.Fatalf("raw output changed")
				}
				return
			}
			if len(out) >= len(tc.input) {
				t.Fatalf("not shorter: %d >= %d", len(out), len(tc.input))
			}
			for _, s := range tc.contains {
				if !strings.Contains(string(out), s) {
					t.Errorf("missing %q in\n%s", s, out)
				}
			}
			for _, s := range tc.absent {
				if strings.Contains(string(out), s) {
					t.Errorf("unexpected %q in\n%s", s, out)
				}
			}
		})
	}
}

func TestDetectRubocop(t *testing.T) {
	for _, name := range []string{"rubocop_clean.txt", "rubocop_offenses.txt"} {
		if f, _ := detectFormat([]byte(rubocopFixture(t, name))); f != FormatRubocop {
			t.Errorf("%s detected as %q", name, f)
		}
	}
}
