// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// The comparator behind the routing quality gate, on synthetic scores. These
// cases prove the gate can fail; TestRoutingQualityBaseline applies it to the
// recorded baseline.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift

import "testing"

func TestQualityRegressions(t *testing.T) {
	base := qualityScore{Cases: 200, TierCorrect: 141, UnderRouted: 25, Digest: "d1"}
	with := func(f func(*qualityScore)) qualityScore { s := base; f(&s); return s }
	cases := []struct {
		name string
		got  qualityScore
		want int // number of reported problems
	}{
		{"identical", base, 0},
		{"improved", with(func(s *qualityScore) { s.TierCorrect, s.UnderRouted = 150, 20 }), 0},
		{"agreement lost within tolerance (one case, 0.5pp)", with(func(s *qualityScore) { s.TierCorrect = 140 }), 0},
		{"agreement lost beyond tolerance", with(func(s *qualityScore) { s.TierCorrect = 139 }), 1},
		{"under-routing rose within tolerance", with(func(s *qualityScore) { s.UnderRouted = 26 }), 0},
		{"under-routing rose beyond tolerance", with(func(s *qualityScore) { s.UnderRouted = 27 }), 1},
		{"both fell", with(func(s *qualityScore) { s.TierCorrect, s.UnderRouted = 130, 40 }), 2},
		{"labels edited: digest differs", with(func(s *qualityScore) { s.Digest = "d2"; s.TierCorrect = 200 }), 1},
		{"cases removed: count differs", with(func(s *qualityScore) { s.Cases = 190; s.TierCorrect = 141 }), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := qualityRegressions("set", base, tc.got, 0.5)
			if len(got) != tc.want {
				t.Fatalf("got %d problems %q, want %d", len(got), got, tc.want)
			}
		})
	}
}

func TestQualityRegressions_ZeroToleranceFailsOnAnyLoss(t *testing.T) {
	base := qualityScore{Cases: 90, TierCorrect: 84, UnderRouted: 0, Digest: "d1"}
	one := base
	one.TierCorrect = 83
	if len(qualityRegressions("set", base, one, 0)) != 1 {
		t.Fatal("tolerance 0 must fail on a single lost case")
	}
	if len(qualityRegressions("set", base, base, 0)) != 0 {
		t.Fatal("tolerance 0 must pass on an identical score")
	}
}
