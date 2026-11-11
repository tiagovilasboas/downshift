// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "testing"

// The two marker texts are the contract the agent reads: the size cut, then
// either the command that returns the original or the plain statement that no
// copy exists.
func TestMarkerTexts(t *testing.T) {
	for _, tc := range []struct{ name, got, want string }{
		{"no copy", Marker(194873, 91342), "[downshift compact: 194873 to 91342 bytes; original not kept]\n"},
		{"copy kept", MarkerWithID(194873, 91342, "0123456789ab"), "[downshift compact: 194873 to 91342 bytes; downshift retrieve 0123456789ab]\n"},
		{"copy kept with session", MarkerWithID(10, 2, "--session=aaaaaaaaaaaa 0123456789ab"), "[downshift compact: 10 to 2 bytes; downshift retrieve --session=aaaaaaaaaaaa 0123456789ab]\n"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
		if !HasMarker(tc.got) {
			t.Errorf("%s: HasMarker is false for %q", tc.name, tc.got)
		}
	}
}

// reKeptMarker reads the retrieve argument from the current marker and from
// the older spelling that is already printed in transcripts.
func TestKeptMarkerParsesBothSpellings(t *testing.T) {
	for _, tc := range []struct{ name, line, want string }{
		{"current", "[downshift compact: 9 to 3 bytes; downshift retrieve 0123456789ab]\n", "0123456789ab"},
		{"current with session", "[downshift compact: 9 to 3 bytes; downshift retrieve --session=aaaaaaaaaaaa 0123456789ab]\n", "--session=aaaaaaaaaaaa 0123456789ab"},
		{"older", "[downshift compact: compacted 9 to 3 bytes; full output: downshift retrieve 0123456789ab]\n", "0123456789ab"},
		{"older with session", "[downshift compact: compacted 9 to 3 bytes; full output: downshift retrieve --session=aaaaaaaaaaaa 0123456789ab]\n", "--session=aaaaaaaaaaaa 0123456789ab"},
		{"no copy", Marker(9, 3), ""},
	} {
		m := reKeptMarker.FindSubmatch([]byte(tc.line))
		got := ""
		if m != nil {
			got = string(m[1])
		}
		if got != tc.want {
			t.Errorf("%s: parsed %q, want %q", tc.name, got, tc.want)
		}
	}
}
