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
	store, ok := storeFor(emb)
	if !ok {
		return label, false
	}
	return AugmentWith(prompt, label, confident, store, emb)
}

func storeFor(emb Embedder) (PrototypeStore, bool) {
	if _, cmd := emb.(fallbackEmbedder); cmd {
		if s, ok := loadMiniLM(); ok {
			return s, true
		}
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
