// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/codeintel"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestDCIStats_DisplaySection(t *testing.T) {
	tempDir := t.TempDir()
	dciLog := filepath.Join(tempDir, "dci-events.jsonl")
	routeLog := filepath.Join(tempDir, "events.jsonl")

	t.Setenv("DOWNSHIFT_DCI_EVENT_LOG", dciLog)
	t.Setenv("DOWNSHIFT_EVENT_LOG", routeLog)

	now := time.Now().UTC()
	events := []codeintel.Telemetry{
		{
			TotalDurationMs:  12,
			InjectionApplied: true,
			OutputBytes:      1500,
			Timestamp:        now.Add(-1 * time.Hour),
		},
		{
			TotalDurationMs:  8,
			InjectionApplied: true,
			OutputBytes:      2000,
			Timestamp:        now.Add(-2 * time.Hour),
		},
		{
			TotalDurationMs:  4,
			InjectionApplied: false,
			AbstentionReason: "conversational_greeting",
			Timestamp:        now.Add(-3 * time.Hour),
		},
	}

	for _, ev := range events {
		if err := codeintel.RecordTelemetryTo(dciLog, ev); err != nil {
			t.Fatalf("failed to record DCI event: %v", err)
		}
	}

	var rc int
	out := captureStdout(func() {
		rc = runStats([]string{})
	})

	if rc != 0 {
		t.Fatalf("runStats rc = %d, want 0", rc)
	}

	expectedSubstrings := []string{
		"Downshift Code Intelligence (DCI)",
		"Prompts evaluated            3",
		"Injections applied           2   66.7%",
		"Abstentions                  1   33.3%",
		"conversational greeting      1",
		"Context bytes emitted     3500  (total)",
		"Avg bytes / injection   1750.0",
		"Latency (p50 / p95)",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(out, sub) {
			t.Errorf("expected stdout to contain %q, got:\n%s", sub, out)
		}
	}
}

func TestDCIStats_ExportJSON(t *testing.T) {
	tempDir := t.TempDir()
	dciLog := filepath.Join(tempDir, "dci-events.jsonl")
	routeLog := filepath.Join(tempDir, "events.jsonl")

	t.Setenv("DOWNSHIFT_DCI_EVENT_LOG", dciLog)
	t.Setenv("DOWNSHIFT_EVENT_LOG", routeLog)

	now := time.Now().UTC()
	events := []codeintel.Telemetry{
		{
			TotalDurationMs:  25,
			InjectionApplied: true,
			OutputBytes:      2200,
			Timestamp:        now.Add(-30 * time.Minute),
		},
		{
			TotalDurationMs:  5,
			InjectionApplied: false,
			AbstentionReason: "chatter",
			Timestamp:        now.Add(-15 * time.Minute),
		},
	}

	for _, ev := range events {
		if err := codeintel.RecordTelemetryTo(dciLog, ev); err != nil {
			t.Fatalf("failed to record DCI event: %v", err)
		}
	}

	var rc int
	out := captureStdout(func() {
		rc = runStats([]string{"--export"})
	})

	if rc != 0 {
		t.Fatalf("runStats --export rc = %d, want 0", rc)
	}

	var exportSummary telemetry.Summary
	if err := json.Unmarshal([]byte(out), &exportSummary); err != nil {
		t.Fatalf("failed to unmarshal JSON export: %v\nRaw output:\n%s", err, out)
	}

	if exportSummary.DCISummary == nil {
		t.Fatalf("expected export to contain non-nil dci summary")
	}

	dci := exportSummary.DCISummary
	if dci.TotalPrompts != 2 {
		t.Errorf("TotalPrompts = %d, want 2", dci.TotalPrompts)
	}
	if dci.InjectionsApplied != 1 {
		t.Errorf("InjectionsApplied = %d, want 1", dci.InjectionsApplied)
	}
	if dci.Abstentions != 1 {
		t.Errorf("Abstentions = %d, want 1", dci.Abstentions)
	}
	if dci.TotalContextBytes != 2200 {
		t.Errorf("TotalContextBytes = %d, want 2200", dci.TotalContextBytes)
	}
	if exportSummary.Note == "" {
		t.Errorf("expected non-empty Note in export summary")
	}
}

