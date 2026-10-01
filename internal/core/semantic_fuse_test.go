// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

func TestClassifyWithSemantic_DisabledMatchesClassify(t *testing.T) {
	t.Setenv("DOWNSHIFT_MINILM", "0")
	prompt := "rename the userId variable to user_id"
	base := core.Classify(prompt)
	sem := core.ClassifyWithSemantic(prompt)
	if sem.Complexity != base.Complexity {
		t.Fatalf("without MINILM: classify=%v semantic=%v", base.Complexity, sem.Complexity)
	}
}

func TestClassifyWithSemantic_EnabledCanRaise(t *testing.T) {
	t.Setenv("DOWNSHIFT_MINILM", "1")
	t.Setenv("DOWNSHIFT_MINILM_EMBED", "hash")

	prompt := "implement streaming pipeline for processing 1M events/sec"
	base := core.Classify(prompt)
	sem := core.ClassifyWithSemantic(prompt)
	if sem.Complexity.Tier() < base.Complexity.Tier() {
		t.Fatalf("semantic must not downgrade tier: base=%v sem=%v", base.Complexity, sem.Complexity)
	}
	if sem.Complexity.Tier() > base.Complexity.Tier() {
		t.Logf("semantic raised tier: %v -> %v", base.Complexity, sem.Complexity)
	}
}
