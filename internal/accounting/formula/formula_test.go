// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package formula

import (
	"math"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
)

func mustPriceBook(t *testing.T) *catalog.PriceBook {
	t.Helper()
	cat := catalog.Load()
	if cat == nil {
		t.Fatal("catalog.Load() returned nil")
	}
	return catalog.NewPriceBook(cat)
}

// --- Golden tests: known inputs → expected outputs ---

func TestSaved_EmptyBuckets(t *testing.T) {
	pb := mustPriceBook(t)
	result := Saved("claude-sonnet-5-5", nil, pb)

	if result.Quality != SummaryExact {
		t.Errorf("Quality = %q, want %q", result.Quality, SummaryExact)
	}
	if result.Saved != 0 {
		t.Errorf("Saved = %f, want 0", result.Saved)
	}
	if result.TotalTokens != 0 {
		t.Errorf("TotalTokens = %d, want 0", result.TotalTokens)
	}
}

func TestSaved_NilPriceBook(t *testing.T) {
	result := Saved("claude-sonnet-5-5", []TokenBucket{{Model: "haiku"}}, nil)

	if result.Quality != SummaryUnpriced {
		t.Errorf("Quality = %q, want %q", result.Quality, SummaryUnpriced)
	}
}

func TestSaved_UnpricedBaseline(t *testing.T) {
	pb := mustPriceBook(t)
	result := Saved("unknown-model-xyz", []TokenBucket{{Model: "haiku"}}, pb)

	if result.Quality != SummaryUnpriced {
		t.Errorf("Quality = %q, want %q", result.Quality, SummaryUnpriced)
	}
	if result.IsValid() {
		t.Error("IsValid should be false for unpriced baseline")
	}
}

func TestSaved_AllSameModel_ZeroSaving(t *testing.T) {
	pb := mustPriceBook(t)
	buckets := []TokenBucket{
		{Model: "claude-sonnet-5-5", InputTokens: 1000, OutputTokens: 500},
		{Model: "claude-sonnet-5-5", InputTokens: 2000, OutputTokens: 1000},
	}
	result := Saved("claude-sonnet-5-5", buckets, pb)

	if result.Quality != SummaryExact {
		t.Errorf("Quality = %q, want %q", result.Quality, SummaryExact)
	}
	if math.Abs(result.Saved) > 0.0001 {
		t.Errorf("Saved = %f, want ~0 (same model)", result.Saved)
	}
	// Allow small floating point difference.
	if math.Abs(result.ActualCost-result.BaselineCost) > 0.0001 {
		t.Errorf("ActualCost (%f) != BaselineCost (%f)", result.ActualCost, result.BaselineCost)
	}
}

func TestSaved_CheaperModel_PositiveSaving(t *testing.T) {
	pb := mustPriceBook(t)
	// Run on haiku (cheaper) but requested sonnet.
	buckets := []TokenBucket{
		{Model: "claude-haiku-4-5", InputTokens: 100000, OutputTokens: 50000},
	}
	result := Saved("claude-sonnet-5-5", buckets, pb)

	if !result.IsValid() {
		t.Fatal("result should be valid")
	}
	if result.Saved <= 0 {
		t.Errorf("Saved = %f, want positive (haiku cheaper than sonnet)", result.Saved)
	}
	if result.ActualCost >= result.BaselineCost {
		t.Errorf("ActualCost (%f) should be < BaselineCost (%f)", result.ActualCost, result.BaselineCost)
	}
	if result.SavedPercent() <= 0 {
		t.Errorf("SavedPercent = %f, want positive", result.SavedPercent())
	}
}

func TestSaved_ExpensiveModel_NegativeSaving(t *testing.T) {
	pb := mustPriceBook(t)
	// Run on opus (more expensive) but requested sonnet.
	buckets := []TokenBucket{
		{Model: "claude-opus-5-5", InputTokens: 100000, OutputTokens: 50000},
	}
	result := Saved("claude-sonnet-5-5", buckets, pb)

	if !result.IsValid() {
		t.Fatal("result should be valid")
	}
	if result.Saved >= 0 {
		t.Errorf("Saved = %f, want negative (opus more expensive than sonnet)", result.Saved)
	}
	if result.SavedPositive() {
		t.Error("SavedPositive should be false for negative saving")
	}
}

func TestSaved_MixedModels(t *testing.T) {
	pb := mustPriceBook(t)
	// Mix of cheaper and same model.
	buckets := []TokenBucket{
		{Model: "claude-haiku-4-5", InputTokens: 50000, OutputTokens: 25000},
		{Model: "claude-sonnet-5-5", InputTokens: 50000, OutputTokens: 25000},
	}
	result := Saved("claude-sonnet-5-5", buckets, pb)

	if !result.IsValid() {
		t.Fatal("result should be valid")
	}
	// Half on cheaper model → positive saving.
	if result.Saved <= 0 {
		t.Errorf("Saved = %f, want positive (half on cheaper model)", result.Saved)
	}
	if result.TotalTokens != 150000 {
		t.Errorf("TotalTokens = %d, want 150000", result.TotalTokens)
	}
}

