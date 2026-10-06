// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	"hash/fnv"
	"math"
	"strings"
)

const defaultEmbedDim = 384

// HashEmbed produces a deterministic unit vector from prompt text.
// Used to build shipped prototypes without Python; pair with
// DOWNSHIFT_MINILM_EMBED=hash for the same algorithm at runtime.
func HashEmbed(prompt string, dim int) []float64 {
	if dim <= 0 {
		dim = defaultEmbedDim
	}
	vec := make([]float64, dim)
	words := strings.Fields(strings.ToLower(prompt))
	if len(words) == 0 {
		return vec
	}
	for _, w := range words {
		h := fnv.New64a()
		h.Write([]byte(w))
		sum := h.Sum64()
		for i := 0; i < dim; i++ {
			// Spread bits across dimensions.
			bit := (sum >> (i % 56)) & 1
			if bit == 1 {
				vec[i] += 1
			} else {
				vec[i] -= 0.5
			}
		}
	}
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	if norm == 0 {
		return vec
	}
	norm = math.Sqrt(norm)
	for i := range vec {
		vec[i] /= norm
	}
	return vec
}

// HashEmbedder implements Embedder using HashEmbed.
type HashEmbedder struct {
	Dim int
}

func (h HashEmbedder) Embed(prompt string) ([]float64, error) {
	dim := h.Dim
	if dim == 0 {
		dim = defaultEmbedDim
	}
	return HashEmbed(prompt, dim), nil
}
