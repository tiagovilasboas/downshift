// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package benchmark_test

import (
	"io"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/benchmark"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// An abstention (unconfident, no signal) is reported apart from a confident
// wrong answer; TierAccuracy itself is unchanged.
func TestAbstentionSplit_SeparatesAbstentionFromError(t *testing.T) {
	tasks := []benchmark.Task{
		{Prompt: "a", Label: "COMPLEX"},
		{Prompt: "b", Label: "COMPLEX"},
		{Prompt: "c", Label: "TRIVIAL"},
	}
	cls := map[string]core.Classification{
		"a": {Complexity: core.Complex, Confident: true},
		"b": {Complexity: core.Medium, NoSignal: true},
		"c": {Complexity: core.Complex, Confident: true},
	}
	results := benchmark.RunWithClassifier(tasks, func(p string) core.Classification { return cls[p] }, io.Discard)
	sp := benchmark.AbstentionSplit(results)
	if sp.ConfidentHit != 1 || sp.ConfidentMiss != 1 || sp.Abstained != 1 || sp.NoSignal != 1 || sp.AbstainedHit != 0 {
		t.Fatalf("split = %+v", sp)
	}
	if acc := benchmark.TierAccuracy(results); acc < 0.333 || acc > 0.334 {
		t.Fatalf("TierAccuracy changed: %v", acc)
	}
}
