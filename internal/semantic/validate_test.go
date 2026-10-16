// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package semantic_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/semantic"
)

func TestPrototypeStoreValidate(t *testing.T) {
	ok := semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
		},
	}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			"UNKNOWN": {1, 0},
		},
	}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected unknown label error")
	}
}
