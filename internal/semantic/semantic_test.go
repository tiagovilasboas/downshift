// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

package semantic_test

import (
	"os"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/semantic"
)

func TestMaybeAugment_DisabledByDefault(t *testing.T) {
	out, ok := semantic.MaybeAugment("rename userId", semantic.LabelTrivial, true)
	if ok || out != semantic.LabelTrivial {
		t.Fatalf("expected no change, got %q ok=%v", out, ok)
	}
}

func TestMaybeAugment_MonotonicWhenEnabled(t *testing.T) {
	t.Setenv("DOWNSHIFT_MINILM", "1")
	t.Setenv("DOWNSHIFT_MINILM_EMBED", "hash")

	out, ok := semantic.MaybeAugment("rearchitect the payment monolith for multi-tenant compliance", semantic.LabelTrivial, false)
	if !ok {
		t.Skip("hash prototypes did not escalate this prompt")
	}
	if semantic.LabelRank(out) < semantic.LabelRank(semantic.LabelTrivial) {
		t.Fatalf("semantic downgraded to %q", out)
	}
}

func TestHashEmbed_UnitNorm(t *testing.T) {
	v := semantic.HashEmbed("hello world", 32)
	var sum float64
	for _, x := range v {
		sum += x * x
	}
	if sum < 0.99 || sum > 1.01 {
		t.Fatalf("expected unit norm, got %f", sum)
	}
}

func TestRefreshHashPrototypesFromBenchmark(t *testing.T) {
	if os.Getenv("REFRESH_PROTOTYPES") != "1" {
		t.Skip("set REFRESH_PROTOTYPES=1 to rewrite prototypes.json")
	}
	err := semantic.RefreshHashPrototypesFromBenchmark("../../benchmark/tasks.json", "data/prototypes.json")
	if err != nil {
		t.Fatal(err)
	}
}
