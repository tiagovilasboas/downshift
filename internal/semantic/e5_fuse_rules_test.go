// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package semantic_test

// harness-downshift by Tiago de Carvalho Vilas Boas
// https://github.com/tiagovilasboas/downshift

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/semantic"
)

// Documented E5 nearest-label fuse (Phase E). AugmentVector is the only
// production function shadow and a future Route fuse may call: lift is
// monotonic, never downshifts, abstains below MinSimilarity and when regex
// is already confident. Core still refuses NoSignal lifts and COMPLEX with
// no regex Complex score (see classifyWithSemantic); those extra guards are
// asserted in internal/core/phase_e_fuse_acceptance_test.go.

func TestE5NearestLabel_FuseRulesTable(t *testing.T) {
	store := semantic.PrototypeStore{
		Dim: 3,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0, 0},
			semantic.LabelMedium:  {0, 1, 0},
			semantic.LabelComplex: {0, 0, 1},
		},
	}

	// Vectors are aligned with a centroid or orthogonal (cosine 0).
	vecTrivial := []float64{1, 0, 0}
	vecMedium := []float64{0, 1, 0}
	vecComplex := []float64{0, 0, 1}
	vecOrthogonal := []float64{0.0, 0.0, 0.0} // cosine 0 against every centroid → abstain

	cases := []struct {
		name      string
		vec       []float64
		regex     string
		confident bool
		want      string
		lift      bool
	}{
		{
			name:      "abstain_below_similarity",
			vec:       vecOrthogonal,
			regex:     semantic.LabelTrivial,
			confident: false,
			want:      semantic.LabelTrivial,
			lift:      false,
		},
		{
			name:      "abstain_when_regex_confident",
			vec:       vecComplex,
			regex:     semantic.LabelTrivial,
			confident: true,
			want:      semantic.LabelTrivial,
			lift:      false,
		},
		{
			name:      "abstain_equal_rank",
			vec:       vecMedium,
			regex:     semantic.LabelMedium,
			confident: false,
			want:      semantic.LabelMedium,
			lift:      false,
		},
		{
			name:      "never_downgrade_risk_or_otherwise",
			vec:       vecTrivial,
			regex:     semantic.LabelComplex,
			confident: false,
			want:      semantic.LabelComplex,
			lift:      false,
		},
		{
			name:      "never_downgrade_from_medium_to_trivial",
			vec:       vecTrivial,
			regex:     semantic.LabelMedium,
			confident: false,
			want:      semantic.LabelMedium,
			lift:      false,
		},
		{
			name:      "lift_unconfident_trivial_to_medium",
			vec:       vecMedium,
			regex:     semantic.LabelTrivial,
			confident: false,
			want:      semantic.LabelMedium,
			lift:      true,
		},
		{
			name:      "lift_unconfident_medium_to_complex",
			vec:       vecComplex,
			regex:     semantic.LabelMedium,
			confident: false,
			want:      semantic.LabelComplex,
			lift:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, lift := semantic.AugmentVector(tc.vec, tc.regex, tc.confident, store)
			if lift != tc.lift {
				t.Fatalf("lift=%v want %v (got %q regex %q)", lift, tc.lift, got, tc.regex)
			}
			if semantic.LabelRank(got) < semantic.LabelRank(tc.regex) {
				t.Fatalf("fuse downgraded %s -> %s", tc.regex, got)
			}
			if tc.want != "" && got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			if tc.lift && semantic.LabelRank(got) <= semantic.LabelRank(tc.regex) {
				t.Fatalf("lifted but rank did not rise: %s -> %s", tc.regex, got)
			}
			if !tc.lift && got != tc.regex {
				t.Fatalf("abstain must keep regex label, got %q", got)
			}
		})
	}
}

func TestE5NearestLabel_MinSimilarityBoundary(t *testing.T) {
	store := semantic.PrototypeStore{
		Dim: 3,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0, 0},
			semantic.LabelComplex: {0, 0, 1},
		},
	}
	if semantic.MinSimilarity != 0.45 {
		t.Fatalf("MinSimilarity=%v, fuse tests assume 0.45", semantic.MinSimilarity)
	}

	// Nearest is COMPLEX (other centroid is orthogonal) but cosine 0.287 < 0.45.
	below := []float64{0, 1, 0.3}
	got, lift := semantic.AugmentVector(below, semantic.LabelTrivial, false, store)
	if lift {
		t.Fatalf("expected abstain below MinSimilarity, got %q", got)
	}

	above := []float64{0, 0, 1}
	got, lift = semantic.AugmentVector(above, semantic.LabelTrivial, false, store)
	if !lift || got != semantic.LabelComplex {
		t.Fatalf("expected COMPLEX lift above threshold, got %q lift=%v", got, lift)
	}
}
