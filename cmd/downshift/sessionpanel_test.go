// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// panelFixtureEnv isolates the state dir and clears the harness session env,
// so `session --panel` reads only what the test writes.
func panelFixtureEnv(t *testing.T) {
	t.Helper()
	endEnv(t)
	t.Setenv(paths.EnvStateDir, t.TempDir())
	t.Setenv(execwrap.DepthEnv, "")
	for _, n := range core.AllHarnessCaps() {
		if n.Caps.SessionEnv != "" {
			t.Setenv(n.Caps.SessionEnv, "")
		}
	}
}

// The compactor block is the report's Compactor over the report's window.
// It totals every session's runs, attributed or not, and the raw session id
// never reaches the panel.
func TestPanel_CompactorIsTheReportWindowOfAllRuns(t *testing.T) {
	panelFixtureEnv(t)
	bin := t.TempDir()
	var suite strings.Builder
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&suite, "ok  \tsecret/pkg/p%d\t0.1s\n", i)
	}
	if err := os.WriteFile(filepath.Join(bin, "suite.txt"), []byte(suite.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncat \"$(dirname \"$0\")/suite.txt\"\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	mine, other := telemetry.HashSessionID("s-mine"), telemetry.HashSessionID("s-other")
	for _, args := range [][]string{
		{"--session=" + mine, "--"},
		{"--session=" + other, "--"},
		{"--"},                   // a direct call with no session
		{"--session=../x", "--"}, // a malformed session is ignored, the run still counts
	} {
		var out, errb bytes.Buffer
		if rc := runExec(append(args, "go", "test", "./secret/..."), strings.NewReader(""), &out, &errb); rc != 0 {
			t.Fatalf("rc = %d, stderr %q", rc, errb.String())
		}
	}

	execEvents, err := accounting.ReadExecEvents()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(panelWithReport(telemetry.PanelView{Session: mine}, nil, execEvents))
	if err != nil {
		t.Fatal(err)
	}
	var got panelOut
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	c := got.Compactor
	if c == nil || c.Scope != "last 30 days" || c.Commands != 4 || c.Compacted != 4 || c.SavedBytes <= 0 {
		t.Fatalf("compactor block = %+v, want 4 compacted runs across every session, scope = report window", c)
	}
	if got.TokensSavedEstimate != c.EstimatedTokensSaved || c.EstimatedTokensSaved <= 0 {
		t.Fatalf("tokens_saved_estimate = %d, want the compactor estimate %d (no routing tokens)", got.TokensSavedEstimate, c.EstimatedTokensSaved)
	}
	if strings.Contains(string(raw), "s-mine") || strings.Contains(string(raw), "s-other") {
		t.Fatalf("a raw session id leaked into the panel: %s", raw)
	}
}

// The panel's report block equals `downshift report --json` for the same logs:
// same window, same numbers, same final figure, and the compactor block is the
// report's. An exec run older than the window is in neither.
func TestPanel_ReportEqualsReportJSON(t *testing.T) {
	panelFixtureEnv(t)
	ev := telemetry.Event{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Harness: "claude-code", SessionID: "s-1",
		Agent: "worker", Complexity: "SIMPLE", FromModel: "claude-opus-4-7", ToModel: "claude-haiku-4-5",
		Verdict: "DOWNSHIFT", Outcome: "rewrite_emitted", Tier: "SMALL",
	}
	if err := telemetry.AppendTo(os.Getenv("DOWNSHIFT_EVENT_LOG"), ev); err != nil {
		t.Fatal(err)
	}
	execPath, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	old := execwrap.Event{TS: time.Now().UTC().Add(-40 * 24 * time.Hour).Format(time.RFC3339), RawBytes: 4000, Compacted: true}
	recent := execwrap.Event{TS: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339), RawBytes: 400, EmittedBytes: 100, Compacted: true, Session: telemetry.HashSessionID("s-1")}
	for _, e := range []execwrap.Event{old, recent} {
		if err := execwrap.Append(execPath, e); err != nil {
			t.Fatal(err)
		}
	}

	var repOut, repErr bytes.Buffer
	if rc := runReport([]string{"--json"}, &repOut, &repErr); rc != 0 {
		t.Fatalf("report rc=%d %s", rc, repErr.String())
	}
	var rep struct {
		TokensSavedEstimate int64              `json:"tokens_saved_estimate"`
		Numbers             accounting.Numbers `json:"numbers"`
	}
	if err := json.Unmarshal(repOut.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}

	var panOut, panErr bytes.Buffer
	if rc := runSession([]string{"--panel"}, &panOut, &panErr, nil); rc != 0 {
		t.Fatalf("session --panel rc=%d %s", rc, panErr.String())
	}
	var pan panelOut
	if err := json.Unmarshal(panOut.Bytes(), &pan); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(pan.Report, rep.Numbers) {
		t.Fatalf("panel report = %+v\nreport numbers = %+v", pan.Report, rep.Numbers)
	}
	if pan.TokensSavedEstimate != rep.TokensSavedEstimate || pan.TokensSavedEstimate != rep.Numbers.TokensSavedEstimate {
		t.Fatalf("panel tokens_saved_estimate = %d, report = %d", pan.TokensSavedEstimate, rep.TokensSavedEstimate)
	}
	if pan.Compactor == nil || pan.Compactor.Scope != rep.Numbers.Scope.Label() ||
		pan.Compactor.Commands != rep.Numbers.Compactor.Commands || pan.Compactor.SavedBytes != rep.Numbers.Compactor.SavedBytes ||
		pan.Compactor.EstimatedTokensSaved != rep.Numbers.Compactor.Tokens {
		t.Fatalf("panel compactor = %+v, report compactor = %+v", pan.Compactor, rep.Numbers.Compactor)
	}
	if pan.Compactor.Commands != 1 || pan.Compactor.SavedBytes != 300 {
		t.Fatalf("compactor = %+v, want the recent run only (1 command, 300 bytes)", pan.Compactor)
	}
}

