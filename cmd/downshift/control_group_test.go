// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// Built-in control group through the shared runner: a control spawn is
// classified and logged but never rewritten, and `downshift report` reads it.

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

func runCursorSpawn(t *testing.T, eventLog string) (updated map[string]any, events []telemetry.Event) {
	t.Helper()
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
	var resp struct {
		UpdatedInput map[string]any `json:"updated_input"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("hook stdout is not valid JSON: %v\n%s", err, out)
	}
	events, err := telemetry.ReadEventsFrom(eventLog)
	if err != nil {
		t.Fatal(err)
	}
	return resp.UpdatedInput, events
}

func TestControlGroup_NeverRewritesThroughSharedRunner(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "1")

	updated, events := runCursorSpawn(t, eventLog)
	if updated != nil {
		t.Errorf("control spawn must not be rewritten; got updated_input %v", updated)
	}
	if len(events) != 1 || events[0].Outcome != telemetry.OutcomeBaseline {
		t.Fatalf("want one baseline event, got %+v", events)
	}
	found := false
	for _, c := range events[0].ReasonCodes {
		found = found || c == telemetry.ReasonControl
	}
	if !found {
		t.Errorf("reason codes %v lack %s", events[0].ReasonCodes, telemetry.ReasonControl)
	}
}

func TestControlGroup_ZeroDisablesAndRoutedStillRewrites(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")

	updated, events := runCursorSpawn(t, eventLog)
	if updated == nil {
		t.Fatal("with the control group off the spawn must be rewritten as before")
	}
	if len(events) != 1 || events[0].Outcome == telemetry.OutcomeBaseline {
		t.Errorf("want one routed event, got %+v", events)
	}
}

func TestControlGroup_NoRouteStaysUncoded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "1")
	t.Setenv("DOWNSHIFT_NO_ROUTE", "1")

	_, events := runCursorSpawn(t, eventLog)
	if len(events) != 1 || events[0].Outcome != telemetry.OutcomeBaseline {
		t.Fatalf("want one baseline event, got %+v", events)
	}
	for _, c := range events[0].ReasonCodes {
		if c == telemetry.ReasonControl {
			t.Error("DOWNSHIFT_NO_ROUTE spawns are not the control group and must not carry CONTROL")
		}
	}
}

func TestRunReport_JSONAndFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	var out, errOut bytes.Buffer
	if rc := runReport([]string{"--days=7", "--json"}, &out, &errOut); rc != 0 {
		t.Fatalf("rc = %d, stderr %s", rc, errOut.String())
	}
	var rep telemetry.Report
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("report --json is not JSON: %v\n%s", err, out.String())
	}
	if rep.Enough || !strings.Contains(rep.Message, "not enough data") {
		t.Errorf("empty log must say not enough data: %+v", rep)
	}
	out.Reset()
	if rc := runReport(nil, &out, &errOut); rc != 0 || !strings.Contains(out.String(), "not enough data") {
		t.Errorf("text report rc=%d out=%q", rc, out.String())
	}
	if rc := runReport([]string{"--bogus"}, &out, &errOut); rc != 2 {
		t.Errorf("unknown flag rc = %d, want 2", rc)
	}
	if rc := runReport([]string{"--days=x"}, &out, &errOut); rc != 2 {
		t.Errorf("bad days rc = %d, want 2", rc)
	}
}
