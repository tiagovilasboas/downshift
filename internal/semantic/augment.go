// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	"encoding/json"
	"os"
	"sync"

	_ "embed"
)

//go:embed data/prototypes.json
var embeddedPrototypes []byte

//go:embed data/minilm.json
var embeddedMiniLM []byte

var (
	loadOnce   sync.Once
	storeCache PrototypeStore
	storeOK    bool
)

func loadStore() (PrototypeStore, bool) {
	loadOnce.Do(func() {
		var p PrototypeStore
		if err := json.Unmarshal(embeddedPrototypes, &p); err != nil {
			return
		}
		if p.Dim == 0 || len(p.Centroids) == 0 {
			return
		}
		storeCache = p
		storeOK = true
	})
	return storeCache, storeOK
}

// MinSimilarity is the cosine threshold to trust a semantic label.
const MinSimilarity = 0.45

// MaybeAugment applies a monotonic semantic boost when regex confidence is low.
// label is TRIVIAL|SIMPLE|MEDIUM|COMPLEX. Returns the new label and whether
// semantic fired. Never downgrades rank.
func MaybeAugment(prompt string, label string, confident bool) (string, bool) {
	if !enabled() {
		return label, false
	}
	emb, ok := EmbedderFromEnv()
	if !ok {
		return label, false
	}
	vec, neural, err := embedWithSource(emb, prompt)
	if err != nil {
		return label, false
	}
	store, ok := storeFor(neural)
	if !ok {
		return label, false
	}
	return augmentVector(vec, label, confident, store)
}

// embedWithSource embeds prompt and reports whether the vector came from the
// external MiniLM command (true) or the hash embedder (false), including
// when the command failed and the hash fallback ran.
func embedWithSource(emb Embedder, prompt string) ([]float64, bool, error) {
	if f, ok := emb.(fallbackEmbedder); ok {
		return f.embedSource(prompt)
	}
	vec, err := emb.Embed(prompt)
	return vec, false, err
}

// storeFor returns the centroids for the space that produced the vector:
// MiniLM centroids for MiniLM vectors, hash centroids for hash vectors.
// Embedding spaces are never mixed.
func storeFor(neural bool) (PrototypeStore, bool) {
	if neural {
		return loadMiniLM()
	}
	return loadStore()
}

func loadMiniLM() (PrototypeStore, bool) {
	var p PrototypeStore
	if err := json.Unmarshal(embeddedMiniLM, &p); err != nil {
		return PrototypeStore{}, false
	}
	if p.Dim == 0 || len(p.Centroids) == 0 {
		return PrototypeStore{}, false
	}
	return p, true
}

// AugmentWith runs semantic fusion with explicit store and embedder (for tests and tools).
func AugmentWith(prompt, label string, confident bool, store PrototypeStore, emb Embedder) (string, bool) {
	vec, err := emb.Embed(prompt)
	if err != nil {
		return label, false
	}
	return augmentVector(vec, label, confident, store)
}

func augmentVector(vec []float64, label string, confident bool, store PrototypeStore) (string, bool) {
	semLabel, sim := store.Nearest(vec)
	if sim < MinSimilarity {
		return label, false
	}
	baseRank := labelRank(label)
	semRank := labelRank(semLabel)
	if semRank <= baseRank {
		return label, false
	}
	if confident {
		return label, false
	}
	return semLabel, true
}

// LoadPrototypesFromFile loads a PrototypeStore for training/benchmark tools.
func LoadPrototypesFromFile(path string) (PrototypeStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PrototypeStore{}, err
	}
	var p PrototypeStore
	if err := json.Unmarshal(data, &p); err != nil {
		return PrototypeStore{}, err
	}
	return p, nil
}
