// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	"fmt"
	"math"
)

const (
	maxPrototypeDim     = 1024
	maxPrototypeLabels  = 4
	maxPrototypeNameLen = 128
)

// Validate checks prototype store shape before shadow or tool use.
func (p PrototypeStore) Validate() error {
	if p.Dim <= 0 || p.Dim > maxPrototypeDim {
		return fmt.Errorf("invalid dim")
	}
	if len(p.Centroids) == 0 || len(p.Centroids) > maxPrototypeLabels {
		return fmt.Errorf("invalid centroid count")
	}
	if len(p.Model) > maxPrototypeNameLen {
		return fmt.Errorf("model name too long")
	}
	for label, vec := range p.Centroids {
		switch label {
		case LabelTrivial, LabelSimple, LabelMedium, LabelComplex:
		default:
			return fmt.Errorf("unknown label")
		}
		if len(vec) != p.Dim {
			return fmt.Errorf("centroid dim mismatch")
		}
		for _, v := range vec {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Errorf("non-finite centroid value")
			}
		}
	}
	return nil
}
