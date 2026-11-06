// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func TestFamilyOf_RipgrepAndGitGrep(t *testing.T) {
	cases := []struct {
		cmd  string
		want Family
	}{
		// rg match rows: accepted
		{"rg -n foo internal", FamilyRg},
		{"rg --line-number foo internal cmd", FamilyRg},
		{"rg -ni foo internal", FamilyRg},
		{"rg -in foo internal", FamilyRg},
		{"rg -ne foo internal", FamilyRg},
		{"rg -n -e foo internal", FamilyRg},
		{"rg -n --regexp=foo internal", FamilyRg},
		{"rg -n --regexp foo internal", FamilyRg},
		{"rg -n --glob=*.go foo internal", FamilyRg},
		{"rg -n -g *.go foo internal", FamilyRg},
		{"rg -n -t go -T md -m 3 foo internal", FamilyRg},
		{"rg -n --type=go --max-count 3 foo .", FamilyRg},
		{"rg -nSsHwxFuL foo .", FamilyRg},
		{"rg -n --hidden --no-ignore --follow --no-heading --with-filename foo .", FamilyRg},
		// rows without -n, file lists (-l) and counts (-c)
		{"rg foo internal", FamilyRg},
		{"rg -nc foo internal", FamilyRg},
		{"rg -n --count foo internal", FamilyRg},
		{"rg -nl foo internal", FamilyRg},
		{"rg -n --files-with-matches foo internal", FamilyRg},
		// rg match rows: refused
		{"rg -n foo", FamilyNone},
		{"rg -n -e foo", FamilyNone},
		{"rg -n", FamilyNone},
		{"rg -nr bar foo internal", FamilyNone},
		{"rg -n -r bar foo internal", FamilyNone},
		{"rg -n --replace=bar foo internal", FamilyNone},
		{"rg -n --pre cat foo internal", FamilyNone},
		{"rg -n --pre=cat foo internal", FamilyNone},
		{"rg -n --pre-glob=*.gz foo internal", FamilyNone},
		{"rg -nz foo internal", FamilyNone},
		{"rg -n --search-zip foo internal", FamilyNone},
		{"rg -n -A 2 foo internal", FamilyNone},
		{"rg -n -B 2 foo internal", FamilyNone},
		{"rg -n -C 2 foo internal", FamilyNone},
		{"rg -n --context=2 foo internal", FamilyNone},
		{"rg -n --files-without-match foo internal", FamilyNone},
		{"rg -no foo internal", FamilyNone},
		{"rg -nv foo internal", FamilyNone},
		{"rg -n --json foo internal", FamilyNone},
		{"rg -n --heading foo internal", FamilyNone},
		{"rg -np foo internal", FamilyNone},
		{"rg -n --pretty foo internal", FamilyNone},
		{"rg -n --vimgrep foo internal", FamilyNone},
		{"rg -n0 foo internal", FamilyNone},
		{"rg -n --null foo internal", FamilyNone},
		{"rg -n --color=always foo internal", FamilyNone},
		{"rg -nU foo internal", FamilyNone},
		{"rg -n --multiline foo internal", FamilyNone},
		{"rg -n --passthru foo internal", FamilyNone},
		{"rg -n --stats foo internal", FamilyNone},
		{"rg -n --debug foo internal", FamilyNone},
		{"rg -nh foo internal", FamilyNone},
		{"rg -n foo -", FamilyNone},
		{"rg -n foo -- internal", FamilyNone},
		{"rg -n -- foo internal", FamilyNone},
		{"rg -en foo internal", FamilyNone},         // -e must end its cluster
		{"rg -n --regexp", FamilyNone},              // missing value
		{"rg -n --line-number=1 foo .", FamilyNone}, // plain flag with a value
		{"rg -n -g", FamilyNone},
		// rg --files
		{"rg --files", FamilyRgFiles},
		{"rg --files internal cmd", FamilyRgFiles},
		{"rg --files --hidden --no-ignore --follow internal", FamilyRgFiles},
		{"rg --files -uL internal", FamilyRgFiles},
		{"rg --files -g *.go internal", FamilyRgFiles},
		{"rg --files --glob=*.go -t go -T md", FamilyRgFiles},
		{"rg --files -n foo", FamilyNone},
		{"rg --files -e foo", FamilyNone},
		{"rg --files --pre cat", FamilyNone},
		{"rg --files -0", FamilyNone},
		{"rg --files -- internal", FamilyNone},
		{"rg --files -", FamilyNone},
		{"rg -n --files foo internal", FamilyNone}, // -n form does not know --files
		// git grep: accepted
		{"git grep -n foo", FamilyGitGrep},
		{"git grep -n foo internal", FamilyGitGrep},
		{"git grep --line-number foo -- internal cmd", FamilyGitGrep},
		{"git grep -niwFEI foo", FamilyGitGrep},
		{"git grep -ne foo internal", FamilyGitGrep},
		{"git grep -n -e foo -- internal", FamilyGitGrep},
		{"git grep -n -e foo", FamilyGitGrep},
		{"git grep -n --ignore-case --word-regexp --fixed-strings --extended-regexp foo", FamilyGitGrep},
		// git grep: refused (the subcommand stays git-other, never compacted)
		{"git grep foo", FamilyGitOther},
		{"git grep -n", FamilyGitOther},
		{"git grep -n --", FamilyGitOther},
		{"git grep -nh foo", FamilyGitOther},
		{"git grep -nc foo", FamilyGitOther},
		{"git grep -nl foo", FamilyGitOther},
		{"git grep -nL foo", FamilyGitOther},
		{"git grep -no foo", FamilyGitOther},
		{"git grep -nv foo", FamilyGitOther},
		{"git grep -np foo", FamilyGitOther},
		{"git grep -nW foo", FamilyGitOther},
		{"git grep -n -A 2 foo", FamilyGitOther},
		{"git grep -n -B 2 foo", FamilyGitOther},
		{"git grep -n -C 2 foo", FamilyGitOther},
		{"git grep -nz foo", FamilyGitOther},
		{"git grep -n -O foo", FamilyGitOther},
		{"git grep -nOcat foo", FamilyGitOther},
		{"git grep -n --open-files-in-pager=cat foo", FamilyGitOther},
		{"git grep -n --open-files-in-pager foo", FamilyGitOther},
		{"git grep -n --cached foo", FamilyGitOther},
		{"git grep -n --no-index foo", FamilyGitOther},
		{"git grep -n --threads 2 foo", FamilyGitOther},
		{"git grep -n --count foo", FamilyGitOther},
		{"git grep -en foo", FamilyGitOther},
		{"git grep -n -e", FamilyGitOther},
		{"git grep -n foo -", FamilyGitOther},
		{"git -C x grep -n foo", FamilyNone},
	}
	for _, tc := range cases {
		t.Run(tc.cmd, func(t *testing.T) {
			if got := FamilyOf(strings.Fields(tc.cmd)); got != tc.want {
				t.Fatalf("FamilyOf(%q) = %q, want %q", tc.cmd, got, tc.want)
			}
		})
	}
}
