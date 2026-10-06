// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package semantic

import "testing"

func TestCosine_Identical(t *testing.T) {
	if cosine([]float64{1, 0}, []float64{1, 0}) < 0.99 {
		t.Fatal("expected ~1 for identical vectors")
	}
}

func TestCosine_Orthogonal(t *testing.T) {
	if cosine([]float64{1, 0}, []float64{0, 1}) > 1e-9 {
		t.Fatal("expected 0 for orthogonal vectors")
	}
}
