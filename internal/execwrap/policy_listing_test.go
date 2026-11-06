// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func TestListingFamilyTable(t *testing.T) {
	cases := []struct {
		command string
		want    Family
	}{
		// ls: names only; flags that change the output shape are refused.
		{"ls", FamilyLs},
		{"ls -la internal", FamilyLs},
		{"ls -1pt internal", FamilyLs},
		{"ls -R internal", FamilyNone},
		{"ls -s internal", FamilyNone},
		{"ls -h internal", FamilyNone},
		{"ls -C internal", FamilyNone},
		{"ls --color=never internal", FamilyNone},
		{"ls internal cmd", FamilyNone}, // several operands print name headers
		{"ls -", FamilyNone},

		// find: any path list; the acting and output-shape primaries are refused.
		{"find internal -name *.go", FamilyFind},
		{"find . -type f -maxdepth 2", FamilyFind},
		{"find . -exec rm {} ;", FamilyNone},
		{"find . -execdir sh -c x ;", FamilyNone},
		{"find . -ok rm ;", FamilyNone},
		{"find . -okdir rm ;", FamilyNone},
		{"find . -delete", FamilyNone},
		{"find . -fprint out.txt", FamilyNone},
		{"find . -fprint0 out.txt", FamilyNone},
		{"find . -fprintf out.txt %p", FamilyNone},
		{"find . -fls out.txt", FamilyNone},
		{"find . -ls", FamilyNone},
		{"find . -printf %p", FamilyNone},
		{"find . -print0", FamilyNone},

		// grep: needs a file name on every row (-H, -r or two paths), and a path unless -r.
		{"grep -rn func internal", FamilyGrep},
		{"grep -rn foo", FamilyGrep},
		{"grep -Hn foo file.go", FamilyGrep},
		{"grep -n -e foo -e bar internal", FamilyNone}, // two patterns, one path, no -r or -H: no file names
		{"grep -rn -e foo -e bar internal", FamilyGrep},
		{"grep -rn -e foo internal", FamilyGrep},
		{"grep -n foo file.go", FamilyNone},  // one file, no -H or -r: no file name
		{"grep -rn", FamilyNone},             // no pattern
		{"grep -r foo internal", FamilyGrep}, // rows without a line number
		{"grep -rnc foo internal", FamilyGrep},
		{"grep -rc foo internal", FamilyGrep},
		{"grep -rl foo internal", FamilyGrep},
		{"grep -rlc foo internal", FamilyNone}, // -l with -c
		{"grep -rnl foo internal", FamilyGrep},
		{"grep -rnL foo internal", FamilyNone},
		{"grep -rno foo internal", FamilyNone},
		{"grep -rnq foo internal", FamilyNone},
		{"grep -rnv foo internal", FamilyNone},
		{"grep -rnA2 foo internal", FamilyNone},
		{"grep -rnh foo internal", FamilyNone},
		{"grep -rnP foo internal", FamilyNone},
		{"grep -rn --include=*.go foo internal", FamilyNone},
		{"grep -rn -- foo internal", FamilyNone},
		{"grep -rnie foo internal", FamilyGrep}, // -e ends its cluster
		{"grep -rnei foo internal", FamilyNone}, // -e is not last in its cluster
	}
	for _, tc := range cases {
		argv := strings.Fields(tc.command)
		if got := FamilyOf(argv); got != tc.want {
			t.Errorf("FamilyOf(%q) = %q, want %q", tc.command, got, tc.want)
		}
	}
}

func TestListingDecideWrapsOnlyPlainForms(t *testing.T) {
	const exe = "/usr/local/bin/downshift"
	if d := Decide("ls -la internal", exe); !d.Rewrite || d.Family != FamilyLs {
		t.Fatalf("ls -la: rewrite=%v family=%q reason=%q", d.Rewrite, d.Family, d.Reason)
	}
	if d := Decide("find internal -delete", exe); d.Rewrite {
		t.Fatalf("find -delete was wrapped: %q", d.Command)
	}
}
