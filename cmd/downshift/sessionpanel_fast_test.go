// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// appendPanelFixture writes one routed decision and one recent exec run into
// the state dir the test isolates, so both the tail and the report have data.
func appendPanelFixture(t *testing.T) {
	t.Helper()
	ev := telemetry.Event{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Harness: "claude-code", SessionID: telemetry.HashSessionID("s-1"),
		Agent: "worker", Complexity: "SIMPLE", FromModel: "claude-opus-4-7", ToModel: "claude-haiku-4-5",
		Verdict: "DOWNSHIFT", Outcome: "rewrite_emitted", Tier: "SMALL", CorrelationID: "corr-fast", ToolUseHash: telemetry.ToolUseKey("toolu_fast"),
	}
	// endEnv points DOWNSHIFT_EVENT_LOG at the log the panel reads.
	if err := telemetry.AppendTo(os.Getenv("DOWNSHIFT_EVENT_LOG"), ev); err != nil {
		t.Fatal(err)
	}
	execPath, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	recent := execwrap.Event{TS: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339), RawBytes: 400, EmittedBytes: 100, Compacted: true}
	if err := execwrap.Append(execPath, recent); err != nil {
		t.Fatal(err)
	}
}

// panelJSON runs `session --panel` with args and returns the object's keys.
func panelJSON(t *testing.T, args ...string) map[string]json.RawMessage {
	t.Helper()
	var out, errOut bytes.Buffer
	if rc := runSession(append([]string{"--panel"}, args...), &out, &errOut, nil); rc != 0 {
		t.Fatalf("session --panel %v rc=%d %s", args, rc, errOut.String())
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &obj); err != nil {
		t.Fatalf("session --panel %v: %v\n%s", args, err, out.String())
	}
	return obj
}

func keysOf(obj map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --no-report: the session view only. It has no report, compactor or
// tokens_saved_estimate, and its keys are exactly the PanelView's.
func TestPanelNoReport_SessionViewOnly(t *testing.T) {
	panelFixtureEnv(t)
	appendPanelFixture(t)

	noReport := panelJSON(t, "--no-report")
	for _, key := range []string{"report", "compactor", "tokens_saved_estimate", "tokens_saved_short"} {
		if _, ok := noReport[key]; ok {
			t.Errorf("--no-report carries %q: %v", key, keysOf(noReport))
		}
	}
	full := panelJSON(t)
	if _, ok := full["report"]; !ok {
		t.Fatalf("full panel lacks the report block: %v", keysOf(full))
	}
	for _, key := range []string{"session", "summary", "switched", "held", "tokens", "spawns"} {
		if string(noReport[key]) != string(full[key]) {
			t.Errorf("--no-report %q = %s, full = %s", key, noReport[key], full[key])
		}
	}
}

// --brief: the band's fields only. The counts, summary and spawn rows equal
// the full panel's; the token text is gone; with the report the figure is there.
func TestPanelBrief_FieldsMatchFullPanel(t *testing.T) {
	panelFixtureEnv(t)
	appendPanelFixture(t)

	full := panelJSON(t)
	brief := panelJSON(t, "--brief")
	for _, key := range []string{"tokens", "tokens_line", "by_agent_type", "report", "compactor"} {
		if _, ok := brief[key]; ok {
			t.Errorf("--brief carries %q: %v", key, keysOf(brief))
		}
	}
	for _, key := range []string{"session", "summary", "switched", "model_switched", "effort_changed", "held", "held_guardrail", "upshift_not_applied", "observed"} {
		if string(brief[key]) != string(full[key]) {
			t.Errorf("--brief %q = %s, full = %s", key, brief[key], full[key])
		}
	}
	var spawns []map[string]json.RawMessage
	if err := json.Unmarshal(brief["spawns"], &spawns); err != nil || len(spawns) != 1 {
		t.Fatalf("brief spawns = %s (%v), want one row", brief["spawns"], err)
	}
	if _, ok := spawns[0]["tokens"]; ok {
		t.Errorf("--brief spawn carries tokens: %s", brief["spawns"])
	}
	var fullSpawns []map[string]json.RawMessage
	if err := json.Unmarshal(full["spawns"], &fullSpawns); err != nil || len(fullSpawns) != 1 {
		t.Fatalf("full spawns = %s (%v), want one row", full["spawns"], err)
	}
	for _, key := range []string{"tool_use", "state", "from_model", "to_model", "label", "saved_total", "saved_short"} {
		if string(spawns[0][key]) != string(fullSpawns[0][key]) {
			t.Errorf("--brief spawn %q = %s, full = %s", key, spawns[0][key], fullSpawns[0][key])
		}
	}
	for _, key := range []string{"label", "saved_total", "saved_short", "state", "tool_use"} {
		if _, ok := spawns[0][key]; !ok {
			t.Errorf("--brief spawn lacks %q: %s", key, brief["spawns"])
		}
	}
	var estimate int64
	if err := json.Unmarshal(brief["tokens_saved_estimate"], &estimate); err != nil {
		t.Fatalf("--brief lacks tokens_saved_estimate with the report: %v (%v)", err, keysOf(brief))
	}
	var fullEstimate int64
	if err := json.Unmarshal(full["tokens_saved_estimate"], &fullEstimate); err != nil || estimate != fullEstimate {
		t.Fatalf("--brief tokens_saved_estimate = %d, full = %d (%v)", estimate, fullEstimate, err)
	}
	var short string
	if err := json.Unmarshal(brief["tokens_saved_short"], &short); err != nil || short != telemetry.TokensShort(estimate) {
		t.Fatalf("--brief tokens_saved_short = %q, want %q (%v)", short, telemetry.TokensShort(estimate), err)
	}
}

// --brief --no-report: no figure from the report, only the band's rows.
func TestPanelBriefNoReport_NoTokenFigures(t *testing.T) {
	panelFixtureEnv(t)
	appendPanelFixture(t)

	obj := panelJSON(t, "--brief", "--no-report")
	want := []string{"effort_changed", "held", "held_guardrail", "model_switched", "observed", "session", "spawns", "summary", "switched", "upshift_not_applied"}
	if got := keysOf(obj); !equalStrings(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
}

// The new flags need --panel: alone they are a usage error.
func TestPanelFlags_NeedPanel(t *testing.T) {
	panelFixtureEnv(t)
	for _, flag := range []string{"--no-report", "--brief"} {
		var out, errOut bytes.Buffer
		if rc := runSession([]string{flag}, &out, &errOut, nil); rc != 2 {
			t.Errorf("session %s without --panel: rc = %d, want 2", flag, rc)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
