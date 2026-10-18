// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

type listerStub struct {
	Resolver
	allowed []string
}

func (s listerStub) AllowedEfforts(_, _ string) []string { return s.allowed }

func TestConstrainEffort(t *testing.T) {
	cases := []struct {
		name        string
		res         Resolver
		in          Effort
		want        Effort
		wantOK, chg bool
	}{
		{"allowed value passes", listerStub{allowed: []string{"low", "medium", "high"}}, EffortHigh, EffortHigh, true, false},
		{"unsupported high downgrades to medium", listerStub{allowed: []string{"low", "medium"}}, EffortHigh, EffortMid, true, true},
		{"nearest lower skips a gap", listerStub{allowed: []string{"low"}}, EffortHigh, EffortLow, true, true},
		{"nothing at or below omits", listerStub{allowed: []string{"high"}}, EffortLow, EffortLow, false, true},
		{"absent list unchanged", listerStub{}, EffortHigh, EffortHigh, true, false},
		{"resolver without lister unchanged", nil, EffortHigh, EffortHigh, true, false},
	}
	for _, c := range cases {
		got, ok, chg := ConstrainEffort(c.res, "h", "m", c.in)
		if got != c.want || ok != c.wantOK || chg != c.chg {
			t.Errorf("%s: got (%v,%v,%v), want (%v,%v,%v)", c.name, got, ok, chg, c.want, c.wantOK, c.chg)
		}
	}
}
