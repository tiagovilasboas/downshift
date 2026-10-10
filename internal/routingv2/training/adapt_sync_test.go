// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package training

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/adapt"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routingv2/domain"
)

func TestRefreshAdaptMemoryAfterOutcome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	path := filepath.Join(dir, "loop-events.jsonl")
	store := NewEventStore(path)
	event := Event{
		ID:           "r1",
		Timestamp:    time.Now().UTC(),
		Features:     domain.FeatureVector{Coding: 0.8},
		SelectedTier: core.TierMid,
		Harness:      "codex",
	}
	if err := store.Record(event); err != nil {
		t.Fatal(err)
	}
	if err := store.AddOutcome(event.ID, Outcome{Success: true}); err != nil {
		t.Fatal(err)
	}
	memPath := filepath.Join(dir, "adapt-memory.json")
	if _, err := os.Stat(memPath); err != nil {
		t.Fatalf("adapt memory not written: %v", err)
	}
	mem, err := adapt.Load(memPath)
	if err != nil || mem == nil {
		t.Fatalf("load adapt memory: %v %#v", err, mem)
	}
	if _, ok := mem.Shapes["coding"]; !ok {
		t.Fatalf("shapes = %#v", mem.Shapes)
	}
}
