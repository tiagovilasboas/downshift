// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// The serialized strings are a contract with telemetry, policy files and the
// hint convention: changing one breaks stored events and user config.
func TestEnumStringsAreStable(t *testing.T) {
	cases := []struct {
		name string
		got  []string
		want []string
	}{
		{"Kind", strs(Kinds()), []string{"research", "code", "review", "command", "scrap", "docs"}},
		{"EffortClass", strs(EffortClasses()), []string{"light", "standard", "deep"}},
		{"Tier", strs(Tiers()), []string{"small", "mid", "frontier"}},
		{"Verdict", strs(Verdicts()), []string{"kept", "profile", "rule", "hint_down", "guard_floor", "held_unknown", "refused_config"}},
		{"Reason", strs(Reasons()), []string{"risk_floor", "never_widen", "explicit_only", "foreign_id", "profile_override", "unknown_profile", "no_candidate", "fail_open"}},
		{"HeadroomVerdict", strs(HeadroomVerdicts()), []string{"chosen", "ignored"}},
		{"Channel", strs(Channels()), []string{"hook", "launch", "compile", "block_retry"}},
		{"Window", strs(Windows()), []string{"short", "week", "model"}},
	}
	for _, c := range cases {
		if len(c.got) != len(c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
			continue
		}
		for i := range c.got {
			if c.got[i] != c.want[i] {
				t.Errorf("%s[%d] = %q, want %q", c.name, i, c.got[i], c.want[i])
			}
		}
	}
}

func TestValidAcceptsOnlyDeclaredValues(t *testing.T) {
	for _, k := range Kinds() {
		if !k.Valid() {
			t.Errorf("Kind %q not valid", k)
		}
	}
	bad := []bool{
		KindNone.Valid(), Kind("mechanical").Valid(), Kind("Code").Valid(),
		EffortNone.Valid(), EffortClass("max").Valid(),
		TierUnknown.Valid(), Tier("strong").Valid(),
		Verdict("").Valid(), Verdict("chosen").Valid(),
		ReasonNone.Valid(), HeadroomUnknown.Valid(), ChannelUnknown.Valid(), WindowUnknown.Valid(),
	}
	for i, b := range bad {
		if b {
			t.Errorf("case %d: undeclared value reported valid", i)
		}
	}
}

func TestTierRoundTripsCoreAndKeepsUnknownDistinct(t *testing.T) {
	for _, ct := range []core.Tier{core.TierSmall, core.TierMid, core.TierFrontier} {
		got := TierOf(ct)
		if string(got) != ct.String() {
			t.Errorf("TierOf(%v) = %q, want core spelling %q", ct, got, ct.String())
		}
		back, ok := got.Core()
		if !ok || back != ct {
			t.Errorf("%q.Core() = %v,%v, want %v,true", got, back, ok, ct)
		}
	}
	if got := TierOf(core.Tier(99)); got != TierUnknown {
		t.Errorf("TierOf(99) = %q, want unknown", got)
	}
	if _, ok := TierUnknown.Core(); ok {
		t.Error("TierUnknown.Core() ok = true; unknown must never read as small")
	}
}

func TestIdentifiersRejectFreeText(t *testing.T) {
	for _, ok := range []string{"reviewer", "r1", "read-only.small_2"} {
		if !RuleID(ok).Valid() || !ProfileName(ok).Valid() {
			t.Errorf("%q should be a valid identifier", ok)
		}
	}
	for _, bad := range []string{"", "Reviewer", "fix the auth bug", "/home/u/x", "a\\b", "-x", string(make([]byte, 65))} {
		if RuleID(bad).Valid() || ProfileName(bad).Valid() {
			t.Errorf("%q should be refused", bad)
		}
	}
}

func TestZeroNoteSerializesNoClaim(t *testing.T) {
	b, err := json.Marshal(Note{Verdict: VerdictKept})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"verdict":"kept"}` {
		t.Fatalf("Note{kept} = %s; unknown fields must be omitted, not serialized as a tier", b)
	}
}

func strs[T ~string](vs []T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}
