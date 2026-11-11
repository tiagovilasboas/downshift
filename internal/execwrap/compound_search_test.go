// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"fmt"
	"strings"
	"testing"
)

func manyRows(n int) []byte {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "internal/execwrap/compound_search.go:%d:\tif keep == nil || exitCode > 1 {\n", i)
	}
	return []byte(b.String())
}

func TestCutSearch_Gate(t *testing.T) {
	keep := func([]byte) (string, bool) { return "0123456789ab", true }
	rows := manyRows(60)
	cases := []struct {
		name    string
		command string
		data    []byte
		exit    int
		keep    func([]byte) (string, bool)
		want    bool
	}{
		{"grep piped to head", "grep -rn keep internal | head -80", rows, 0, keep, true},
		{"git grep piped to grep", "git grep -n keep | grep -v _test", rows, 0, keep, true},
		{"env before rg", "LC_ALL=C rg -n keep internal | sort", rows, 0, keep, true},
		{"filter found nothing else", "grep -rn keep . | grep -v x", rows, 1, keep, true},
		{"over the balanced threshold", "grep -rn keep . | grep -v x", manyRows(400), 0, keep, true},
		{"grep error", "grep -rn keep internal | head", rows, 2, keep, false},
		{"file read first", "cat rows.txt | grep keep", rows, 0, keep, false},
		{"find first", "find . -name '*.go' | xargs grep -n keep", rows, 0, keep, false},
		{"nothing stored", "grep -rn keep internal | head", rows, 0, nil, false},
		{"store refused", "grep -rn keep internal | head", rows, 0, func([]byte) (string, bool) { return "", false }, false},
		{"under the per-file cap", "grep -rn keep internal | head", manyRows(25), 0, keep, false},
	}
	for _, c := range cases {
		res, ok := CutSearch(c.command, c.data, c.exit, c.keep, "")
		if ok != c.want {
			t.Errorf("%s: cut %v, want %v", c.name, ok, c.want)
			continue
		}
		if !ok {
			continue
		}
		if !res.Compacted || !res.Folded || res.KeptID != "0123456789ab" {
			t.Errorf("%s: compacted %v folded %v id %q", c.name, res.Compacted, res.Folded, res.KeptID)
		}
		if !HasMarker(string(res.Output)) || len(res.Output) >= len(c.data) {
			t.Errorf("%s: %d to %d bytes or no marker", c.name, len(c.data), len(res.Output))
		}
	}
}

func TestCutSearch_HintNamesSession(t *testing.T) {
	keep := func([]byte) (string, bool) { return "0123456789ab", true }
	res, ok := CutSearch("grep -rn keep internal | head", manyRows(60), 0, keep, strings.Repeat("ab", 32))
	if !ok || !strings.HasSuffix(string(res.Output), "downshift retrieve --session=abababababab 0123456789ab]\n") {
		t.Fatalf("hint without session: %s", res.Output)
	}
}
