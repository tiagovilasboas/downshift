// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routingv2/classifier"
	"github.com/tiagovilasboas/downshift/internal/routingv2/training"
)

func TestShadowHookReviewReportEndToEnd(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "telemetry.jsonl"))
	t.Setenv("DOWNSHIFT_MINILM_EMBED", "")
	t.Setenv("DOWNSHIFT_NO_ROUTE", "")
	frontier := cmdCat.ModelFor("codex", core.TierFrontier).ID
	setSessionAllowlist(t, "codex", []string{frontier, cmdCat.ModelFor("codex", core.TierSmall).ID})
	payload, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse", "tool_name": "spawn_agent", "model": frontier,
		"tool_input": map[string]string{"message": "rename userId private-token-should-not-persist"},
	})
	if err != nil {
		t.Fatal(err)
	}
	hook := func() string {
		return captureStdout(func() {
			if rc := runHookAdapter("codex", bytes.NewReader(payload),
				func(b []byte) (codex.Event, error) { var e codex.Event; return e, json.Unmarshal(b, &e) },
				func(e codex.Event) (any, string, core.Decision) { return codex.Handle(e, cmdCat) },
				printAllow, func(e codex.Event) string { return e.TaskText() }); rc != 0 {
				t.Fatalf("hook rc=%d", rc)
			}
		})
	}
	t.Setenv("DOWNSHIFT_SHADOW_WEIGHTS", "")
	baseline := hook()
	path := filepath.Join(t.TempDir(), "candidate.json")
	w := classifier.DefaultWeights()
	w.Frontier.Bias = 20 // Force disagreement; this must never change hook output.
	if err := classifier.SaveWeights(w, path); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNSHIFT_SHADOW_WEIGHTS", path)
	if got := hook(); got != baseline {
		t.Fatalf("shadow changed production output: %s / %s", baseline, got)
	}
	store := training.NewEventStore(training.DefaultEventsPath())
	events, err := store.Load()
	if err != nil || len(events) != 2 || events[0].Shadow != nil || events[1].Shadow == nil || !events[1].Shadow.Valid() {
		t.Fatalf("events=%+v err=%v", events, err)
	}
	if events[1].Shadow.PolicyTier == events[1].SelectedTier {
		t.Fatal("fixture must disagree with production")
	}
	if rc := runFeedback([]string{events[1].ID, "success"}); rc != 0 {
		t.Fatal(rc)
	}
	reportText := captureStdout(func() {
		if rc := runShadowReport(nil); rc != 0 {
			t.Fatal(rc)
		}
	})
	var report training.ShadowReport
	if err := json.Unmarshal([]byte(reportText), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Models) != 1 || report.Models[0].Labeled != 0 || report.Models[0].ReviewedOutcomes != 1 {
		t.Fatalf("implicit label leaked into report: %s", reportText)
	}
	if rc := runFeedback([]string{events[1].ID, "success", "--required-tier=SMALL"}); rc != 0 {
		t.Fatal(rc)
	}
	reportText = captureStdout(func() {
		if rc := runShadowReport(nil); rc != 0 {
			t.Fatal(rc)
		}
	})
	if err := json.Unmarshal([]byte(reportText), &report); err != nil {
		t.Fatal(err)
	}
	if report.Models[0].Labeled != 1 || report.Models[0].PolicyCorrect != 0 {
		t.Fatalf("report=%s", reportText)
	}
	data, err := os.ReadFile(training.DefaultEventsPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private-token") || strings.Contains(string(data), path) {
		t.Fatal("shadow persisted task text/path")
	}
	t.Setenv("DOWNSHIFT_SHADOW_WEIGHTS", path+"-missing")
	if got := hook(); got != baseline {
		t.Fatal("missing candidate changed hook output")
	}
}

func TestShadowReportCLIValidation(t *testing.T) {
	for _, args := range [][]string{{"--unknown"}, {"--events="}} {
		if rc := runShadowReport(args); rc != 2 {
			t.Fatalf("args=%v rc=%d", args, rc)
		}
	}
	path := t.TempDir()
	if rc := runShadowReport([]string{"--events=" + path}); rc != 1 {
		t.Fatal("unreadable event log must fail")
	}
	out := captureStdout(func() {
		if rc := runShadowReport([]string{"--events=" + filepath.Join(path, "absent.jsonl")}); rc != 0 {
			t.Fatal(rc)
		}
	})
	if !strings.Contains(out, `"models":[]`) {
		t.Fatalf("empty report=%s", out)
	}
}

// TestShadowReport_E5FromTelemetryLog runs the command over an explicit
// telemetry log and checks the JSON keys and the --text headings; with no
// E5 objects the JSON is the same bytes as before the E5 section existed.
func TestShadowReport_E5FromTelemetryLog(t *testing.T) {
	dir := t.TempDir()
	loop := filepath.Join(dir, "loop-events.jsonl") // absent: no candidate observations
	withE5 := filepath.Join(dir, "events.jsonl")
	lines := strings.Join([]string{
		`{"outcome":"allow","harness":"cursor","reason_codes":["RISK_CATEGORY"],"e5_shadow":{"model_id":"e5-prototypes-sha256:abc","nearest_label":"SIMPLE","regex_label":"MEDIUM","latency_ms":12}}`,
		`{"outcome":"allow","harness":"cursor","e5_shadow":{"model_id":"e5-prototypes-sha256:abc","error_code":"timeout","latency_ms":500}}`,
		`{"outcome":"allow","harness":"cursor","e5_fuse":{"model_id":"e5-prototypes-sha256:abc","nearest_label":"MEDIUM","action":"none","latency_ms":9}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(withE5, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	without := filepath.Join(dir, "plain.jsonl")
	if err := os.WriteFile(without, []byte(`{"outcome":"allow","harness":"cursor","complexity":"MEDIUM"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := shadowReport([]string{"--events=" + loop, "--telemetry=" + withE5}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	for _, k := range []string{"e5_shadow", "e5_fuse", "models"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key %q in %s", k, out.String())
		}
	}
	var shadow struct {
		Observed            int `json:"observed"`
		Errors              int `json:"errors"`
		FalseDowngradeProxy int `json:"false_downgrade_proxy"`
	}
	if err := json.Unmarshal(got["e5_shadow"], &shadow); err != nil || shadow.Observed != 2 || shadow.Errors != 1 || shadow.FalseDowngradeProxy != 1 {
		t.Errorf("e5_shadow = %+v (%v)", shadow, err)
	}

	out.Reset()
	if code := shadowReport([]string{"--events=" + loop, "--telemetry=" + withE5, "--text"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "E5 shadow\n") || !strings.Contains(out.String(), "E5 fuse\n") {
		t.Errorf("--text should print both E5 sections:\n%s", out.String())
	}

	out.Reset()
	if code := shadowReport([]string{"--events=" + loop, "--telemetry=" + without}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if want := `{"load_errors":0,"invalid_observations":0,"models":[]}` + "\n"; out.String() != want {
		t.Errorf("JSON without E5 events must be unchanged:\n got %q\nwant %q", out.String(), want)
	}

	out.Reset()
	errOut.Reset()
	if code := shadowReport([]string{"--bogus"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "--telemetry=") {
		t.Errorf("unknown flag: code %d, stderr %q", code, errOut.String())
	}
}
