// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

// findPathSet rebuilds the input paths from a find tree. Each line is one
// component, indented by its depth; a name without a trailing slash is an
// input path, and a name with one is a directory the filter did not print.
func findPathSet(t *testing.T, tree string) map[string]bool {
	t.Helper()
	set := map[string]bool{}
	var stack []string
	for _, line := range strings.Split(strings.TrimSuffix(tree, "\n"), "\n") {
		depth := len(line) - len(strings.TrimLeft(line, " "))
		name := line[depth:]
		if depth > len(stack) {
			t.Fatalf("indent jumps past the parent: %q", line)
		}
		part := strings.TrimSuffix(name, "/")
		if name == "/" {
			part = "/"
		}
		full := part
		if depth > 0 {
			parent := stack[depth-1]
			if name == "/" {
				t.Fatalf("root below depth 0: %q", line)
			}
			if strings.HasSuffix(parent, "/") {
				full = parent + part
			} else {
				full = parent + "/" + part
			}
		}
		stack = append(stack[:depth], full)
		if !strings.HasSuffix(name, "/") {
			set[full] = true
		}
	}
	return set
}

func lines(s ...string) string { return strings.Join(s, "\n") + "\n" }

func TestCompressFindTable(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantApplied bool
		wantOut     string // checked when applied
		wantReason  string
	}{
		{
			name: "filtered paths keep the directories they pass through as slash nodes",
			input: lines(
				"internal/compressor/apply.go",
				"internal/compressor/types.go",
				"internal/compressor/deep/x.go",
				"internal/execwrap/run.go",
			),
			wantApplied: true,
			wantOut: lines(
				"internal/",
				" compressor/",
				"  apply.go",
				"  types.go",
				"  deep/",
				"   x.go",
				" execwrap/",
				"  run.go",
			),
			wantReason: "find tree",
		},
		{
			name: "a printed directory is an input path without a slash",
			input: lines(
				"internal",
				"internal/a.go",
				"internal/sub",
				"internal/sub/b.go",
			),
			wantApplied: true,
			wantOut: lines(
				"internal",
				" a.go",
				" sub",
				"  b.go",
			),
			wantReason: "find tree",
		},
		{
			name: "absolute paths hang under a root slash",
			input: lines(
				"/Users/dev/Github/project/internal/alpha.go",
				"/Users/dev/Github/project/internal/beta.go",
				"/Users/dev/Github/other/internal/gamma.go",
			),
			wantApplied: true,
			wantOut: lines(
				"/",
				" Users/",
				"  dev/",
				"   Github/",
				"    project/",
				"     internal/",
				"      alpha.go",
				"      beta.go",
				"    other/",
				"     internal/",
				"      gamma.go",
			),
			wantReason: "find tree",
		},
		{
			name: "a danger word in a path is kept, not dropped",
			input: lines(
				"./internal/errors.go",
				"./internal/panic_test.go",
				"./internal/warning.md",
			),
			wantApplied: true,
			wantOut: lines(
				"./",
				" internal/",
				"  errors.go",
				"  panic_test.go",
				"  warning.md",
			),
			wantReason: "find tree",
		},
		{
			name:        "duplicate path stays raw",
			input:       lines("internal/a.go", "internal/a.go", "internal/b.go", "internal/c.go"),
			wantApplied: false,
			wantReason:  "find_unrecognized_structure_preserved",
		},
		{
			name:        "double slash stays raw",
			input:       lines("internal//a.go", "internal/b.go"),
			wantApplied: false,
			wantReason:  "find_unrecognized_structure_preserved",
		},
		{
			name:        "name with a leading space stays raw",
			input:       lines("internal/ a.go", "internal/b.go"),
			wantApplied: false,
			wantReason:  "find_unrecognized_structure_preserved",
		},
		{
			name:        "blank line stays raw",
			input:       lines("internal/a.go", "", "internal/b.go"),
			wantApplied: false,
			wantReason:  "find_unrecognized_structure_preserved",
		},
		{
			name:        "single short path has no gain",
			input:       lines("a"),
			wantApplied: false,
			wantReason:  "no_compression_gain",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, applied, reason := compressFind([]byte(tc.input))
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
			want := map[string]bool{}
			for _, p := range strings.Split(strings.TrimSuffix(tc.input, "\n"), "\n") {
				want[p] = true
			}
			got := findPathSet(t, string(out))
			if len(got) != len(want) {
				t.Fatalf("path set size %d, want %d", len(got), len(want))
			}
			for p := range want {
				if !got[p] {
					t.Fatalf("path %q lost in the tree", p)
				}
			}
		})
	}
}
