// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// dsmon-hook — KiroCrew preToolUse lifecycle tracker
// Writes a line to ~/.harness-downshift/agents.jsonl for every subagent spawn
// so dsmon can show active agents. Always exits 0 (never blocks a spawn).
//
// Build:  go build -o dsmon-hook ./cmd/dsmon-hook
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/harness-downshift/internal/telemetry"
)

var spawnTools = map[string]bool{
	"spawn_run":        true,
	"spawn_sub_agents": true,
	"subagent":         true,
	"agent_crew":       true,
	"use_subagent":     true,
}

type hookEvent struct {
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
}

type agentEvent struct {
	Timestamp string `json:"timestamp"`
	Event     string `json:"event"`             // "spawn"
	Tool      string `json:"tool"`              // e.g. "spawn_run"
	Harness   string `json:"harness,omitempty"` // harness name when provided by the caller
	Session   string `json:"session"`           // SHA-256 of KIRO_SESSION_ID, never the raw value
	Task      string `json:"task"`              // always empty for new events (privacy: no prompt storage); key kept so old readers parse
	Model     string `json:"model"`             // requested model if set
}

func main() {
	// Always exit 0 — we are observers only, never gatekeepers
	defer os.Exit(0)
	run()
}

func run() {

	var ev hookEvent
	sc := bufio.NewScanner(os.Stdin)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	raw := strings.Join(lines, "")
	if raw == "" {
		return
	}
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		return
	}
	if !spawnTools[ev.ToolName] {
		return // not a spawn — exit 0 silently
	}

	var ti map[string]any
	json.Unmarshal(ev.ToolInput, &ti) //nolint:errcheck

	// Privacy: never persist task/prompt text. The task key stays in the
	// schema (empty) so old readers keep parsing new lines.
	model := ""
	if v, ok := ti["model"].(string); ok {
		model = v
	}
	harness := ""
	if v, ok := ti["harness"].(string); ok {
		harness = v
	}

	ae := agentEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Event:     "spawn",
		Tool:      ev.ToolName,
		Harness:   harness,
		Session:   telemetry.HashSessionID(os.Getenv("KIRO_SESSION_ID")),
		Task:      "",
		Model:     model,
	}

	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".harness-downshift", "agents.jsonl")
	// Mirror telemetry.AppendTo: create parents locked down, tighten
	// permissions on existing files, and sync before returning.
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	// OpenFile mode applies only on creation, so enforce 0600 here too.
	if err := f.Chmod(0600); err != nil {
		return
	}

	line, err := json.Marshal(ae)
	if err != nil {
		return
	}
	fmt.Fprintf(f, "%s\n", line)
	if err := f.Sync(); err != nil {
		return
	}
}
