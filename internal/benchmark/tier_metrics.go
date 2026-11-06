// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package benchmark

import "github.com/tiagovilasboas/downshift/internal/core"

// Accuracy returns the fraction of exact complexity-label matches.
func Accuracy(results []Result) float64 {
	if len(results) == 0 {
		return 0
	}
	correct := 0
	for _, r := range results {
		if r.Correct {
			correct++
		}
	}
	return float64(correct) / float64(len(results))
}

// TierAccuracy returns the fraction of tasks where the predicted complexity
// maps to the correct model tier. This is the economically meaningful metric:
// TRIVIAL and SIMPLE share the small tier. MEDIUM is mid. A label miss
// inside the same tier counts as correct routing. SIMPLE predicted as
// MEDIUM does not.
func TierAccuracy(results []Result) float64 {
	if len(results) == 0 {
		return 0
	}
	correct := 0
	for _, r := range results {
		expected, ok := complexityFromString(r.Task.Label)
		if !ok {
			continue
		}
		if core.TierOf(expected) == core.TierOf(r.Predicted) {
			correct++
		}
	}
	return float64(correct) / float64(len(results))
}

// UnsafeDowngradeRates returns the fraction of FRONTIER-tier tasks (COMPLEX)
// mis-routed to MID or SMALL. These are the safety-critical failures.
// Returns (toMID rate, toSMALL rate, count-to-mid, count-to-small, total).
func UnsafeDowngradeRates(results []Result) (toMID, toSMALL float64, cntMID, cntSMALL, total int) {
	for _, r := range results {
		expected, ok := complexityFromString(r.Task.Label)
		if !ok || core.TierOf(expected) != core.TierFrontier {
			continue
		}
		total++
		switch core.TierOf(r.Predicted) {
		case core.TierMid:
			cntMID++
		case core.TierSmall:
			cntSMALL++
		}
	}
	if total == 0 {
		return 0, 0, 0, 0, 0
	}
	return float64(cntMID) / float64(total),
		float64(cntSMALL) / float64(total),
		cntMID, cntSMALL, total
}

// WastefulOverRoutingRates returns the fraction of SMALL-tier tasks (TRIVIAL)
// mis-routed to MID or FRONTIER. Wasteful but safe.
// Returns (toMID rate, toFRONTIER rate, count-to-mid, count-to-frontier, total).
func WastefulOverRoutingRates(results []Result) (toMID, toFrontier float64, cntMID, cntFrontier, total int) {
	for _, r := range results {
		expected, ok := complexityFromString(r.Task.Label)
		if !ok || core.TierOf(expected) != core.TierSmall {
			continue
		}
		total++
		switch core.TierOf(r.Predicted) {
		case core.TierMid:
			cntMID++
		case core.TierFrontier:
			cntFrontier++
		}
	}
	if total == 0 {
		return 0, 0, 0, 0, 0
	}
	return float64(cntMID) / float64(total),
		float64(cntFrontier) / float64(total),
		cntMID, cntFrontier, total
}

// ConfidenceInterval holds the 95% bootstrap confidence bounds [Low, High].
type ConfidenceInterval struct {
	Low  float64 `json:"low"`
	High float64 `json:"high"`
}

// BootstrapConfidenceInterval computes a 95% bootstrap confidence interval
// for TierAccuracy using B resamples.
func BootstrapConfidenceInterval(results []Result, resamples int) ConfidenceInterval {
	n := len(results)
	if n == 0 {
		return ConfidenceInterval{}
	}
	if resamples <= 0 {
		resamples = 1000
	}

	accuracies := make([]float64, resamples)
	// Deterministic LCG pseudo-random generator for reproducibility without external deps
	seed := uint64(133742)
	lcgNext := func() int {
		seed = seed*6364136223846793005 + 1
		return int((seed >> 32) % uint64(n))
	}

	for b := 0; b < resamples; b++ {
		correct := 0
		for i := 0; i < n; i++ {
			idx := lcgNext()
			r := results[idx]
			expected, ok := complexityFromString(r.Task.Label)
			if ok && core.TierOf(expected) == core.TierOf(r.Predicted) {
				correct++
			}
		}
		accuracies[b] = float64(correct) / float64(n)
	}

	// Simple sort for percentiles
	for i := 0; i < len(accuracies)-1; i++ {
		for j := i + 1; j < len(accuracies); j++ {
			if accuracies[j] < accuracies[i] {
				accuracies[i], accuracies[j] = accuracies[j], accuracies[i]
			}
		}
	}

	lowIdx := int(float64(resamples) * 0.025)
	highIdx := int(float64(resamples) * 0.975)
	if highIdx >= resamples {
		highIdx = resamples - 1
	}

	return ConfidenceInterval{
		Low:  accuracies[lowIdx],
		High: accuracies[highIdx],
	}
}
