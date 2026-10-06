// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/routingv2/classifier"
	"github.com/tiagovilasboas/harness-downshift/internal/routingv2/training"
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
			if rc := runHookAdapter(bytes.NewReader(payload),
				func(b []byte) (codex.Event, error) { var e codex.Event; return e, json.Unmarshal(b, &e) },
				func(e codex.Event) (any, string, core.Decision) { return codex.Handle(e, cmdCat) },
				printCodexAllow, func(e codex.Event) string { return e.TaskText() }); rc != 0 {
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
