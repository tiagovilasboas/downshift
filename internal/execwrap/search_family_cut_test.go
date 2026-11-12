// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"fmt"
	"strings"
	"testing"
)

func searchRows(n int) []byte {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "internal/a/file.go:%d:func handler%d() error { return nil }\n", i, i*7)
	}
	return []byte(b.String())
}

// The grep and git grep compressors already group rows by file and cap each
// file, so the row cut wins only for rg (whose output passes whole under the
// threshold); the table asserts every search family leaves the wrapper cut,
// stored and marked, and that no refusal path cuts.
func TestCompactCaptured_SearchFamilyCut(t *testing.T) {
	fam := map[Family][]string{
		FamilyGrep:    {"grep", "-rn", "handler", "."},
		FamilyRg:      {"rg", "handler"},
		FamilyGitGrep: {"git", "grep", "-n", "handler"},
	}
	store := func([]byte) (string, bool) { return "0123456789ab", true }
	refuse := func([]byte) (string, bool) { return "", false }
	cases := []struct {
		name    string
		rows    int
		exit    int
		keep    func([]byte) (string, bool)
		balance bool
		cut     bool
	}{
		{"cut", 60, 0, store, true, true},
		{"no match exit 1", 60, 1, store, true, true},
		{"keep nil", 60, 0, nil, true, false},
		{"balanced off", 60, 0, store, false, false},
		{"exit 2", 60, 2, store, true, false},
		{"store refused", 60, 0, refuse, true, false},
		{"under cap", 10, 0, store, true, false},
	}
	for family, argv := range fam {
		for _, tc := range cases {
			t.Run(string(family)+"/"+tc.name, func(t *testing.T) {
				data := searchRows(tc.rows)
				res := CompactCaptured(argv, family, data, tc.exit, tc.keep, tc.balance, "")
				got := string(res.Output)
				if tc.cut {
					if !res.Compacted || res.KeptID != "0123456789ab" || !strings.Contains(got, "downshift retrieve 0123456789ab]") {
						t.Fatalf("not cut: %+v", res)
					}
					if family == FamilyRg && !res.Folded {
						t.Fatalf("rg cut not folded: %+v", res)
					}
					if strings.Count(got, "handler") > 26 || len(got) >= len(data) {
						t.Fatalf("%d rows, %d of %d bytes", strings.Count(got, "handler"), len(got), len(data))
					}
					return
				}
				if family == FamilyRg && res.Compacted {
					t.Fatalf("rg cut without cause: %q", got)
				}
				if family == FamilyRg && got != string(data) {
					t.Fatalf("rg output altered")
				}
				if tc.name == "keep nil" && strings.Contains(got, "downshift retrieve") {
					t.Fatalf("marker without storage: %q", got)
				}
			})
		}
	}
}

func TestCapSearchFamily(t *testing.T) {
	raw := searchRows(60)
	store := func([]byte) (string, bool) { return "0123456789ab", true }
	for _, family := range []Family{FamilyGrep, FamilyRg, FamilyGitGrep} {
		out, ok := capSearchFamily(store, family, 0, raw, raw)
		if !ok || !strings.HasSuffix(string(out), "downshift retrieve 0123456789ab]\n") || len(out) >= len(raw) {
			t.Fatalf("%s: ok=%v out=%q", family, ok, out)
		}
		for _, tc := range []struct {
			name string
			keep func([]byte) (string, bool)
			exit int
			fam  Family
		}{
			{"nil keep", nil, 0, family},
			{"exit 2", store, 2, family},
			{"not search", store, 0, FamilyFind},
		} {
			if _, ok := capSearchFamily(tc.keep, tc.fam, tc.exit, raw, raw); ok {
				t.Fatalf("%s %s: cut", family, tc.name)
			}
		}
	}
	if _, ok := capSearchFamily(store, FamilyRg, 0, searchRows(10), searchRows(10)); ok {
		t.Fatal("under cap cut")
	}
}
