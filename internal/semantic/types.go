// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

// Package semantic provides an optional local MiniLM embedding boost for the
// regex classifier. It never calls an LLM and never downgrades complexity:
// semantic signal may only preserve or raise the class when regex confidence
// is low (Jev/Laya-style decision layer beside generation).
//
// On by default. Opt out with DOWNSHIFT_MINILM=0.
// The local hash embedder always runs. DOWNSHIFT_MINILM_EMBED may point at
// tools/minilm/embed_stdin.py; if that command fails, hash is the fallback.
package semantic

// Label names match benchmark complexity labels.
const (
	LabelTrivial = "TRIVIAL"
	LabelSimple  = "SIMPLE"
	LabelMedium  = "MEDIUM"
	LabelComplex = "COMPLEX"
)

// PrototypeStore holds centroid embeddings per complexity label.
type PrototypeStore struct {
	Model     string               `json:"model"`
	Dim       int                  `json:"dim"`
	Centroids map[string][]float64 `json:"centroids"`
}

// Nearest returns the best-matching label and cosine similarity.
func (p PrototypeStore) Nearest(vec []float64) (label string, score float64) {
	if p.Dim > 0 && len(vec) != p.Dim {
		return "", 0
	}
	bestLabel := ""
	bestScore := -1.0
	for label, centroid := range p.Centroids {
		s := cosine(vec, centroid)
		if s > bestScore {
			bestScore = s
			bestLabel = label
		}
	}
	return bestLabel, bestScore
}

// LabelRank orders complexity labels for monotonic fusion.
func LabelRank(label string) int {
	return labelRank(label)
}

func labelRank(label string) int {
	switch label {
	case LabelTrivial:
		return 0
	case LabelSimple:
		return 1
	case LabelMedium:
		return 2
	case LabelComplex:
		return 3
	default:
		return 1
	}
}
