// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// harness-downshift by Tiago de Carvalho Vilas Boas
// https://github.com/tiagovilasboas/downshift

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/semantic"
)

// documentedE5Fuse is the Phase E contract fullstack must keep when wiring
// nearest-label into classifyWithSemantic / Route:
//   - AugmentVector rules (threshold, confident abstain, monotonic raise only)
//   - no-signal prompts never lift
//   - COMPLEX requires a regex Complex score (never sole-source frontier)
//   - language mismatch may raise but the fuse keeps Confident=false
func documentedE5Fuse(base core.Classification, nearest string, sim float64) (string, bool) {
	regex := base.Complexity.String()
	if base.NoSignal {
		return regex, false
	}
	if sim < semantic.MinSimilarity {
		return regex, false
	}
	if base.Confident {
		return regex, false
	}
	if semantic.LabelRank(nearest) <= semantic.LabelRank(regex) {
		return regex, false
	}
	if nearest == semantic.LabelComplex && base.Scores[core.Complex] == 0 {
		return regex, false
	}
	return nearest, true
}

func TestPhaseE_DocumentedFuseAgreesWithAugmentVectorGuards(t *testing.T) {
	store := semantic.PrototypeStore{
		Dim: 3,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0, 0},
			semantic.LabelMedium:  {0, 1, 0},
			semantic.LabelComplex: {0, 0, 1},
		},
	}

	type row struct {
		name      string
		vec       []float64
		base      core.Classification
		wantLabel string
		wantLift  bool
	}
	cases := []row{
		{
			name: "lift_unconfident_with_regex_evidence",
			vec:  []float64{0, 0, 1},
			base: core.Classification{
				Complexity: core.Medium,
				Confident:  false,
				Scores:     map[core.Complexity]int{core.Medium: 2, core.Complex: 1},
			},
			wantLabel: semantic.LabelComplex,
			wantLift:  true,
		},
		{
			name: "abstain_bare_complex_without_regex_score",
			vec:  []float64{0, 0, 1},
			base: core.Classification{
				Complexity: core.Medium,
				Confident:  false,
				Scores:     map[core.Complexity]int{core.Medium: 2},
			},
			wantLabel: semantic.LabelMedium,
			wantLift:  false,
		},
		{
			name: "abstain_no_signal",
			vec:  []float64{0, 0, 1},
			base: core.Classification{
				Complexity: core.Medium,
				Confident:  false,
				NoSignal:   true,
				Scores:     map[core.Complexity]int{},
			},
			wantLabel: semantic.LabelMedium,
			wantLift:  false,
		},
		{
			name: "abstain_confident_regex",
			vec:  []float64{0, 0, 1},
			base: core.Classification{
				Complexity: core.Trivial,
				Confident:  true,
				Scores:     map[core.Complexity]int{core.Trivial: 4, core.Complex: 1},
			},
			wantLabel: semantic.LabelTrivial,
			wantLift:  false,
		},
		{
			name: "never_sole_source_downshift_on_risk_class",
			vec:  []float64{1, 0, 0},
			base: core.Classification{
				Complexity: core.Complex,
				Confident:  false,
				RiskFloor:  true,
				Scores:     map[core.Complexity]int{core.Complex: 3},
			},
			wantLabel: semantic.LabelComplex,
			wantLift:  false,
		},
		{
			name: "unsupported_language_may_raise_unconfident",
			vec:  []float64{0, 1, 0},
			base: core.Classification{
				Complexity:          core.Trivial,
				Confident:           false,
				UnsupportedLanguage: true,
				Scores:              map[core.Complexity]int{core.Trivial: 1},
			},
			wantLabel: semantic.LabelMedium,
			wantLift:  true,
		},
		{
			name: "abstain_below_threshold",
			vec:  []float64{0, 0, 0}, // cosine 0 against every centroid
			base: core.Classification{
				Complexity: core.Trivial,
				Confident:  false,
				Scores:     map[core.Complexity]int{core.Trivial: 1, core.Complex: 1},
			},
			wantLabel: semantic.LabelTrivial,
			wantLift:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nearest, sim := store.Nearest(tc.vec)
			got, lift := documentedE5Fuse(tc.base, nearest, sim)
			if got != tc.wantLabel || lift != tc.wantLift {
				t.Fatalf("documented fuse: nearest=%s sim=%.3f -> %q lift=%v, want %q lift=%v",
					nearest, sim, got, lift, tc.wantLabel, tc.wantLift)
			}
			aug, augLift := semantic.AugmentVector(tc.vec, tc.base.Complexity.String(), tc.base.Confident, store)
			if semantic.LabelRank(aug) < semantic.LabelRank(tc.base.Complexity.String()) {
				t.Fatalf("AugmentVector downgraded %s -> %s", tc.base.Complexity, aug)
			}
			// Extra core guards may abstain when AugmentVector would lift.
			if lift && !augLift {
				t.Fatalf("documented fuse lifted while AugmentVector abstained")
			}
			if lift && got != aug {
				t.Fatalf("documented lift label %q != AugmentVector %q", got, aug)
			}
			if tc.base.NoSignal && augLift && lift {
				t.Fatal("no-signal must not lift")
			}
		})
	}
}

func TestPhaseE_ClassifyWithSemanticKeepsNoSignalAndBareComplexGuards(t *testing.T) {
	t.Setenv("DOWNSHIFT_MINILM", "1")
	t.Setenv("DOWNSHIFT_MINILM_EMBED", "hash")
	t.Setenv(core.NBUpshiftEnv, "")
	t.Setenv(core.NBDownshiftEnv, "")
	t.Setenv("DOWNSHIFT_GRAPHIFY", "off")
	t.Setenv("DOWNSHIFT_GRAPHIFY_CMD", "")

	noSignal := []string{"hook", "payload", "lorem ipsum dolor sit amet"}
	for i, p := range noSignal {
		base := core.Classify(p)
		if !base.NoSignal {
			t.Fatalf("precondition case %d: expected NoSignal", i)
		}
		sem := core.ClassifyWithSemantic(p)
		if sem.Complexity != base.Complexity || sem.Confident != base.Confident {
			t.Errorf("case %d: no-signal fuse changed classification: base=%+v sem=%+v", i, base, sem)
		}
		if sem.SemanticLift {
			t.Errorf("case %d: no-signal must not SemanticLift", i)
		}
	}

	// Trivial English rename: regex has no Complex score; fusion must not
	// invent COMPLEX / frontier on its own.
	p := h14PureEnglishTrivialPrompts()[0]
	base := core.Classify(p)
	sem := core.ClassifyWithSemantic(p)
	if base.Scores[core.Complex] == 0 && sem.Complexity == core.Complex {
		t.Fatalf("bare COMPLEX from semantic fuse: base=%+v sem=%+v", base, sem)
	}
}