func TestDCIStats_ZeroEvents(t *testing.T) {
	tempDir := t.TempDir()
	dciLog := filepath.Join(tempDir, "empty-dci.jsonl")
	routeLog := filepath.Join(tempDir, "empty-events.jsonl")

	t.Setenv("DOWNSHIFT_DCI_EVENT_LOG", dciLog)
	t.Setenv("DOWNSHIFT_EVENT_LOG", routeLog)

	var rc int
	var out string
	errOut := captureStderr(func() {
		out = captureStdout(func() {
			rc = runStats([]string{})
		})
	})

	if rc != 0 {
		t.Fatalf("runStats rc = %d, want 0", rc)
	}
	if out != "" {
		t.Errorf("expected empty stdout when 0 events, got: %q", out)
	}
	if !strings.Contains(errOut, "no events recorded yet") {
		t.Errorf("expected 'no events recorded yet' in stderr, got: %q", errOut)
	}
}

func TestDCIStats_OnlySubagentEventsNoDCI(t *testing.T) {
	tempDir := t.TempDir()
	dciLog := filepath.Join(tempDir, "nonexistent-dci.jsonl")
	routeLog := filepath.Join(tempDir, "events.jsonl")

	t.Setenv("DOWNSHIFT_DCI_EVENT_LOG", dciLog)
	t.Setenv("DOWNSHIFT_EVENT_LOG", routeLog)

	// Record a subagent event
	subagentEv := telemetry.Event{
		SessionID: "sess-test-1234",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Outcome:   "rewrite_emitted",
		Tier:      "standard",
		ToModel:   "claude-haiku",
		FromModel: "claude-opus",
		Harness:   "claude-code",
	}
	if err := telemetry.AppendTo(routeLog, subagentEv); err != nil {
		t.Fatalf("failed to record subagent event: %v", err)
	}

	// 1. Regular stats output should show routing stats, but no DCI header
	out := captureStdout(func() {
		if rc := runStats([]string{}); rc != 0 {
			t.Errorf("runStats rc = %d, want 0", rc)
		}
	})
	if strings.Contains(out, "Downshift Code Intelligence (DCI)") {
		t.Errorf("expected no DCI section when zero DCI events exist, got:\n%s", out)
	}

	// 2. Export JSON should have dci omitted
	exportOut := captureStdout(func() {
		if rc := runStats([]string{"--export"}); rc != 0 {
			t.Errorf("runStats --export rc = %d, want 0", rc)
		}
	})

	var exportMap map[string]any
	if err := json.Unmarshal([]byte(exportOut), &exportMap); err != nil {
		t.Fatalf("failed to parse export JSON: %v", err)
	}
	if _, ok := exportMap["dci"]; ok {
		t.Errorf("expected 'dci' field to be omitted from JSON when zero DCI events exist, got: %v", exportMap["dci"])
	}
}

