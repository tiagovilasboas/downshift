// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	"math"
	"testing"
)

// hashBaselinePrompt is the fixed fixture for the shipped hash embedder.
// Leading values are float64 round-trip literals: normalization couples every
// dimension, so a bit-rule change moves them.
const hashBaselinePrompt = "rename userId to accountId"

func TestHashEmbed_Deterministic(t *testing.T) {
	const dim = 384
	a := HashEmbed(hashBaselinePrompt, dim)
	b := HashEmbed(hashBaselinePrompt, dim)
	if len(a) != len(b) {
		t.Fatalf("length %d != %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] || math.Float64bits(a[i]) != math.Float64bits(b[i]) {
			t.Fatalf("calls differ at %d: %g != %g", i, a[i], b[i])
		}
	}
}

func TestHashEmbed_NonEmptyUnitLength(t *testing.T) {
	vec := HashEmbed(hashBaselinePrompt, 384)
	got := hashEmbedL2(vec)
	if math.Abs(got-1) > 1e-9 {
		t.Fatalf("L2 = %.17g, want about 1", got)
	}
}

func TestHashEmbed_EmptyPromptIsZero(t *testing.T) {
	const dim = 384
	vec := HashEmbed("", dim)
	if len(vec) != dim {
		t.Fatalf("len = %d, want %d", len(vec), dim)
	}
	for i, v := range vec {
		if v != 0 {
			t.Fatalf("vec[%d] = %g, want 0", i, v)
		}
	}
}

func TestHashEmbed_BaselineFixture(t *testing.T) {
	const dim = 384
	vec := HashEmbed(hashBaselinePrompt, dim)
	if len(vec) != dim {
		t.Fatalf("len = %d, want %d", len(vec), dim)
	}

	want := []float64{
		0.07104537980986436,
		0.028418151923945742,
		0.028418151923945742,
		-0.014209075961972871,
		0.07104537980986436,
		0.07104537980986436,
		0.028418151923945742,
		-0.014209075961972871,
	}
	for i, w := range want {
		if vec[i] != w {
			t.Fatalf("vec[%d] = %.17g, want %.17g", i, vec[i], w)
		}
	}

	const wantL2 = 1.0000000000000016
	if got := hashEmbedL2(vec); got != wantL2 {
		t.Fatalf("L2 = %.17g, want %.17g", got, wantL2)
	}
}

func TestHashEmbed_DimChangesLength(t *testing.T) {
	short := HashEmbed(hashBaselinePrompt, 16)
	long := HashEmbed(hashBaselinePrompt, 384)
	if len(short) == len(long) {
		t.Fatalf("dim 16 and dim 384 returned the same length %d", len(short))
	}
	if len(short) != 16 || len(long) != 384 {
		t.Fatalf("lengths = %d and %d, want 16 and 384", len(short), len(long))
	}
}

func hashEmbedL2(v []float64) float64 {
	var sum float64
	for _, x := range v {
		sum += x * x
	}
	return math.Sqrt(sum)
}