func TestSaved_InferredModel_MixedQuality(t *testing.T) {
	pb := mustPriceBook(t)
	// Use uppercase HAIKU which goes through inference.
	buckets := []TokenBucket{
		{Model: "HAIKU", InputTokens: 100000, OutputTokens: 50000},
	}
	result := Saved("claude-sonnet-5-5", buckets, pb)

	if result.Quality != SummaryMixed {
		t.Errorf("Quality = %q, want %q (inferred model)", result.Quality, SummaryMixed)
	}
	if result.InferredBuckets != 1 {
		t.Errorf("InferredBuckets = %d, want 1", result.InferredBuckets)
	}
}

func TestSaved_UnpricedBucket_PartialQuality(t *testing.T) {
	pb := mustPriceBook(t)
	// One bucket with unknown model.
	buckets := []TokenBucket{
		{Model: "unknown-model", InputTokens: 50000, OutputTokens: 25000},
		{Model: "claude-haiku-4-5", InputTokens: 50000, OutputTokens: 25000},
	}
	result := Saved("claude-sonnet-5-5", buckets, pb)

	if result.Quality != SummaryPartial {
		t.Errorf("Quality = %q, want %q (unpriced bucket)", result.Quality, SummaryPartial)
	}
	if result.UnpricedBuckets != 1 {
		t.Errorf("UnpricedBuckets = %d, want 1", result.UnpricedBuckets)
	}
}

func TestSaved_CacheTokens(t *testing.T) {
	pb := mustPriceBook(t)
	buckets := []TokenBucket{
		{
			Model:            "claude-haiku-4-5",
			InputTokens:      10000,
			OutputTokens:     5000,
			CacheReadTokens:  20000,
			CacheWriteTokens: 10000,
		},
	}
	result := Saved("claude-sonnet-5-5", buckets, pb)

	if !result.IsValid() {
		t.Fatal("result should be valid")
	}
	if result.TotalTokens != 45000 {
		t.Errorf("TotalTokens = %d, want 45000", result.TotalTokens)
	}
	// Baseline should include cache tokens priced at sonnet rates.
	if result.BaselineCost <= 0 {
		t.Error("BaselineCost should be positive")
	}
}

// --- Property tests ---

func TestSaved_Property_SameModelZeroSaving(t *testing.T) {
	pb := mustPriceBook(t)
	models := []string{"claude-sonnet-5-5", "claude-haiku-4-5", "claude-opus-5-5"}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			buckets := []TokenBucket{
				{Model: model, InputTokens: 1000, OutputTokens: 500},
			}
			result := Saved(model, buckets, pb)

			if !result.IsValid() {
				t.Skip("model not priced")
			}
			if math.Abs(result.Saved) > 0.0001 {
				t.Errorf("Saved = %f, want ~0 for same model", result.Saved)
			}
		})
	}
}

func TestSaved_Property_SavedNonNegativeWhenAllCheaper(t *testing.T) {
	pb := mustPriceBook(t)
	// All buckets on haiku, baseline on sonnet → Saved >= 0.
	for i := 1; i <= 5; i++ {
		buckets := make([]TokenBucket, i)
		for j := 0; j < i; j++ {
			buckets[j] = TokenBucket{
				Model:        "claude-haiku-4-5",
				InputTokens:  int64(1000 * (j + 1)),
				OutputTokens: int64(500 * (j + 1)),
			}
		}
		result := Saved("claude-sonnet-5-5", buckets, pb)

		if result.Saved < 0 {
			t.Errorf("%d buckets: Saved = %f, want >= 0", i, result.Saved)
		}
	}
}

// --- TokenBucket tests ---

func TestTokenBucket_TotalTokens(t *testing.T) {
	b := TokenBucket{
		InputTokens:      100,
		OutputTokens:     50,
		CacheReadTokens:  200,
		CacheWriteTokens: 150,
	}
	if got := b.TotalTokens(); got != 500 {
		t.Errorf("TotalTokens() = %d, want 500", got)
	}
}

func TestTokenBucket_TotalTokens_Zero(t *testing.T) {
	b := TokenBucket{}
	if got := b.TotalTokens(); got != 0 {
		t.Errorf("TotalTokens() = %d, want 0", got)
	}
}

// --- SavedResult method tests ---

func TestSavedResult_SavedPercent_ZeroBaseline(t *testing.T) {
	r := SavedResult{BaselineCost: 0, Saved: 100}
	if got := r.SavedPercent(); got != 0 {
		t.Errorf("SavedPercent() = %f, want 0 for zero baseline", got)
	}
}

func TestSavedResult_SavedPercent_Invalid(t *testing.T) {
	r := SavedResult{Quality: SummaryUnpriced, BaselineCost: 100, Saved: 50}
	if got := r.SavedPercent(); got != 0 {
		t.Errorf("SavedPercent() = %f, want 0 for invalid result", got)
	}
}

func TestSavedResult_SavedPercent_Valid(t *testing.T) {
	r := SavedResult{
		Quality:      SummaryExact,
		BaselineCost: 100,
		Saved:        25,
	}
	if got := r.SavedPercent(); math.Abs(got-25) > 0.01 {
		t.Errorf("SavedPercent() = %f, want 25", got)
	}
}
