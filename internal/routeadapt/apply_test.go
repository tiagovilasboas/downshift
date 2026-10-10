// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package routeadapt

import (
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapt"
	"github.com/tiagovilasboas/downshift/internal/core"
)

func TestApply_ShadowDoesNotChangeTier(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	prompt := "implement the function handler service module"
	key := adapt.MemoryKeyForPrompt(prompt, "")
	if key == "" {
		t.Fatal("expected a memory key")
	}
	mem := adapt.BuildMemory([]adapt.FeedbackEvent{
		{Shape: key, SelectedTier: adapt.TierSmall, Success: true},
	})
	if err := adapt.Save(filepath.Join(dir, "adapt-memory.json"), mem); err != nil {
		t.Fatal(err)
	}
	base := core.ClassifyWithSemantic(prompt).Complexity.Tier()
	if base == core.TierSmall {
		t.Fatalf("prompt classified small, cannot show a held downshift")
	}
	d := core.Route(prompt, "claude-code", "")
	if d.Tier != base {
		t.Fatalf("routed tier = %s, classifier = %s; shadow must not apply", d.Tier, base)
	}
	if !d.AdaptWouldSet || d.AdaptWould != core.TierSmall {
		t.Fatalf("shadow = set:%v tier:%s, want small", d.AdaptWouldSet, d.AdaptWould)
	}
}
