// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func regroupRows(n int) []byte {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("internal/execwrap/compound_search.go:" + strings.Repeat("1", 1+i%3) + ":\tif keep == nil {\n")
	}
	return []byte(b.String())
}

func TestRegroupCompound_Gate(t *testing.T) {
	keep := func([]byte) (string, bool) { return "0123456789ab", true }
	rows := regroupRows(12)
	cases := []struct {
		name    string
		command string
		data    []byte
		exit    int
		keep    func([]byte) (string, bool)
		want    bool
	}{
		{"grep piped to head", "grep -rn keep internal | head -20", rows, 0, keep, true},
		{"git grep piped to grep", "git grep -n keep | grep -v _test", rows, 0, keep, true},
		{"env before rg", "LC_ALL=C rg -n keep internal | head", rows, 0, keep, true},
		{"filter found nothing else", "grep -rn keep . | grep -v x", rows, 1, keep, true},
		{"grep error", "grep -rn keep internal | head", rows, 2, keep, false},
		{"file read first", "cat rows.txt | grep keep", rows, 0, keep, false},
		{"find first", "find . -name '*.go' | xargs grep -n keep", rows, 0, keep, false},
		{"nothing stored", "grep -rn keep internal | head", rows, 0, nil, false},
		{"store refused", "grep -rn keep internal | head", rows, 0, func([]byte) (string, bool) { return "", false }, false},
		{"over the threshold", "grep -rn keep internal | head", regroupRows(400), 0, keep, false},
		{"too few rows", "grep -rn keep internal | head", regroupRows(2), 0, keep, false},
	}
	for _, c := range cases {
		res, ok := RegroupCompound(c.command, c.data, c.exit, c.keep, "")
		if ok != c.want {
			t.Errorf("%s: regrouped %v, want %v", c.name, ok, c.want)
			continue
		}
		if !ok {
			continue
		}
		if !res.Compacted || res.Folded || res.KeptID != "0123456789ab" {
			t.Errorf("%s: compacted %v folded %v id %q", c.name, res.Compacted, res.Folded, res.KeptID)
		}
		if !HasMarker(string(res.Output)) || len(res.Output)*4 > len(c.data)*3 {
			t.Errorf("%s: %d to %d bytes or no marker:\n%s", c.name, len(c.data), len(res.Output), res.Output)
		}
	}
}

func TestRegroupCompound_HintNamesSession(t *testing.T) {
	keep := func([]byte) (string, bool) { return "0123456789ab", true }
	res, ok := RegroupCompound("grep -rn keep internal | head", regroupRows(12), 0, keep, strings.Repeat("ab", 32))
	if !ok || !strings.Contains(string(res.Output), "downshift retrieve --session=abababababab 0123456789ab]") {
		t.Fatalf("hint without session: %s", res.Output)
	}
}
