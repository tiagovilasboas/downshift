// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package semantic

import "fmt"

// StaticEmbedder returns a fixed vector for every prompt (tests only).
type StaticEmbedder struct {
	Vector []float64
}

func (s StaticEmbedder) Embed(_ string) ([]float64, error) {
	if len(s.Vector) == 0 {
		return nil, fmt.Errorf("empty static vector")
	}
	return s.Vector, nil
}
