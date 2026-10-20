// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import "testing"

func TestSnapshotPrice(t *testing.T) {
	in, out := 0.5, 1.5
	openRouter := OpenRouterAPI{}.Name()
	f := File{Version: 1, Harnesses: map[string]Entry{
		"opencode": {Source: openRouter, Models: []Model{
			{ID: "vendor/priced", InputM: &in, OutputM: &out},
			{ID: "vendor/half-priced", InputM: &in},
		}},
		"cursor": {Source: "cursor CLI", Models: []Model{{ID: "vendor/cli-only", InputM: &in, OutputM: &out}}},
	}}
	cases := []struct {
		model string
		ok    bool
	}{
		{"vendor/priced", true},
		{"vendor/half-priced", false},
		{"vendor/cli-only", false},
		{"vendor/absent", false},
	}
	for _, tc := range cases {
		gotIn, gotOut, ok := SnapshotPrice(f, tc.model)
		if ok != tc.ok {
			t.Fatalf("SnapshotPrice(%q) ok=%v, want %v", tc.model, ok, tc.ok)
		}
		if ok && (gotIn != in || gotOut != out) {
			t.Fatalf("SnapshotPrice(%q) = %v/%v, want %v/%v", tc.model, gotIn, gotOut, in, out)
		}
	}
}
