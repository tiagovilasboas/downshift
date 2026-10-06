// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// TestHookE2E_RewriteEventStats is the hook-layer end-to-end proof: realistic
// Cursor preToolUse JSON on stdin drives the real adapter route path (no mocks
// of core policy), emits a model rewrite on stdout, appends one durable JSONL
// event, and telemetry.Aggregate reads that event back as 1 DOWNSHIFT with
// positive normalised savings.
//
// Scope note: this is hook-layer E2E, not a real subagent spawn. A real spawn
// is impossible in CI (it needs provider keys and a live harness), so the
// honest E2E boundary is stdin -> rewrite -> event log -> stats.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestHookE2E_RewriteEventStats(t *testing.T) {
	// Keep the test hermetic: isolate HOME (training loop log root) and point
	// the telemetry event log at a temp file.
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)

	// Cursor has the simplest hook contract (flat task/model fields), so it is
	// the adapter under test here.
	smallID := cmdCat.ModelFor("cursor", core.TierSmall).ID
	midID := cmdCat.ModelFor("cursor", core.TierMid).ID
	frontierID := cmdCat.ModelFor("cursor", core.TierFrontier).ID
	if smallID == "" || frontierID == "" {
		t.Fatalf("cursor catalog missing tier models (small=%q frontier=%q)", smallID, frontierID)
	}
	setSessionAllowlist(t, "cursor", []string{smallID, midID, frontierID})

	// A trivial task (rename a variable) arriving on a frontier model must
	// downshift to the small tier.
	task := "rename the userId variable to userIdentifier"
	stdin := []byte(`{"hook_event_name":"preToolUse","tool_name":"Task","model_id":"` +
		frontierID + `","tool_input":{"task":"` + task + `","model":"` + frontierID + `"}}`)

	out := captureStdout(func() {
		rc := runHookAdapter(
			bytes.NewReader(stdin),
			func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
			func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, cmdCat) },
			printCursorAllow,
		)
		if rc != 0 {
			t.Errorf("runHookAdapter() rc = %d, want 0", rc)
		}
	})

	// The hook must allow the spawn with a rewrite to the small-tier model.
	if !strings.Contains(out, `"permission":"allow"`) {
		t.Fatalf("expected cursor allow envelope; got:\n%s", out)
	}
	var hookOut struct {
		UpdatedInput map[string]any `json:"updated_input"`
	}
	if err := json.Unmarshal([]byte(out), &hookOut); err != nil {
		t.Fatalf("hook stdout is not valid JSON: %v\n%s", err, out)
	}
	if hookOut.UpdatedInput == nil {
		t.Fatalf("expected updated_input rewrite; got:\n%s", out)
	}
	if got := hookOut.UpdatedInput["model"]; got != smallID {
		t.Fatalf("rewritten model = %v, want small-tier %s", got, smallID)
	}

	// Exactly one durable JSONL line must have been appended.
	raw, err := os.ReadFile(eventLog)
	if err != nil {
		t.Fatalf("reading event log: %v", err)
	}
	if lines := bytes.Count(bytes.TrimSpace(raw), []byte("\n")) + 1; len(bytes.TrimSpace(raw)) == 0 || lines != 1 {
		t.Fatalf("expected 1 JSONL line, got %d in:\n%s", lines, raw)
	}
	if strings.Contains(string(raw), task) {
		t.Fatal("event log must never persist task text")
	}

	// The event must round-trip with DOWNSHIFT provenance.
	events, err := telemetry.ReadEventsFrom(eventLog)
	if err != nil {
		t.Fatalf("telemetry.ReadEventsFrom: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	ev := events[0]
	if ev.Verdict != "DOWNSHIFT" {
		t.Errorf("verdict = %q, want DOWNSHIFT", ev.Verdict)
	}
	if ev.ToModel != smallID {
		t.Errorf("final_model = %q, want %s", ev.ToModel, smallID)
	}
	if ev.FromModel != frontierID {
		t.Errorf("requested_model = %q, want %s", ev.FromModel, frontierID)
	}

	// Stats aggregation over the file must count the downshift with positive
	// normalised savings (saved units and fraction both make sense: > 0).
	stats := telemetry.Aggregate(events)
	if stats.Total != 1 || stats.Downshifted != 1 {
		t.Fatalf("stats = %+v, want Total=1 Downshifted=1", stats)
	}
	saved, frac := stats.NormSaved()
	if saved <= 0 || frac <= 0 || frac > 1 {
		t.Fatalf("NormSaved() = (%v, %v), want positive units and a fraction in (0,1]", saved, frac)
	}
}
