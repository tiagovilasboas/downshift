// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Command multilang measures authored private fixtures without retaining prompts
// in the report. Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/semantic"
)

type item struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Prompt   string `json:"prompt"`
	Expected string `json:"expected_tier"`
	Risk     bool   `json:"risk"`
}
type result struct {
	ID           string  `json:"id"`
	Language     string  `json:"language"`
	Expected     string  `json:"expected_tier"`
	Risk         bool    `json:"risk"`
	Class        string  `json:"class"`
	Tier         string  `json:"tier"`
	Confident    bool    `json:"confident"`
	Unsupported  bool    `json:"unsupported_language"`
	Milliseconds float64 `json:"milliseconds"`
}

func tier(label string) string {
	switch label {
	case "TRIVIAL", "SIMPLE":
		return "small"
	case "MEDIUM":
		return "mid"
	default:
		return "frontier"
	}
}
func run() error {
	input := flag.String("input", "", "private M3 JSON fixture")
	vectors := flag.String("vectors", "", "optional private JSON map id to vector")
	prototypes := flag.String("prototypes", "", "matching development-only centroid store")
	helperProbe := flag.String("helper-probe", "", "measure existing bounded external-helper path on first item")
	flag.Parse()
	if *input == "" {
		return fmt.Errorf("--input is required")
	}
	data, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	var fixture struct {
		Items []item `json:"items"`
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		return err
	}
	if len(fixture.Items) == 0 {
		return fmt.Errorf("empty fixture")
	}
	if *helperProbe != "" {
		start := time.Now()
		v, probeErr := semantic.HashEmbedder{}.Embed(fixture.Items[0].Prompt)
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"milliseconds": float64(time.Since(start).Nanoseconds()) / 1e6, "success": probeErr == nil, "dimensions": len(v), "measurement": "hash embedder in-process probe (helperProbe flag kept for CLI compat)"})
	}
	var vecs map[string][]float64
	var store semantic.PrototypeStore
	if (*vectors == "") != (*prototypes == "") {
		return fmt.Errorf("vectors and prototypes must be supplied together")
	}
	if *vectors != "" {
		b, e := os.ReadFile(*vectors)
		if e != nil {
			return e
		}
		var encoded struct {
			Model   string               `json:"model"`
			Vectors map[string][]float64 `json:"vectors"`
		}
		if e = json.Unmarshal(b, &encoded); e != nil {
			return e
		}
		vecs = encoded.Vectors
		store, e = semantic.LoadPrototypesFromFile(*prototypes)
		if e != nil {
			return e
		}
		if encoded.Model == "" || encoded.Model != store.Model {
			return fmt.Errorf("vectors and centroids use different or unknown models")
		}
	}
	rows := make([]result, 0, len(fixture.Items))
	seen := map[string]bool{}
	for _, it := range fixture.Items {
		if it.ID == "" || seen[it.ID] || it.Prompt == "" || (it.Expected != "small" && it.Expected != "mid" && it.Expected != "frontier") {
			return fmt.Errorf("invalid or duplicate fixture item %q", it.ID)
		}
		seen[it.ID] = true
		start := time.Now()
		c := core.ClassifyWithSemantic(it.Prompt)
		label := c.Complexity.String()
		if *vectors != "" {
			c = core.Classify(it.Prompt)
			label = c.Complexity.String()
			v, ok := vecs[it.ID]
			if !ok || len(v) != store.Dim {
				return fmt.Errorf("missing/wrong dimension vector %q", it.ID)
			}
			// Same conservative semantics as Classify: a semantic result may only
			// raise rank, never create COMPLEX alone, or bypass confidence/language.
			if !c.NoSignal && !c.Confident {
				candidate, similarity := store.Nearest(v)
				if similarity >= semantic.MinSimilarity && semantic.LabelRank(candidate) > semantic.LabelRank(label) && (candidate != semantic.LabelComplex || c.Scores[core.Complex] > 0) {
					label = candidate
				}
			}
		}
		rows = append(rows, result{it.ID, it.Language, it.Expected, it.Risk, label, tier(label), c.Confident, c.UnsupportedLanguage, float64(time.Since(start).Nanoseconds()) / 1e6})
	}
	sum := sha256.Sum256(data)
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"fixture_sha256": hex.EncodeToString(sum[:]), "mode": "offline-classifier-measurement-not-executor-proof", "rows": rows})
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