// The session fields of the panel are the PanelView unchanged: every key the
// view marshals keeps its value in the panel JSON.
func TestPanel_SessionFieldsUnchanged(t *testing.T) {
	view := telemetry.PanelView{Session: "h", Switched: 2, ModelSwitched: 2, Held: 1, Tokens: telemetry.TokenSaving{WorkSaved: 300, CacheSaved: 200}}
	viewRaw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	panelRaw, err := json.Marshal(panelWithReport(view, nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	var want, got map[string]json.RawMessage
	if err := json.Unmarshal(viewRaw, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(panelRaw, &got); err != nil {
		t.Fatal(err)
	}
	for key, w := range want {
		if string(got[key]) != string(w) {
			t.Errorf("panel %q = %s, want the session view's %s", key, got[key], w)
		}
	}
	if _, ok := got["report"]; !ok {
		t.Errorf("panel lacks the report block: %s", panelRaw)
	}
}

// A log with no exec events has no compactor block; the final figure is the
// report's routing figure alone.
func TestPanel_NoExecEventsNoCompactorBlock(t *testing.T) {
	events := []telemetry.Event{{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Harness: "claude-code", SessionID: "s-1",
		Agent: "worker", Complexity: "SIMPLE", FromModel: "claude-opus-4-7", ToModel: "claude-haiku-4-5",
		Verdict: "DOWNSHIFT", Outcome: "rewrite_emitted", Tier: "SMALL",
	}}
	out := panelWithReport(telemetry.PanelView{Session: "h"}, events, nil)
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	if _, ok := top["compactor"]; ok {
		t.Fatalf("an empty exec log must carry no compactor block: %s", raw)
	}
	if out.Report.Compactor.Commands != 0 {
		t.Fatalf("report compactor commands = %d, want 0", out.Report.Compactor.Commands)
	}
	if out.TokensSavedEstimate != out.Report.Routing.WorkSaved+out.Report.Routing.CacheSaved {
		t.Fatalf("tokens_saved_estimate = %d, want the routing figure alone (%+v)", out.TokensSavedEstimate, out.Report.Routing)
	}
}
