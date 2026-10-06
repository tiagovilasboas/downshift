// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// Control-group coverage: DOWNSHIFT_NO_ROUTE=1 classifies but never rewrites,
// recording outcome "baseline" events that stats excludes from routed rates.
// Plus stats --export producing pasteable JSON for multi-user evidence.

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestHookNoRoute_AllowsWithoutRewrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	t.Setenv("DOWNSHIFT_NO_ROUTE", "1")

	smallID := cmdCat.ModelFor("cursor", core.TierSmall).ID
	midID := cmdCat.ModelFor("cursor", core.TierMid).ID
	frontierID := cmdCat.ModelFor("cursor", core.TierFrontier).ID
	setSessionAllowlist(t, "cursor", []string{smallID, midID, frontierID})

	stdin := []byte(`{"hook_event_name":"preToolUse","tool_name":"Task","model_id":"` +
		frontierID + `","tool_input":{"task":"rename the userId variable","model":"` + frontierID + `"}}`)

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

	if !strings.Contains(out, `"permission":"allow"`) {
		t.Fatalf("expected allow envelope; got:\n%s", out)
	}
	var hookOut struct {
		UpdatedInput map[string]any `json:"updated_input"`
	}
	if err := json.Unmarshal([]byte(out), &hookOut); err != nil {
		t.Fatalf("hook stdout is not valid JSON: %v\n%s", err, out)
	}
	if hookOut.UpdatedInput != nil {
		t.Errorf("no-route must not rewrite; got updated_input %v", hookOut.UpdatedInput)
	}

	events, err := telemetry.ReadEventsFrom(eventLog)
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %d, err = %v; want 1 baseline event", len(events), err)
	}
	if events[0].Outcome != telemetry.OutcomeBaseline {
		t.Errorf("Outcome = %q, want baseline", events[0].Outcome)
	}
	s := telemetry.Aggregate(events)
	if s.Total != 0 || s.Baseline != 1 {
		t.Errorf("Total/Baseline = %d/%d, want 0/1", s.Total, s.Baseline)
	}
}

func TestStatsExport_PasteableJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)

	smallID := cmdCat.ModelFor("cursor", core.TierSmall).ID
	midID := cmdCat.ModelFor("cursor", core.TierMid).ID
	frontierID := cmdCat.ModelFor("cursor", core.TierFrontier).ID
	setSessionAllowlist(t, "cursor", []string{smallID, midID, frontierID})
	stdin := []byte(`{"hook_event_name":"preToolUse","tool_name":"Task","model_id":"` +
		frontierID + `","tool_input":{"task":"rename the userId variable","model":"` + frontierID + `"}}`)
	captureStdout(func() {
		runHookAdapter(
			bytes.NewReader(stdin),
			func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
			func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, cmdCat) },
			printCursorAllow,
		)
	})

	out := captureStdout(func() {
		if rc := runStats([]string{"--days=0", "--export"}); rc != 0 {
			t.Errorf("runStats --export rc = %d, want 0", rc)
		}
	})
	var summary struct {
		Total         int     `json:"total"`
		DownshiftRate float64 `json:"downshift_rate"`
		Note          string  `json:"note"`
	}
	if err := json.Unmarshal([]byte(out), &summary); err != nil {
		t.Fatalf("export is not valid JSON: %v\n%s", err, out)
	}
	if summary.Total != 1 || summary.Note == "" {
		t.Errorf("summary = %+v, want total 1 with honesty note", summary)
	}
}
