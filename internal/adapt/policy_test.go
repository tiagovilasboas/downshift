// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package adapt

import "testing"

func TestAdjustTier_CheapestHitBelowCurrent(t *testing.T) {
	mem := &Memory{Shapes: map[string]ShapeRecord{
		"coding": {Hits: []tierLabel{"SMALL"}, Miss: []tierLabel{"MID"}},
	}}
	got := AdjustTier(TierMid, "coding", mem)
	if got != TierSmall {
		t.Fatalf("got %v want SMALL", got)
	}
}

func TestAdjustTier_CurrentHitDoesNotUpshift(t *testing.T) {
	mem := &Memory{Shapes: map[string]ShapeRecord{
		"security": {Hits: []tierLabel{"MID"}, Miss: []tierLabel{"SMALL"}},
	}}
	got := AdjustTier(TierMid, "security", mem)
	if got != TierMid {
		t.Fatalf("got %v want MID", got)
	}
}

func TestAdjustTier_FailureUsesHigherHit(t *testing.T) {
	mem := &Memory{Shapes: map[string]ShapeRecord{
		"debugging": {Hits: []tierLabel{"MID"}, Miss: []tierLabel{"SMALL"}},
	}}
	got := AdjustTier(TierSmall, "debugging", mem)
	if got != TierMid {
		t.Fatalf("got %v want MID", got)
	}
}

func TestAdjustTier_NoHigherHitStaysPut(t *testing.T) {
	mem := &Memory{Shapes: map[string]ShapeRecord{
		"debugging": {Miss: []tierLabel{"SMALL"}},
	}}
	got := AdjustTier(TierSmall, "debugging", mem)
	if got != TierSmall {
		t.Fatalf("got %v want SMALL", got)
	}
}

func TestAdjustTier_FrozenShapeUsesCheapestHit(t *testing.T) {
	mem := &Memory{Shapes: map[string]ShapeRecord{
		"coding": {Hits: []tierLabel{"SMALL"}},
	}}
	got := AdjustTier(TierMid, "coding", mem)
	if got != TierSmall {
		t.Fatalf("got %v want SMALL (hit-only shape still downshifts)", got)
	}
}

func TestAdjustTier_EmptyShapeNoOp(t *testing.T) {
	mem := &Memory{Shapes: map[string]ShapeRecord{
		"coding": {Hits: []tierLabel{"SMALL"}},
	}}
	got := AdjustTier(TierMid, "", mem)
	if got != TierMid {
		t.Fatalf("got %v want MID", got)
	}
}

func TestBuildMemory_SuccessAndFailure(t *testing.T) {
	mem := BuildMemory([]FeedbackEvent{
		{Shape: "coding", SelectedTier: TierMid, Success: true},
		{Shape: "coding", SelectedTier: TierSmall, Failed: true},
	})
	rec := mem.Shapes["coding"]
	if len(rec.Hits) != 1 || rec.Hits[0] != "MID" {
		t.Fatalf("hits = %#v", rec.Hits)
	}
	if len(rec.Miss) != 1 || rec.Miss[0] != "SMALL" {
		t.Fatalf("misses = %#v", rec.Miss)
	}
}

func TestLoad_MissingFileFailOpen(t *testing.T) {
	mem, err := Load(t.TempDir() + "/missing.json")
	if err != nil {
		t.Fatal(err)
	}
	if mem != nil {
		t.Fatalf("want nil memory, got %#v", mem)
	}
}

func TestShapeFromValues(t *testing.T) {
	var zero [13]float64
	if ShapeFromValues(zero) != "" {
		t.Fatal("zero vector should not produce a shape")
	}
	var v [13]float64
	v[1] = 0.5
	if ShapeFromValues(v) != "coding" {
		t.Fatalf("got %q", ShapeFromValues(v))
	}
}
