// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package benchmark_test

import (
	"io"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/benchmark"
	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

// Tier-labelled datasets (small/mid/frontier) are scored on tiers instead
// of being skipped as unknown labels.
func TestRun_AcceptsTierLabels(t *testing.T) {
	tasks := []benchmark.Task{
		{Prompt: "a", Label: "small"},
		{Prompt: "b", Label: "mid"},
		{Prompt: "c", Label: "frontier"},
		{Prompt: "d", Label: "frontier"},
	}
	predict := map[string]core.Complexity{"a": core.Trivial, "b": core.Medium, "c": core.Complex, "d": core.Medium}
	results := benchmark.RunWithClassifier(tasks, func(p string) core.Classification {
		return core.Classification{Complexity: predict[p]}
	}, io.Discard)
	if len(results) != 4 {
		t.Fatalf("results = %d, want 4 (tier labels must not be skipped)", len(results))
	}
	rep := benchmark.GenerateReport(results)
	if rep.TierAccuracy != 0.75 || rep.ComplexityAccuracy != 0.75 {
		t.Fatalf("tier=%.2f complexity=%.2f, want 0.75/0.75", rep.TierAccuracy, rep.ComplexityAccuracy)
	}
	if rep.FrontierTotal != 2 || rep.FrontierToMidRate != 0.5 || rep.SmallTotal != 1 {
		t.Fatalf("frontier_total=%d f->mid=%.2f small_total=%d", rep.FrontierTotal, rep.FrontierToMidRate, rep.SmallTotal)
	}
}