func TestDCIStats_BothSubagentAndDCIEvents(t *testing.T) {
	tempDir := t.TempDir()
	dciLog := filepath.Join(tempDir, "dci-events.jsonl")
	routeLog := filepath.Join(tempDir, "events.jsonl")

	t.Setenv("DOWNSHIFT_DCI_EVENT_LOG", dciLog)
	t.Setenv("DOWNSHIFT_EVENT_LOG", routeLog)

	subagentEv := telemetry.Event{
		SessionID: "sess-test-5678",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Outcome:   "rewrite_emitted",
		Tier:      "standard",
		ToModel:   "claude-haiku",
		FromModel: "claude-opus",
		Harness:   "claude-code",
	}
	if err := telemetry.AppendTo(routeLog, subagentEv); err != nil {
		t.Fatalf("failed to record subagent event: %v", err)
	}

	dciEv := codeintel.Telemetry{
		TotalDurationMs:  30,
		InjectionApplied: true,
		OutputBytes:      1250,
		Timestamp:        time.Now().UTC(),
	}
	if err := codeintel.RecordTelemetryTo(dciLog, dciEv); err != nil {
		t.Fatalf("failed to record DCI event: %v", err)
	}

	// 1. Regular stats output should show both sections
	out := captureStdout(func() {
		if rc := runStats([]string{}); rc != 0 {
			t.Errorf("runStats rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "Downshift Code Intelligence (DCI)") {
		t.Errorf("expected DCI section in combined output, got:\n%s", out)
	}

	// 2. Export JSON should have both routing data and dci
	exportOut := captureStdout(func() {
		if rc := runStats([]string{"--export"}); rc != 0 {
			t.Errorf("runStats --export rc = %d, want 0", rc)
		}
	})
	var exportSummary telemetry.Summary
	if err := json.Unmarshal([]byte(exportOut), &exportSummary); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if exportSummary.DCISummary == nil || exportSummary.DCISummary.InjectionsApplied != 1 {
		t.Errorf("unexpected dci summary in combined export: %+v", exportSummary.DCISummary)
	}
}

func TestDCIStats_DaysFilter(t *testing.T) {
	tempDir := t.TempDir()
	dciLog := filepath.Join(tempDir, "dci-events.jsonl")
	routeLog := filepath.Join(tempDir, "events.jsonl")

	t.Setenv("DOWNSHIFT_DCI_EVENT_LOG", dciLog)
	t.Setenv("DOWNSHIFT_EVENT_LOG", routeLog)

	now := time.Now().UTC()
	// Event 1: 2 days ago
	_ = codeintel.RecordTelemetryTo(dciLog, codeintel.Telemetry{
		TotalDurationMs:  10,
		InjectionApplied: true,
		OutputBytes:      500,
		Timestamp:        now.Add(-2 * 24 * time.Hour),
	})
	// Event 2: 20 days ago
	_ = codeintel.RecordTelemetryTo(dciLog, codeintel.Telemetry{
		TotalDurationMs:  20,
		InjectionApplied: true,
		OutputBytes:      800,
		Timestamp:        now.Add(-20 * 24 * time.Hour),
	})

	// With --days=7, only Event 1 is included
	out := captureStdout(func() {
		if rc := runStats([]string{"--days=7", "--export"}); rc != 0 {
			t.Errorf("runStats rc = %d, want 0", rc)
		}
	})

	var exportSummary telemetry.Summary
	if err := json.Unmarshal([]byte(out), &exportSummary); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if exportSummary.DCISummary == nil {
		t.Fatalf("expected non-nil dci summary")
	}
	if exportSummary.DCISummary.TotalPrompts != 1 {
		t.Errorf("TotalPrompts with --days=7 = %d, want 1", exportSummary.DCISummary.TotalPrompts)
	}
	if exportSummary.DCISummary.TotalContextBytes != 500 {
		t.Errorf("TotalContextBytes with --days=7 = %d, want 500", exportSummary.DCISummary.TotalContextBytes)
	}
}

func TestDCIStats_DCIFlag(t *testing.T) {
	tempDir := t.TempDir()
	dciLog := filepath.Join(tempDir, "dci-events.jsonl")
	routeLog := filepath.Join(tempDir, "events.jsonl")

	t.Setenv("DOWNSHIFT_DCI_EVENT_LOG", dciLog)
	t.Setenv("DOWNSHIFT_EVENT_LOG", routeLog)

	// When both route and DCI events exist, --dci flag prints only DCI section
	subagentEv := telemetry.Event{
		SessionID: "sess-test-9999",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Outcome:   "rewrite_emitted",
		Tier:      "standard",
		ToModel:   "claude-haiku",
		FromModel: "claude-opus",
		Harness:   "claude-code",
	}
	_ = telemetry.AppendTo(routeLog, subagentEv)

	dciEv := codeintel.Telemetry{
		TotalDurationMs:  15,
		InjectionApplied: true,
		OutputBytes:      900,
		Timestamp:        time.Now().UTC(),
	}
	_ = codeintel.RecordTelemetryTo(dciLog, dciEv)

	out := captureStdout(func() {
		if rc := runStats([]string{"--dci"}); rc != 0 {
			t.Errorf("runStats --dci rc = %d, want 0", rc)
		}
	})

	if !strings.Contains(out, "Downshift Code Intelligence (DCI)") {
		t.Errorf("expected DCI output, got: %s", out)
	}
	if strings.Contains(out, "Subagent decisions") || strings.Contains(out, "Estimated at list price") {
		t.Errorf("--dci should focus on DCI stats, got: %s", out)
	}
}
