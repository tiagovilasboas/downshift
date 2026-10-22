// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"testing"
)

func TestPriceBook_NilCatalog(t *testing.T) {
	pb := NewPriceBook(nil)

	if pb.Version() != "" {
		t.Errorf("Version() = %q, want empty", pb.Version())
	}
	if pb.AsOf() != "" {
		t.Errorf("AsOf() = %q, want empty", pb.AsOf())
	}

	result := pb.PriceFor("claude-sonnet-5-5")
	if result.Quality != QualityUnpriced {
		t.Errorf("Quality = %q, want %q", result.Quality, QualityUnpriced)
	}
}

func TestPriceBook_EmptyModelID(t *testing.T) {
	cat := mustLoadTestCatalog(t)
	pb := NewPriceBook(cat)

	result := pb.PriceFor("")
	if result.Quality != QualityUnpriced {
		t.Errorf("Quality = %q, want %q", result.Quality, QualityUnpriced)
	}
}

func TestPriceBook_ExactMatch(t *testing.T) {
	cat := mustLoadTestCatalog(t)
	pb := NewPriceBook(cat)

	result := pb.PriceFor("claude-sonnet-5-5")
	if result.Quality != QualityExact {
		t.Errorf("Quality = %q, want %q", result.Quality, QualityExact)
	}
	if result.ModelID != "claude-sonnet-5-5" {
		t.Errorf("ModelID = %q, want %q", result.ModelID, "claude-sonnet-5-5")
	}
	if result.InputCostM <= 0 {
		t.Error("InputCostM should be positive")
	}
	if result.OutputCostM <= 0 {
		t.Error("OutputCostM should be positive")
	}
	if result.InferredFrom != "" {
		t.Errorf("InferredFrom = %q, want empty for exact match", result.InferredFrom)
	}
}

func TestPriceBook_SlugInference(t *testing.T) {
	cat := mustLoadTestCatalog(t)
	pb := NewPriceBook(cat)

	// Note: "haiku", "sonnet", "opus" are direct aliases in the catalog,
	// so they match as exact, not inferred. The inference path is only
	// used when the slug doesn't match any entry or alias directly.
	//
	// Catalog mappings (kirocrew harness):
	//   haiku  → claude-haiku-4-5
	//   sonnet → claude-sonnet-5-5
	//   opus   → claude-opus-5-5
	cases := []struct {
		slug   string
		wantQ  Quality
		wantID string // expected ModelID (empty = same as slug for unpriced)
	}{
		// Direct aliases in catalog → exact
		{"haiku", QualityExact, "claude-haiku-4-5"},
		{"sonnet", QualityExact, "claude-sonnet-5-5"},
		{"opus", QualityExact, "claude-opus-5-5"},
		// Case variations go through inference (mapped to 5-5 versions per inferFromSlug)
		{"HAIKU", QualityInferred, "claude-haiku-5-5"},
		{"SONNET", QualityInferred, "claude-sonnet-5-5"},
		{"OPUS", QualityInferred, "claude-opus-5-5"},
		// Unknown and gibberish → unpriced
		{"unknown", QualityUnpriced, "unknown"},
		{"gibberish", QualityUnpriced, "gibberish"},
	}

	for _, c := range cases {
		t.Run(c.slug, func(t *testing.T) {
			result := pb.PriceFor(c.slug)
			if result.Quality != c.wantQ {
				t.Errorf("Quality = %q, want %q", result.Quality, c.wantQ)
			}
			if result.ModelID != c.wantID {
				t.Errorf("ModelID = %q, want %q", result.ModelID, c.wantID)
			}
			if c.wantQ == QualityInferred && result.InferredFrom != c.slug {
				t.Errorf("InferredFrom = %q, want %q", result.InferredFrom, c.slug)
			}
		})
	}
}

func TestPriceResult_ComputeCost(t *testing.T) {
	// Using typical Sonnet 5.5 prices: input $2/M, output $8/M, cache_read $0.10/M, cache_write $2.50/M
	pr := PriceResult{
		ModelID:         "claude-sonnet-5-5",
		Quality:         QualityExact,
		InputCostM:      2.0,
		OutputCostM:     8.0,
		CacheReadCostM:  0.10,
		CacheWriteCostM: 2.50,
	}

	// 1M input + 500k output + 200k cache_read + 100k cache_write
	// = $2.00 + $4.00 + $0.02 + $0.25 = $6.27
	cost := pr.ComputeCost(1_000_000, 500_000, 200_000, 100_000)
	want := 6.27
	if cost < want-0.01 || cost > want+0.01 {
		t.Errorf("ComputeCost = %.2f, want ~%.2f", cost, want)
	}
}

func TestPriceResult_ComputeCost_ZeroTokens(t *testing.T) {
	pr := PriceResult{
		Quality:     QualityExact,
		InputCostM:  2.0,
		OutputCostM: 8.0,
	}

	cost := pr.ComputeCost(0, 0, 0, 0)
	if cost != 0 {
		t.Errorf("ComputeCost(0,0,0,0) = %f, want 0", cost)
	}
}

func TestPriceResult_Predicates(t *testing.T) {
	exact := PriceResult{Quality: QualityExact}
	inferred := PriceResult{Quality: QualityInferred}
	unpriced := PriceResult{Quality: QualityUnpriced}
	upperBound := PriceResult{Quality: QualityUpperBound}

	if !exact.IsExact() {
		t.Error("exact.IsExact() = false")
	}
	if inferred.IsExact() {
		t.Error("inferred.IsExact() = true")
	}

	if !exact.IsPriced() {
		t.Error("exact.IsPriced() = false")
	}
	if !inferred.IsPriced() {
		t.Error("inferred.IsPriced() = false")
	}
	if !upperBound.IsPriced() {
		t.Error("upperBound.IsPriced() = false")
	}
	if unpriced.IsPriced() {
		t.Error("unpriced.IsPriced() = true")
	}
}

func TestQuality_Constants(t *testing.T) {
	// Ensure constants match T0 spec vocabulary.
	if QualityExact != "exact" {
		t.Errorf("QualityExact = %q", QualityExact)
	}
	if QualityUpperBound != "upper_bound" {
		t.Errorf("QualityUpperBound = %q", QualityUpperBound)
	}
	if QualityInferred != "inferred" {
		t.Errorf("QualityInferred = %q", QualityInferred)
	}
	if QualityUnpriced != "unpriced" {
		t.Errorf("QualityUnpriced = %q", QualityUnpriced)
	}
}

// mustLoadTestCatalog loads the embedded catalog for tests.
func mustLoadTestCatalog(t *testing.T) *Catalog {
	t.Helper()
	cat := Load()
	if cat == nil {
		t.Fatal("Load catalog returned nil")
	}
	return cat
}
