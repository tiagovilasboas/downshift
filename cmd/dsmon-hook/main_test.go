// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/telemetry"
)

// feedStdin swaps os.Stdin with a pipe carrying payload and returns a
// restore function. Callers must call restore and run() before reading output.
func feedStdin(t *testing.T, payload string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if _, err := w.WriteString(payload); err != nil {
		t.Fatalf("write stdin pipe: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close stdin pipe: %v", err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		r.Close()
	})
}

func TestRunWritesPrivacySafeLine(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	rawSession := "test-session-raw-abc123"
	t.Setenv("KIRO_SESSION_ID", rawSession)

	secret := "SECRET-PROMPT-UNIQUE-987654321"
	payload := `{"hook_event_name":"PreToolUse","tool_name":"spawn_run",` +
		`"tool_input":{"task":"` + secret + ` do something secret","model":"sonnet","harness":"claude-code"}}`
	feedStdin(t, payload)

	run()

	path := filepath.Join(home, ".harness-downshift", "agents.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read agents.jsonl: %v", err)
	}
	if strings.Contains(string(data), secret) {
		t.Fatalf("agents.jsonl contains prompt text; privacy violation")
	}

	line := strings.TrimSpace(string(data))
	var ae agentEvent
	if err := json.Unmarshal([]byte(line), &ae); err != nil {
		t.Fatalf("unmarshal agent line: %v", err)
	}
	if ae.Task != "" {
		t.Fatalf("task must be empty for new events, got %q", ae.Task)
	}
	wantSession := telemetry.HashSessionID(rawSession)
	if wantSession == "" {
		t.Fatalf("test setup: HashSessionID returned empty for %q", rawSession)
	}
	if ae.Session != wantSession {
		t.Fatalf("session = %q, want hashed %q", ae.Session, wantSession)
	}
	if ae.Session == rawSession {
		t.Fatalf("session stored raw; must be hashed")
	}
	if ae.Tool != "spawn_run" {
		t.Fatalf("tool = %q, want spawn_run", ae.Tool)
	}
	if ae.Model != "sonnet" {
		t.Fatalf("model = %q, want sonnet", ae.Model)
	}
	if ae.Harness != "claude-code" {
		t.Fatalf("harness = %q, want claude-code", ae.Harness)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat agents.jsonl: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0600 {
		t.Fatalf("agents.jsonl perm = %o, want 600", perm)
	}
}

func TestRunIgnoresNonSpawnTool(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KIRO_SESSION_ID", "some-session-id")

	feedStdin(t, `{"hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"path":"main.go"}}`)
	run()

	path := filepath.Join(home, ".harness-downshift", "agents.jsonl")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("agents.jsonl should not be created for non-spawn tools")
	}
}
