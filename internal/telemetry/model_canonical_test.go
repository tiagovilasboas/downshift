// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/telemetry/catalogtier"
)

func TestCanonicalizeModels(t *testing.T) {
	prev := DirectionCatalog
	DirectionCatalog = catalogtier.Of(catalog.Load())
	t.Cleanup(func() { DirectionCatalog = prev })

	cases := []struct {
		name                   string
		harness, from, to, raw string
		wantFrom, wantTo       string
		wantFromRaw, wantToRaw string
	}{
		{name: "claude code native names", harness: "claude-code", from: "sonnet", to: "haiku",
			wantFrom: "claude-sonnet-5-5", wantTo: "claude-haiku-4-5", wantFromRaw: "sonnet", wantToRaw: "haiku"},
		{name: "catalog ids unchanged", harness: "claude-code", from: "claude-sonnet-5-5", to: "claude-opus-5-5",
			wantFrom: "claude-sonnet-5-5", wantTo: "claude-opus-5-5"},
		{name: "kirocrew dotted aliases", harness: "kirocrew", from: "claude-opus-4.8", to: "claude-haiku-4.5",
			wantFrom: "claude-opus-4-8", wantTo: "claude-haiku-4-5", wantFromRaw: "claude-opus-4.8", wantToRaw: "claude-haiku-4.5"},
		{name: "family-only match kept", harness: "claude-code", from: "claude-opus-9-9", to: "claude-opus-9-9",
			wantFrom: "claude-opus-9-9", wantTo: "claude-opus-9-9"},
		{name: "unknown kept with its raw", harness: "claude-code", from: "unknown", to: "unknown", raw: "auto",
			wantFrom: "unknown", wantTo: "unknown", wantFromRaw: "auto"},
		{name: "gateway slug kept", harness: "claude-code", from: "anthropic/claude-opus-5-5", to: "anthropic/claude-opus-5-5",
			wantFrom: "anthropic/claude-opus-5-5", wantTo: "anthropic/claude-opus-5-5"},
		{name: "no harness", from: "sonnet", to: "sonnet", wantFrom: "sonnet", wantTo: "sonnet"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := Event{Harness: tc.harness, FromModel: tc.from, ToModel: tc.to, RequestedModelRaw: tc.raw}
			canonicalizeModels(&ev)
			if ev.FromModel != tc.wantFrom || ev.ToModel != tc.wantTo {
				t.Errorf("models = %q -> %q, want %q -> %q", ev.FromModel, ev.ToModel, tc.wantFrom, tc.wantTo)
			}
			if ev.RequestedModelRaw != tc.wantFromRaw || ev.FinalModelRaw != tc.wantToRaw {
				t.Errorf("raw = %q / %q, want %q / %q", ev.RequestedModelRaw, ev.FinalModelRaw, tc.wantFromRaw, tc.wantToRaw)
			}
		})
	}
}

func TestCanonicalizeModels_NoCatalogKeepsValues(t *testing.T) {
	prev := DirectionCatalog
	DirectionCatalog = nil
	t.Cleanup(func() { DirectionCatalog = prev })

	ev := Event{Harness: "claude-code", FromModel: "haiku", ToModel: "haiku"}
	canonicalizeModels(&ev)
	if ev.FromModel != "haiku" || ev.ToModel != "haiku" || ev.RequestedModelRaw != "" || ev.FinalModelRaw != "" {
		t.Errorf("event changed without a catalog: %+v", ev)
	}
}

// Record is the one place every hook and child-end record passes through, so
// the canonical id reaches the log whichever path wrote the event.
func TestRecord_WritesCatalogIDs(t *testing.T) {
	prev := DirectionCatalog
	DirectionCatalog = catalogtier.Of(catalog.Load())
	t.Cleanup(func() { DirectionCatalog = prev })
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)

	Record(Event{CorrelationID: "c", Harness: "claude-code", Outcome: OutcomeAllow, FromModel: "sonnet", ToModel: "sonnet"})
	events, err := ReadEventsFrom(log)
	if err != nil || len(events) != 1 {
		t.Fatalf("ReadEventsFrom = %d events, %v", len(events), err)
	}
	got := events[0]
	if got.FromModel != "claude-sonnet-5-5" || got.ToModel != "claude-sonnet-5-5" || got.RequestedModelRaw != "sonnet" || got.FinalModelRaw != "sonnet" {
		t.Errorf("recorded %q/%q raw %q/%q", got.FromModel, got.ToModel, got.RequestedModelRaw, got.FinalModelRaw)
	}
}
