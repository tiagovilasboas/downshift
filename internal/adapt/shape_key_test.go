// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package adapt

import "testing"

func TestAdjustTier_NoDownshiftWhenSameTierHasHitAndMiss(t *testing.T) {
	mem := &Memory{Shapes: map[string]ShapeRecord{
		"MEDIUM:coding": {Hits: []tierLabel{"SMALL"}, Miss: []tierLabel{"SMALL"}},
	}}
	got := AdjustTier(TierMid, "MEDIUM:coding", mem)
	if got != TierMid {
		t.Fatalf("got %v want MID (SMALL has both hit and miss)", got)
	}
}

func TestSharedMemory_DifferentClassSameFeatureDoNotCrossContaminate(t *testing.T) {
	// Trivial coding task succeeded at SMALL; complex coding task must not inherit that downshift.
	mem := BuildMemory([]FeedbackEvent{
		{Shape: "TRIVIAL:coding", SelectedTier: TierSmall, Success: true},
	})
	baseline := TierFrontier
	got := AdjustTier(baseline, "COMPLEX:coding", &mem)
	if got != baseline {
		t.Fatalf("COMPLEX:coding adjusted %v want %v (no memory for that key)", got, baseline)
	}
	gotTrivial := AdjustTier(TierMid, "TRIVIAL:coding", &mem)
	if gotTrivial != TierSmall {
		t.Fatalf("TRIVIAL:coding got %v want SMALL", gotTrivial)
	}
}

func TestMemoryKeyForPrompt_IncludesComplexity(t *testing.T) {
	key := MemoryKeyForPrompt("rename the local variable foo to bar in one function", "")
	if key == "" || key == "coding" {
		t.Fatalf("expected class:feature key, got %q", key)
	}
}
