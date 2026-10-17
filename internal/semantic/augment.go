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

// MaybeAugment applies a monotonic in-process hash semantic boost when regex
// confidence is low. label is TRIVIAL|SIMPLE|MEDIUM|COMPLEX. Returns the new
// label and whether semantic fired. Never downgrades rank. Honors
// DOWNSHIFT_MINILM=0.
func MaybeAugment(prompt string, label string, confident bool) (string, bool) {
	if !enabled() {
		return label, false
	}
	vec, err := HashEmbedder{}.Embed(prompt)
	if err != nil {
		return label, false
	}
	store, ok := loadStore()
	if !ok {
		return label, false
	}
	return augmentVector(vec, label, confident, store)
}

// AugmentWith runs semantic fusion with explicit store and embedder (for tests and tools).
func AugmentWith(prompt, label string, confident bool, store PrototypeStore, emb Embedder) (string, bool) {
	vec, err := emb.Embed(prompt)
	if err != nil {
		return label, false
	}
	return augmentVector(vec, label, confident, store)
}

// AugmentVector applies monotonic fusion on a precomputed embedding (same rules
// as AugmentWith, without re-embedding). Used by shadow observers and tools.
func AugmentVector(vec []float64, label string, confident bool, store PrototypeStore) (string, bool) {
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
