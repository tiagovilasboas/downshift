// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package operatorexport

import (
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
	"github.com/tiagovilasboas/downshift-ai/internal/catalog"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

var buildNow = time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)

// TestBuildOutputPassesCheck is the tie between the builder and the sensor: the
// real export Build produces must satisfy the allowlist. If Build ever adds a
// string field, this fails until the allowlist decides to allow it.
func TestBuildOutputPassesCheck(t *testing.T) {
	events := []telemetry.Event{
		decisionEvent("claude-code", "DOWNSHIFT"),
		decisionEvent("codex", "OK"),
	}
	e := Build(events, nil, 30, buildNow)

	if err := Check(e); err != nil {
		t.Fatalf("Build output failed the allowlist: %v", err)
	}
	if e.Schema != SchemaVersion {
		t.Errorf("schema = %d, want %d", e.Schema, SchemaVersion)
	}
	if e.WindowDays != 30 {
		t.Errorf("window_days = %d, want 30", e.WindowDays)
	}
	if e.Note != Note {
		t.Errorf("note = %q, want the fixed note", e.Note)
	}
	if !strings.HasPrefix(e.GeneratedAt, "2026-11-01T12:00:00") {
		t.Errorf("generated_at = %q, want the injected clock", e.GeneratedAt)
	}
}

// TestBuildEmptyIsClean proves an empty log still produces a valid, non-leaky
// export (an operator who ran nothing can still paste it safely).
func TestBuildEmptyIsClean(t *testing.T) {
	e := Build(nil, nil, 7, buildNow)
	if err := Check(e); err != nil {
		t.Fatalf("empty export failed the allowlist: %v", err)
	}
	if e.Routed.Spawns != 0 {
		t.Errorf("empty log should have no spawns, got routed=%d", e.Routed.Spawns)
	}
}

// TestBuildHarnessRowsSorted checks the per-harness rows come out in a stable
// order (the JSON an operator pastes must be deterministic).
func TestBuildHarnessRowsSorted(t *testing.T) {
	events := []telemetry.Event{
		decisionEvent("codex", "OK"),
		decisionEvent("claude-code", "DOWNSHIFT"),
	}
	e := Build(events, nil, 30, buildNow)
	for i := 1; i < len(e.Harnesses); i++ {
		if e.Harnesses[i-1].Harness > e.Harnesses[i].Harness {
			t.Fatalf("harness rows not sorted: %q before %q", e.Harnesses[i-1].Harness, e.Harnesses[i].Harness)
		}
	}
}

// decisionEvent is a minimal routing decision at the fixed clock.
func decisionEvent(harness, verdict string) telemetry.Event {
	return telemetry.Event{
		Timestamp:  buildNow.Format(time.RFC3339Nano),
		Harness:    harness,
		Complexity: "TRIVIAL",
		FromModel:  "model-frontier",
		ToModel:    "model-small",
		Verdict:    verdict,
	}
}

// TestBuildFoldsUnknownHarnessNames: a harness name outside the adapter table
// (an old log from a renamed adapter) must not block the export; it is folded,
// with the empty name, into one unknown row that keeps the totals.
func TestBuildFoldsUnknownHarnessNames(t *testing.T) {
	execEvents := []execwrap.Event{
		{TS: buildNow.Format(time.RFC3339), Harness: "legacy-adapter", RawBytes: 4000, EmittedBytes: 0},
		{TS: buildNow.Format(time.RFC3339), Harness: "my-secret-internal-tool", RawBytes: 4000, EmittedBytes: 0},
		{TS: buildNow.Format(time.RFC3339), RawBytes: 4000, EmittedBytes: 0},
	}
	e := Build(nil, execEvents, 0, buildNow)
	if err := Check(e); err != nil {
		t.Fatalf("export with legacy harness names failed the allowlist: %v", err)
	}
	if len(e.Harnesses) != 1 || e.Harnesses[0].Harness != accounting.UnknownHarness {
		t.Fatalf("harness rows = %+v, want one %q row", e.Harnesses, accounting.UnknownHarness)
	}
	want := 3 * 4000 / execwrap.BytesPerToken
	if got := e.Harnesses[0].CompactorAvoided; got != int64(want) {
		t.Errorf("compactor_avoided = %d, want %d (the folded rows summed)", got, want)
	}
}

// TestBuildRoutingRowsStayPerHarness: the routing figures are one row per
// harness, and the export has no cross-harness routing total.
func TestBuildRoutingRowsStayPerHarness(t *testing.T) {
	prev := telemetry.DirectionCatalog
	telemetry.DirectionCatalog = catalog.TierViewOf(catalog.Load())
	t.Cleanup(func() { telemetry.DirectionCatalog = prev })
	in, out := int64(1000), int64(500)
	spawn := func(harness, id string) []telemetry.Event {
		ts := buildNow.Add(-time.Hour).Format(time.RFC3339Nano)
		return []telemetry.Event{
			{Timestamp: ts, CorrelationID: id, Harness: harness, Outcome: telemetry.OutcomeAllow, FromModel: "claude-opus-4-8", ToModel: "claude-sonnet-4-6", ParentModel: "claude-opus-4-8"},
			{Timestamp: ts, CorrelationID: id + "u", Harness: harness, Outcome: telemetry.OutcomeUsage, LinkedDecision: id, FromModel: "claude-opus-4-8", ToModel: "claude-sonnet-4-6", InputTokens: &in, OutputTokens: &out},
		}
	}
	events := append(spawn("claude-code", "a"), spawn("cursor", "b")...)
	e := Build(events, nil, 30, buildNow)
	if err := Check(e); err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, h := range e.Harnesses {
		got[h.Harness] = h.BelowSessionBillable
	}
	if got["claude-code"] != 1500 || got["cursor"] != 0 {
		t.Fatalf("below-session by harness = %v; each harness counts only its own spawns (cursor ranks no model here)", got)
	}
}
