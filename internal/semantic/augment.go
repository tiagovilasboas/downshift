// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

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

// MaybeAugment applies a monotonic semantic boost when regex confidence is low.
// label is TRIVIAL|SIMPLE|MEDIUM|COMPLEX. Returns the new label and whether
// semantic fired. Never downgrades rank.
func MaybeAugment(prompt string, label string, confident bool) (string, bool) {
	if !enabled() {
		return label, false
	}
	store, ok := loadStore()
	if !ok {
		return label, false
	}
	emb, ok := EmbedderFromEnv()
	if !ok {
		return label, false
	}
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
	if confident && semRank <= baseRank {
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
