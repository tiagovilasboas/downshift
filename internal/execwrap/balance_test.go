// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func balanceInput() []byte {
	var b bytes.Buffer
	for i := 0; i < 3000; i++ {
		fmt.Fprintf(&b, "step %d done\n", i)
	}
	return b.Bytes()
}

func TestBalanceNeedsStoredRaw(t *testing.T) {
	raw := balanceInput()
	var stored []byte
	keep := func(r []byte) (string, bool) { stored = r; return "0123456789ab", true }
	out, ok := balance(keep, FamilyMake, raw, raw, balanceConfig{})
	if !ok || !bytes.Equal(stored, raw) {
		t.Fatalf("not cut or raw not stored: ok=%v", ok)
	}
	if !strings.HasSuffix(string(out), "downshift retrieve 0123456789ab]\n") || len(out) >= len(raw)/2 {
		t.Errorf("cut output: %d of %d bytes, tail %q", len(out), len(raw), out[len(out)-80:])
	}
	if _, ok := balance(nil, FamilyMake, raw, raw, balanceConfig{}); ok {
		t.Error("cut without a store: the cut lines would be lost")
	}
	if _, ok := balance(func([]byte) (string, bool) { return "", false }, FamilyMake, raw, raw, balanceConfig{}); ok {
		t.Error("cut when the store failed")
	}
}

func TestBalanceSkipsSmallAndRefused(t *testing.T) {
	keep := func([]byte) (string, bool) { return "0123456789ab", true }
	small := []byte(strings.Repeat("x\n", 100))
	if _, ok := balance(keep, FamilyMake, small, small, balanceConfig{}); ok {
		t.Error("an output under the threshold was cut")
	}
	raw := balanceInput()
	for _, f := range []Family{FamilyFileRead, FamilyAgentDesktop} {
		if _, ok := balance(keep, f, raw, raw, balanceConfig{}); ok {
			t.Errorf("%s was cut with loss", f)
		}
	}
}

func TestBalanceReplacesTheLosslessMarker(t *testing.T) {
	keep := func([]byte) (string, bool) { return "0123456789ab", true }
	raw := balanceInput()
	shown := append(append([]byte{}, raw...), Marker(len(raw)+10, len(raw))...)
	out, ok := balance(keep, FamilyMake, raw, shown, balanceConfig{})
	if !ok || strings.Count(string(out), markerPrefix) != 1 {
		t.Fatalf("want one marker, got ok=%v:\n%s", ok, out[len(out)-200:])
	}
}
