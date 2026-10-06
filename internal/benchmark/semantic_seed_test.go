// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package benchmark_test

import (
	"io"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/benchmark"
	"github.com/tiagovilasboas/downshift/internal/core"
)

func TestSeedDataset_MiniLM_ImprovesTierOrSafety(t *testing.T) {
	tasks, err := benchmark.LoadDataset("../../benchmark/tasks.json")
	if err != nil {
		t.Skip("seed dataset not found")
	}

	regexOnly := benchmark.RunWithClassifier(tasks, core.Classify, io.Discard)

	t.Setenv("DOWNSHIFT_MINILM", "1")
	t.Setenv("DOWNSHIFT_MINILM_EMBED", "hash")
	withMiniLM := benchmark.Run(tasks, io.Discard)

	regexTier := benchmark.TierAccuracy(regexOnly)
	enabledTier := benchmark.TierAccuracy(withMiniLM)
	regexMID, _, _, _, _ := benchmark.UnsafeDowngradeRates(regexOnly)
	enabledMID, _, _, _, _ := benchmark.UnsafeDowngradeRates(withMiniLM)

	t.Logf("tier accuracy: regex=%.1f%% MINILM=1=%.1f%%", regexTier*100, enabledTier*100)
	t.Logf("FRONTIER→MID: regex=%.1f%% MINILM=1=%.1f%%", regexMID*100, enabledMID*100)

	if enabledTier+1e-9 < regexTier {
		t.Errorf("MINILM tier accuracy %.3f < regex-only %.3f", enabledTier, regexTier)
	}
	if enabledMID > regexMID+1e-9 {
		t.Errorf("MINILM FRONTIER→MID %.3f worse than regex-only %.3f", enabledMID, regexMID)
	}
}
