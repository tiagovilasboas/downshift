// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"sort"
	"strings"
	"testing"
)

// A real lefthook v1 pre-commit transcript: git runs the hook with its output
// on stderr, then prints the commit line on stdout.
const lefthookCommitHooks = "╭───────────────────────────────────────╮\n" +
	"│ 🥊 lefthook v1.11.4  hook: pre-commit │\n" +
	"╰───────────────────────────────────────╯\n" +
	"│  gofmt (skip) no files for inspection\n" +
	"│  vet (skip) no matching staged files\n" +
	"│  tests (skip) no matching staged files\n" +
	"                                      \n" +
	"  ────────────────────────────────────\n" +
	"summary: (done in 0.06 seconds)       \n"

const commitLine = "[fix/x 05490e88] probe\n"

func TestRun_GitCommitCutsHookBanners(t *testing.T) {
	cases := []struct {
		name, script, wantOut string
		code                  int
		compacted             bool
	}{
		{
			name:      "clean commit keeps the summary and the commit line",
			script:    "cat >&2 <<'EOF'\n" + lefthookCommitHooks + "EOF\nprintf '" + commitLine[:len(commitLine)-1] + "\\n'\n",
			wantOut:   "summary: (done in 0.06 seconds)       \n" + commitLine,
			compacted: true,
		},
		{
			name:    "a failing hook keeps the whole output",
			script:  "cat >&2 <<'EOF'\n" + lefthookCommitHooks + "EOF\nexit 1\n",
			wantOut: lefthookCommitHooks,
			code:    1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeBin(t, "git", tc.script)
			r, out, errOut := run(t, []string{"git", "commit", "-m", "probe"}, true)
			got := out + errOut
			want := tc.wantOut
			if tc.compacted {
				want += Marker(len(lefthookCommitHooks)+len(commitLine), len(tc.wantOut))
			}
			if r.ExitCode != tc.code || r.Compacted != tc.compacted || r.Family != FamilyGitCommit {
				t.Fatalf("exit=%d compacted=%v family=%q, want %d %v %q", r.ExitCode, r.Compacted, r.Family, tc.code, tc.compacted, FamilyGitCommit)
			}
			// stderr (the hook) and stdout (the commit line) come through
			// separate pipes, so their order in the result is not fixed.
			if sortedLines(got) != sortedLines(want) {
				t.Fatalf("output = %q, want the lines of %q", got, want)
			}
		})
	}
}

// sortedLines is text's lines in sorted order, for output whose lines may
// arrive from two pipes in either order.
func sortedLines(text string) string {
	lines := strings.Split(text, "\n")
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}
