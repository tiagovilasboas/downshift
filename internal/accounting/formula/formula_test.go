// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package formula

import (
	"encoding/json"
	"math"
	"os"
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
	if pct, ok := result.SavedPercent(); !ok || pct <= 0 {
		t.Errorf("SavedPercent = %f (ok=%v), want positive", pct, ok)
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

func TestSaved_UnpricedBucket_ExcludedFromCosts(t *testing.T) {
	pb := mustPriceBook(t)
	// The unknown bucket is 10x larger than the priced one. If it leaked into
	// either cost, the figures below would move.
	buckets := []TokenBucket{
		{Model: "unknown-model", InputTokens: 1000000, OutputTokens: 500000},
		{Model: "claude-haiku-4-5", InputTokens: 100000, OutputTokens: 50000},
	}
	withUnpriced := Saved("claude-sonnet-5-5", buckets, pb)
	onlyPriced := Saved("claude-sonnet-5-5", buckets[1:], pb)

	if math.Abs(withUnpriced.ActualCost-onlyPriced.ActualCost) > 1e-9 {
		t.Errorf("ActualCost = %f, want %f (unpriced bucket must not contribute)", withUnpriced.ActualCost, onlyPriced.ActualCost)
	}
	if math.Abs(withUnpriced.BaselineCost-onlyPriced.BaselineCost) > 1e-9 {
		t.Errorf("BaselineCost = %f, want %f (unpriced tokens must not be priced at baseline)", withUnpriced.BaselineCost, onlyPriced.BaselineCost)
	}
	if withUnpriced.UnpricedBuckets != 1 {
		t.Errorf("UnpricedBuckets = %d, want 1", withUnpriced.UnpricedBuckets)
	}
	if withUnpriced.UnpricedTokens != 1500000 {
		t.Errorf("UnpricedTokens = %d, want 1500000", withUnpriced.UnpricedTokens)
	}
	if withUnpriced.TotalTokens != 1650000 {
		t.Errorf("TotalTokens = %d, want 1650000 (includes unpriced)", withUnpriced.TotalTokens)
	}
}

func TestSaved_AllBucketsUnpriced_SavedPercentNA(t *testing.T) {
	pb := mustPriceBook(t)
	buckets := []TokenBucket{
		{Model: "unknown-a", InputTokens: 1000},
		{Model: "unknown-b", OutputTokens: 500},
	}
	result := Saved("claude-sonnet-5-5", buckets, pb)

	if !result.IsValid() {
		t.Fatal("baseline is priced, result should be valid")
	}
	if result.ActualCost != 0 || result.BaselineCost != 0 || result.Saved != 0 {
		t.Errorf("costs = (%f, %f, %f), want all zero", result.ActualCost, result.BaselineCost, result.Saved)
	}
	if result.UnpricedBuckets != 2 {
		t.Errorf("UnpricedBuckets = %d, want 2", result.UnpricedBuckets)
	}
	if _, ok := result.SavedPercent(); ok {
		t.Error("SavedPercent ok = true, want false (n/a) when no bucket is priced")
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
	r := SavedResult{Quality: SummaryExact, BaselineCost: 0, Saved: 100}
	if _, ok := r.SavedPercent(); ok {
		t.Error("SavedPercent() ok = true, want false (n/a) for zero baseline")
	}
}

func TestSavedResult_SavedPercent_Invalid(t *testing.T) {
	r := SavedResult{Quality: SummaryUnpriced, BaselineCost: 100, Saved: 50}
	if _, ok := r.SavedPercent(); ok {
		t.Error("SavedPercent() ok = true, want false (n/a) for invalid result")
	}
}

func TestSavedResult_SavedPercent_Valid(t *testing.T) {
	r := SavedResult{
		Quality:      SummaryExact,
		BaselineCost: 100,
		Saved:        25,
	}
	got, ok := r.SavedPercent()
	if !ok {
		t.Fatal("SavedPercent() ok = false, want true")
	}
	if math.Abs(got-25) > 0.01 {
		t.Errorf("SavedPercent() = %f, want 25", got)
	}
}

// --- Golden fixtures from JSON ---

type goldenFixtures struct {
	Description string       `json:"description"`
	Version     string       `json:"version"`
	Cases       []goldenCase `json:"cases"`
}

type goldenCase struct {
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	RequestedModel string         `json:"requested_model"`
	Buckets        []goldenBucket `json:"buckets"`
	Expected       goldenExpected `json:"expected"`
}

type goldenBucket struct {
	Model      string `json:"model"`
	Input      int64  `json:"input"`
	Output     int64  `json:"output"`
	CacheRead  int64  `json:"cache_read"`
	CacheWrite int64  `json:"cache_write"`
}

type goldenExpected struct {
	Quality         string  `json:"quality,omitempty"`
	Saved           float64 `json:"saved,omitempty"`
	SavedApprox     float64 `json:"saved_approx,omitempty"`
	SavedPositive   *bool   `json:"saved_positive,omitempty"`
	TotalTokens     int64   `json:"total_tokens,omitempty"`
	Valid           *bool   `json:"valid,omitempty"`
	InferredBuckets int     `json:"inferred_buckets,omitempty"`
	UnpricedBuckets int     `json:"unpriced_buckets,omitempty"`
	UnpricedTokens  int64   `json:"unpriced_tokens,omitempty"`
	// SavedPercentApplicable: pointer so that an explicit false is checked.
	SavedPercentApplicable *bool `json:"saved_percent_applicable,omitempty"`
}

func TestSaved_GoldenFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/golden_cases.json")
	if err != nil {
		t.Fatalf("failed to read golden_cases.json: %v", err)
	}

	var fixtures goldenFixtures
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("failed to parse golden_cases.json: %v", err)
	}

	pb := mustPriceBook(t)

	for _, tc := range fixtures.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			buckets := make([]TokenBucket, len(tc.Buckets))
			for i, b := range tc.Buckets {
				buckets[i] = TokenBucket{
					Model:            b.Model,
					InputTokens:      b.Input,
					OutputTokens:     b.Output,
					CacheReadTokens:  b.CacheRead,
					CacheWriteTokens: b.CacheWrite,
				}
			}

			result := Saved(tc.RequestedModel, buckets, pb)

			// Check quality
			if tc.Expected.Quality != "" {
				wantQuality := SummaryQuality(tc.Expected.Quality)
				if result.Quality != wantQuality {
					t.Errorf("Quality = %q, want %q", result.Quality, wantQuality)
				}
			}

			// Check saved_approx (allows small float diff)
			if tc.Expected.SavedApprox != 0 || tc.Expected.Saved != 0 {
				want := tc.Expected.Saved
				if tc.Expected.SavedApprox != 0 {
					want = tc.Expected.SavedApprox
				}
				if math.Abs(result.Saved-want) > 0.01 {
					t.Errorf("Saved = %f, want ~%f", result.Saved, want)
				}
			}

			// Check saved_positive
			if tc.Expected.SavedPositive != nil {
				gotPositive := result.Saved > 0
				if gotPositive != *tc.Expected.SavedPositive {
					t.Errorf("Saved positive = %v, want %v (saved=%f)", gotPositive, *tc.Expected.SavedPositive, result.Saved)
				}
			}

			// Check total_tokens
			if tc.Expected.TotalTokens != 0 {
				if result.TotalTokens != tc.Expected.TotalTokens {
					t.Errorf("TotalTokens = %d, want %d", result.TotalTokens, tc.Expected.TotalTokens)
				}
			}

			// Check valid
			if tc.Expected.Valid != nil {
				if result.IsValid() != *tc.Expected.Valid {
					t.Errorf("IsValid = %v, want %v", result.IsValid(), *tc.Expected.Valid)
				}
			}

			// Check inferred_buckets
			if tc.Expected.InferredBuckets != 0 {
				if result.InferredBuckets != tc.Expected.InferredBuckets {
					t.Errorf("InferredBuckets = %d, want %d", result.InferredBuckets, tc.Expected.InferredBuckets)
				}
			}

			// Check unpriced_buckets
			if tc.Expected.UnpricedBuckets != 0 {
				if result.UnpricedBuckets != tc.Expected.UnpricedBuckets {
					t.Errorf("UnpricedBuckets = %d, want %d", result.UnpricedBuckets, tc.Expected.UnpricedBuckets)
				}
			}

			// Check unpriced_tokens
			if tc.Expected.UnpricedTokens != 0 {
				if result.UnpricedTokens != tc.Expected.UnpricedTokens {
					t.Errorf("UnpricedTokens = %d, want %d", result.UnpricedTokens, tc.Expected.UnpricedTokens)
				}
			}

			// Check saved_percent_applicable (n/a when false)
			if tc.Expected.SavedPercentApplicable != nil {
				_, ok := result.SavedPercent()
				if ok != *tc.Expected.SavedPercentApplicable {
					t.Errorf("SavedPercent applicable = %v, want %v", ok, *tc.Expected.SavedPercentApplicable)
				}
			}
		})
	}
}
