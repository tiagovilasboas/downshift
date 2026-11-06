// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"fmt"
	"strings"
	"testing"
)

// repeatRun is 20 same-shape lines, a run FoldRepeats cuts.
func repeatRun() []byte {
	var b strings.Builder
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&b, "test_case_%d ... ok\n", i)
	}
	return []byte(b.String())
}

func storeOK(calls *[][]byte) func([]byte) (string, bool) {
	return func(b []byte) (string, bool) { *calls = append(*calls, b); return "0123456789ab", true }
}

func TestKeepUncompacted_FoldsOnlyWhenStored(t *testing.T) {
	raw := repeatRun()
	var calls [][]byte
	out, ok := keepUncompacted(storeOK(&calls), raw, 0, FamilyGoTest, false)
	if !ok || len(calls) != 1 || string(calls[0]) != string(raw) {
		t.Fatalf("ok=%v calls=%d", ok, len(calls))
	}
	if !strings.Contains(string(out), "similar lines folded") || !strings.HasSuffix(string(out), "downshift retrieve 0123456789ab]\n") {
		t.Fatalf("out = %q", out)
	}
	if len(out) >= len(raw) {
		t.Fatalf("folded %d bytes, raw %d", len(out), len(raw))
	}
}

func TestKeepUncompacted_Refuses(t *testing.T) {
	raw := repeatRun()
	cases := []struct {
		name   string
		keep   func([]byte) (string, bool)
		raw    []byte
		exit   int
		family Family
		stderr bool
	}{
		{"nil keep", nil, raw, 0, FamilyGoTest, false},
		{"failed run", func([]byte) (string, bool) { return "0123456789ab", true }, raw, 1, FamilyGoTest, false},
		{"wrote stderr", func([]byte) (string, bool) { return "0123456789ab", true }, raw, 0, FamilyGoTest, true},
		{"not compactible", func([]byte) (string, bool) { return "0123456789ab", true }, raw, 0, Family("none"), false},
		{"nothing to fold", func([]byte) (string, bool) { return "0123456789ab", true }, []byte("a\nb\n"), 0, FamilyGoTest, false},
		{"store refused", func([]byte) (string, bool) { return "", false }, raw, 0, FamilyGoTest, false},
		{"store panics", func([]byte) (string, bool) { panic("boom") }, raw, 0, FamilyGoTest, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if out, ok := keepUncompacted(tc.keep, tc.raw, tc.exit, tc.family, tc.stderr); ok || out != nil {
				t.Fatalf("ok=%v out=%q", ok, out)
			}
		})
	}
}

func TestKeepRaw_FoldsSummaryAfterStore(t *testing.T) {
	summary := append(repeatRun(), []byte(Marker(5000, 400))...)
	var calls [][]byte
	got := keepRaw(storeOK(&calls), []byte("raw"), summary)
	if len(calls) != 1 || !strings.Contains(string(got), "similar lines folded") || !strings.Contains(string(got), "downshift retrieve 0123456789ab") {
		t.Fatalf("got %q", got)
	}
	// Without a store the summary is not folded: cut lines must stay retrievable.
	if got := keepRaw(nil, []byte("raw"), summary); string(got) != string(summary) {
		t.Fatalf("nil keep changed output: %q", got)
	}
}
