// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build ignore

// Command baseline scores the in-tree hash embedder on a development file.
// Development-only: in-sample risk if prototypes share construction with this
// file. The printed accuracy is not a production-quality number and is not a
// holdout result.
//
// Downshift by Tiago de Carvalho Vilas Boas
// https://github.com/tiagovilasboas/downshift
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/semantic"
)

// frozenNeedles are path fragments this delivery must not open.
var frozenNeedles = []string{
	"holdout",
	"heldout",
	"blind",
	"fresh",
	"m3-v1",
	"outcomes-holdout",
	"legacy-splits",
}

type example struct {
	prompt string
	label  string
	hash   uint64
}

func main() {
	split := flag.String("split", "", "path to a development JSON file of labeled prompts")
	flag.Parse()
	if strings.TrimSpace(*split) == "" {
		fmt.Fprintln(os.Stderr, "usage: go run tools/hashv2/baseline.go --split <path>")
		os.Exit(1)
	}
	if pathFrozen(*split) {
		fmt.Fprintf(os.Stderr, "error: refusing frozen or holdout path %q\n", *split)
		os.Exit(2)
	}

	raw, err := os.ReadFile(*split)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read split: %v\n", err)
		os.Exit(1)
	}
	rows, err := parseExamples(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parse split: %v\n", err)
		os.Exit(1)
	}
	if len(rows) < 2 {
		fmt.Fprintf(os.Stderr, "error: need at least 2 labeled prompts, got %d\n", len(rows))
		os.Exit(1)
	}

	fit, score := splitHalves(rows)
	labeled := map[string][]string{}
	for _, row := range fit {
		labeled[row.label] = append(labeled[row.label], row.prompt)
	}
	store := semantic.BuildHashPrototypes(labeled, 0)
	if len(store.Centroids) == 0 {
		fmt.Fprintln(os.Stderr, "error: fit half produced no centroids")
		os.Exit(1)
	}

	correct := 0
	perLabel := map[string][2]int{}
	confusion := map[string]int{}
	for _, row := range score {
		vec := semantic.HashEmbed(row.prompt, store.Dim)
		pred, _ := store.Nearest(vec)
		gold := row.label
		cell := perLabel[gold]
		cell[1]++
		if pred == gold {
			correct++
			cell[0]++
		}
		perLabel[gold] = cell
		confusion[gold+"->"+pred]++
	}

	lo, hi := wilson95(correct, len(score))
	acc := 0.0
	if len(score) > 0 {
		acc = float64(correct) / float64(len(score))
	}

	fmt.Println("note=development-only; in-sample risk if prototypes share construction with this file; not production quality; not a holdout result")
	fmt.Printf("n_fit=%d\n", len(fit))
	fmt.Printf("n_score=%d\n", len(score))
	fmt.Printf("correct=%d\n", correct)
	fmt.Printf("tier_accuracy=%.6f\n", acc)
	fmt.Printf("wilson95=%.6f,%.6f\n", lo, hi)

	labels := make([]string, 0, len(perLabel))
	for label := range perLabel {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		cell := perLabel[label]
		fmt.Printf("counts %s %d/%d\n", label, cell[0], cell[1])
	}

	pairs := make([]string, 0, len(confusion))
	for pair := range confusion {
		pairs = append(pairs, pair)
	}
	sort.Strings(pairs)
	for _, pair := range pairs {
		fmt.Printf("confusion %s %d\n", pair, confusion[pair])
	}
}

func pathFrozen(path string) bool {
	candidates := []string{path, filepath.Clean(path)}
	if abs, err := filepath.Abs(path); err == nil {
		candidates = append(candidates, abs)
	}
	for _, candidate := range candidates {
		folded := strings.ToLower(filepath.ToSlash(candidate))
		for _, needle := range frozenNeedles {
			if strings.Contains(folded, needle) {
				return true
			}
		}
	}
	return false
}

func parseExamples(raw []byte) ([]example, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty file")
	}
	if raw[0] == '[' {
		var items []struct {
			Prompt string `json:"prompt"`
			Label  string `json:"label"`
		}
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		rows := make([]example, 0, len(items))
		for _, item := range items {
			if strings.TrimSpace(item.Prompt) == "" || strings.TrimSpace(item.Label) == "" {
				return nil, fmt.Errorf("array items need prompt and label")
			}
			rows = append(rows, newExample(item.Prompt, item.Label))
		}
		return rows, nil
	}

	var doc struct {
		Labels map[string][]string `json:"labels"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if len(doc.Labels) == 0 {
		return nil, fmt.Errorf("expected a {prompt,label} array or a labels map")
	}
	keys := make([]string, 0, len(doc.Labels))
	for label := range doc.Labels {
		keys = append(keys, label)
	}
	sort.Strings(keys)
	var rows []example
	for _, label := range keys {
		if strings.TrimSpace(label) == "" {
			return nil, fmt.Errorf("empty label")
		}
		for _, prompt := range doc.Labels[label] {
			if strings.TrimSpace(prompt) == "" {
				return nil, fmt.Errorf("empty prompt")
			}
			rows = append(rows, newExample(prompt, label))
		}
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("labels map has no prompts")
	}
	return rows, nil
}

func newExample(prompt, label string) example {
	sum := fnv.New64a()
	_, _ = sum.Write([]byte(prompt))
	return example{
		prompt: prompt,
		label:  strings.ToUpper(strings.TrimSpace(label)),
		hash:   sum.Sum64(),
	}
}

// splitHalves assigns a deterministic half by sorting on the FNV-64a of the
// prompt. The first half fits prototypes; the second half is scored. No RNG.
func splitHalves(rows []example) (fit, score []example) {
	sorted := append([]example(nil), rows...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].hash != sorted[j].hash {
			return sorted[i].hash < sorted[j].hash
		}
		if sorted[i].prompt != sorted[j].prompt {
			return sorted[i].prompt < sorted[j].prompt
		}
		return sorted[i].label < sorted[j].label
	})
	mid := len(sorted) / 2
	return sorted[:mid], sorted[mid:]
}

// wilson95 is the 95% Wilson score interval for k successes out of n.
func wilson95(k, n int) (float64, float64) {
	if n == 0 {
		return 0, 0
	}
	const z = 1.959963984540054
	p := float64(k) / float64(n)
	nf := float64(n)
	den := 1 + z*z/nf
	mid := (p + z*z/(2*nf)) / den
	half := z * math.Sqrt(p*(1-p)/nf+z*z/(4*nf*nf)) / den
	return math.Max(0, mid-half), math.Min(1, mid+half)
}
