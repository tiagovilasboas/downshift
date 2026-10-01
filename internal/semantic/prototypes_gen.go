// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

package semantic

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// BuildHashPrototypes computes centroid embeddings per label using HashEmbed.
func BuildHashPrototypes(labeledPrompts map[string][]string, dim int) PrototypeStore {
	if dim <= 0 {
		dim = defaultEmbedDim
	}
	centroids := make(map[string][]float64, len(labeledPrompts))
	for label, prompts := range labeledPrompts {
		if len(prompts) == 0 {
			continue
		}
		acc := make([]float64, dim)
		for _, p := range prompts {
			v := HashEmbed(p, dim)
			for i := range acc {
				acc[i] += v[i]
			}
		}
		n := float64(len(prompts))
		for i := range acc {
			acc[i] /= n
		}
		centroids[strings.ToUpper(label)] = acc
	}
	return PrototypeStore{
		Model:     "hash-embed-v1 (deterministic; replace with MiniLM via train_prototypes.py)",
		Dim:       dim,
		Centroids: centroids,
	}
}

// WritePrototypesJSON writes a PrototypeStore to path.
func WritePrototypesJSON(path string, store PrototypeStore) error {
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

// RefreshHashPrototypesFromBenchmark reads benchmark/tasks.json and writes prototypes.
func RefreshHashPrototypesFromBenchmark(tasksJSONPath, outPath string) error {
	data, err := os.ReadFile(tasksJSONPath)
	if err != nil {
		return err
	}
	var rows []struct {
		Prompt string `json:"prompt"`
		Label  string `json:"label"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return err
	}
	byLabel := make(map[string][]string)
	for _, r := range rows {
		byLabel[r.Label] = append(byLabel[r.Label], r.Prompt)
	}
	store := BuildHashPrototypes(byLabel, defaultEmbedDim)
	if err := WritePrototypesJSON(outPath, store); err != nil {
		return err
	}
	if len(store.Centroids) == 0 {
		return fmt.Errorf("no centroids generated")
	}
	return nil
}
