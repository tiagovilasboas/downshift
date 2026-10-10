// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package domain

import "testing"

func TestDominantFeature(t *testing.T) {
	if got := DominantFeature(FeatureVector{}); got != "" {
		t.Fatalf("zero vector = %q, want empty", got)
	}
	if got := DominantFeature(FeatureVector{Coding: 0.2, Security: 0.9}); got != "security" {
		t.Fatalf("got %q want security", got)
	}
}

func TestMemoryShapeKey(t *testing.T) {
	if got := MemoryShapeKey("TRIVIAL", "coding"); got != "TRIVIAL:coding" {
		t.Fatalf("got %q", got)
	}
	if MemoryShapeKey("", "coding") != "" {
		t.Fatal("empty class")
	}
	if MemoryShapeKey("TRIVIAL", "") != "" {
		t.Fatal("empty feature")
	}
}
