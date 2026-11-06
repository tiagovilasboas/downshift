// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package fuse

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/classify"
)

func TestApply_DisabledMatchesClassify(t *testing.T) {
	t.Setenv("DOWNSHIFT_MINILM", "0")
	prompt := "rename the userId variable to user_id"
	base := classify.Classify(prompt)
	got := Apply(prompt)
	if got.Complexity != base.Complexity {
		t.Fatalf("without MINILM: classify=%v semantic=%v", base.Complexity, got.Complexity)
	}
}

func TestApply_EnabledDoesNotDowngrade(t *testing.T) {
	t.Setenv("DOWNSHIFT_MINILM", "1")
	prompt := "implement streaming pipeline for processing 1M events/sec"
	base := classify.Classify(prompt)
	got := Apply(prompt)
	if got.Complexity < base.Complexity {
		t.Fatalf("semantic must not downgrade: base=%v sem=%v", base.Complexity, got.Complexity)
	}
}

func TestApply_NoSignalSkipsBoost(t *testing.T) {
	t.Setenv("DOWNSHIFT_MINILM", "1")
	prompt := "https://example.com/some/path?q=1&r=2"
	base := classify.Classify(prompt)
	if !base.NoSignal {
		t.Fatalf("precondition: %q NoSignal=false scores=%v", prompt, base.Scores)
	}
	got := apply(prompt, base)
	if got.Complexity != base.Complexity || got.NoSignal != base.NoSignal || got.SemanticLift {
		t.Fatalf("no-signal prompt changed: base=%+v got=%+v", base, got)
	}
}

func TestApplyBoost_RejectsComplexWithoutRegexScore(t *testing.T) {
	base := classify.Classification{
		Complexity: classify.Simple,
		Scores:     map[classify.Complexity]int{classify.Complex: 0, classify.Simple: 2},
		Confident:  false,
	}
	got := applyBoost("rename the helper", base, func(string, string, bool) (string, bool) {
		return "COMPLEX", true
	})
	if got.Complexity != classify.Simple || got.SemanticLift {
		t.Fatalf("complex without a regex score must stay put: %+v", got)
	}
}

func TestApplyBoost_ReappliesRiskFloor(t *testing.T) {
	base := classify.Classification{
		Complexity: classify.Trivial,
		Scores:     map[classify.Complexity]int{classify.Trivial: 3, classify.Complex: 0},
		Confident:  true,
	}
	got := applyBoost("rename the passwords field", base, func(string, string, bool) (string, bool) {
		return "SIMPLE", true
	})
	if got.Complexity != classify.Medium || !got.RiskFloor || got.Confident {
		t.Fatalf("risk floor after a lift: got %+v", got)
	}
	if !got.SemanticLift {
		t.Fatal("a real raise must set SemanticLift")
	}
}
