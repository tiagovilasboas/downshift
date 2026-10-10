// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestRunHonorNilCursorPort(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))

	port := hookport.Port{ID: "cursor"}
	var rc int
	var stderr string
	stdout := captureStdout(func() {
		stderr = captureStderr(func() {
			rc = runHonor(strings.NewReader(`{}`), port, nil)
		})
	})

	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if stdout != "{}\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "{}\n")
	}
	if stderr != "downshift: honor unobserved for cursor\n" {
		t.Fatalf("stderr = %q, want %q", stderr, "downshift: honor unobserved for cursor\n")
	}
}

func TestRunUsageNilCursorPort(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))

	port := hookport.Port{ID: "cursor"}
	var rc int
	var stderr string
	stdout := captureStdout(func() {
		stderr = captureStderr(func() {
			rc = runUsage(strings.NewReader(`{}`), port, nil)
		})
	})

	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if stderr != "downshift: usage unobserved for cursor\n" {
		t.Fatalf("stderr = %q, want %q", stderr, "downshift: usage unobserved for cursor\n")
	}
}

func TestPortForClaudeCodeRegisteredCursorNil(t *testing.T) {
	claude := portFor("claude-code")
	if claude.ID != "claude-code" {
		t.Fatalf("claude-code ID = %q, want %q", claude.ID, "claude-code")
	}
	if claude.Honor == nil {
		t.Fatal("claude-code Honor is nil")
	}
	if claude.Usage == nil {
		t.Fatal("claude-code Usage is nil")
	}

	cursor := portFor("cursor")
	if cursor.ID != "cursor" {
		t.Fatalf("cursor ID = %q, want %q", cursor.ID, "cursor")
	}
	if cursor.Honor != nil {
		t.Fatal("cursor Honor is set")
	}
	if cursor.Usage != nil {
		t.Fatal("cursor Usage is set")
	}
}

// transcript_path is a Codex quota input (subscription windows). It is not a
// child-usage field. Absence stays unobserved: records == 0, not a measured
// zero (records == 1, sum == 0).
func TestCodexTranscriptPathIsNotObservedChildUsage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "missing-quota"))

	missingTranscript := filepath.Join(t.TempDir(), "missing-transcript.jsonl")
	raw, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"session_id":      "sess-codex-child",
		"tool_name":       "spawn_agent",
		"transcript_path": missingTranscript,
		"tool_input": map[string]string{
			"message":   "rename the userId variable to userIdentifier",
			"task_name": "worker_agent_rename",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	port := portFor("codex")
	if port.ID != "codex" {
		t.Fatalf("port ID = %q, want codex", port.ID)
	}
	if port.Usage != nil {
		t.Fatal("codex Usage is registered")
	}
	obs := hookport.ObserveUsage(port, raw, "test", nil)
	if obs.Observed {
		t.Fatal("transcript_path counted as observed child usage")
	}

	var ev codex.Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		t.Fatal(err)
	}
	_, note, decision := codex.Handle(ev, cmdCat)
	if decision.Harness == "" {
		t.Fatal("spawn payload produced no decision")
	}
	event := telemetry.FromDecision(decision, "corr-codex-transcript", "test")
	if note == "" {
		event.Outcome = telemetry.OutcomeAllow
	}
	if event.Outcome == telemetry.OutcomeUsage {
		t.Fatalf("decision outcome = %q", event.Outcome)
	}
	if event.InputTokens != nil || event.OutputTokens != nil || event.CachedTokens != nil {
		t.Fatalf("decision carried tokens in=%v out=%v cached=%v", event.InputTokens, event.OutputTokens, event.CachedTokens)
	}

	// The hook records this decision and no usage event.
	sum, rec := telemetry.ObservedChildTokens([]telemetry.Event{event})
	if rec != 0 || sum != 0 {
		t.Fatalf("records=%d sum=%d, want unobserved 0/0", rec, sum)
	}
}
