// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

func TestFamilyOf_SearchBreadth(t *testing.T) {
	cases := []struct {
		command string
		want    Family
	}{
		// allowed: rows without -n, file lists, counts
		{"rg foo internal", FamilyRg},
		{"rg -i foo internal cmd", FamilyRg},
		{"rg -l foo internal", FamilyRg},
		{"rg --files-with-matches foo internal", FamilyRg},
		{"rg -c foo internal", FamilyRg},
		{"rg --count foo internal", FamilyRg},
		{"rg -e foo internal", FamilyRg},
		{"rg --files internal", FamilyRgFiles},
		{"grep -r foo internal", FamilyGrep},
		{"grep -R foo internal", FamilyGrep},
		{"grep -rl foo internal", FamilyGrep},
		{"grep -rc foo internal", FamilyGrep},
		{"grep -l foo a.go b.go", FamilyGrep},
		{"grep -rli foo internal", FamilyGrep},

		// refused: standard input, no path, -l with -c, writers and shapers
		{"rg foo", FamilyNone},
		{"rg -l foo", FamilyNone},
		{"rg -c foo", FamilyNone},
		{"rg -lc foo internal", FamilyNone},
		{"rg --count --files-with-matches foo internal", FamilyNone},
		{"rg -L foo internal", FamilyRg}, // -L is --follow in rg
		{"rg --files-without-match foo internal", FamilyNone},
		{"rg --replace=x foo internal", FamilyNone},
		{"rg --pre cat foo internal", FamilyNone},
		{"rg -z foo internal", FamilyNone},
		{"rg -0 -l foo internal", FamilyNone},
		{"rg --null -l foo internal", FamilyNone},
		{"rg --json foo internal", FamilyNone},
		{"rg -A 2 foo internal", FamilyNone},
		{"rg --files --pre cat internal", FamilyNone},
		{"grep foo internal", FamilyNone}, // no -r, no -H, one path: no file name
		{"grep -c foo a.go", FamilyNone},  // one file prints only the count
		{"grep -l foo", FamilyNone},       // standard input
		{"grep -lc foo a.go b.go", FamilyNone},
		{"grep -L foo a.go b.go", FamilyNone},
		{"grep -rL foo internal", FamilyNone},
		{"grep -rq foo internal", FamilyNone},
		{"grep -ro foo internal", FamilyNone},
		{"grep -rZ foo internal", FamilyNone},
		{"grep -r --include=*.go foo internal", FamilyNone},
		{"grep -rh foo internal", FamilyNone},
		{"grep -r -f patterns.txt internal", FamilyNone},
		{"grep -r - internal", FamilyNone},
	}
	for _, tc := range cases {
		if got := FamilyOf(strings.Fields(tc.command)); got != tc.want {
			t.Errorf("FamilyOf(%q) = %q, want %q", tc.command, got, tc.want)
		}
	}
}

func TestCommandPlanSearchFormats(t *testing.T) {
	cases := []struct {
		command string
		format  compressor.Format
		roots   []string
	}{
		{"rg foo internal cmd", compressor.FormatGrepRows, []string{"internal", "cmd"}},
		{"rg -n foo internal", compressor.FormatGrep, []string{"internal"}},
		{"rg -l foo internal", compressor.FormatFind, []string{"internal"}},
		{"rg -c foo internal", compressor.FormatGrepCounts, []string{"internal"}},
		{"rg -e foo internal", compressor.FormatGrepRows, []string{"internal"}},
		{"grep -r foo internal", compressor.FormatGrepRows, []string{"internal"}},
		{"grep -rn foo internal", compressor.FormatGrep, []string{"internal"}},
		{"grep -rl foo internal", compressor.FormatFind, []string{"internal"}},
		{"grep -rc foo .", compressor.FormatGrepCounts, []string{"."}},
		{"grep -r foo", compressor.FormatGrepRows, []string{"."}},
	}
	for _, tc := range cases {
		argv := strings.Fields(tc.command)
		format, hint := commandPlan(FamilyOf(argv), argv)
		if format != tc.format || !reflect.DeepEqual(hint.Roots, tc.roots) {
			t.Errorf("commandPlan(%q) = %q roots %v, want %q roots %v", tc.command, format, hint.Roots, tc.format, tc.roots)
		}
	}
}
