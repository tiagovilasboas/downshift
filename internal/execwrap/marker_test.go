// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

// A copy is never stored and then denied. The guard sizes the marker with the
// longest hint (the session flag included) before keep is called, so a run
// whose output would reach the raw length with that hint is not stored and its
// marker says no copy exists. When a copy is stored the hint names its id.
func TestKeepRawFor_TextMatchesWhetherACopyWasStored(t *testing.T) {
	const summary = "* main\n M a.go\n"
	for _, tc := range []struct {
		name       string
		rawLen     int
		id         string
		wantStored bool
		wantSuffix string
	}{
		{"no room for the session hint", 100, "--session=aaaaaaaaaaaa 0123456789ab", false, "; original not kept]\n"},
		{"room, session hint", 400, "--session=aaaaaaaaaaaa 0123456789ab", true, "; downshift retrieve --session=aaaaaaaaaaaa 0123456789ab]\n"},
		{"room, bare id", 400, "0123456789ab", true, "; downshift retrieve 0123456789ab]\n"},
	} {
		raw := []byte(strings.Repeat("x", tc.rawLen-1) + "\n")
		calls := 0
		keep := func([]byte) (string, bool) { calls++; return tc.id, true }
		out := keepRawFor(FamilyNone, keep, raw, []byte(summary+Marker(len(raw), len(summary))))
		if (calls > 0) != tc.wantStored {
			t.Errorf("%s: keep called %d times, want stored=%v", tc.name, calls, tc.wantStored)
		}
		if !strings.HasSuffix(string(out), tc.wantSuffix) {
			t.Errorf("%s: marker is %q, want it to end %q", tc.name, out, tc.wantSuffix)
		}
		if len(out) >= len(raw) {
			t.Errorf("%s: output %d bytes, raw %d", tc.name, len(out), len(raw))
		}
	}
}

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
