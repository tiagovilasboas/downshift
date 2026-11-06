// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// dsmon-hook — KiroCrew preToolUse lifecycle tracker
// Writes a line to ~/.harness-downshift/agents.jsonl for every subagent spawn
// so dsmon can show active agents. Always exits 0 (never blocks a spawn).
//
// Build:  go build -o dsmon-hook ./cmd/dsmon-hook
package main

import (
	"encoding/json"
	"fmt"
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"github.com/tiagovilasboas/downshift/internal/telemetry/fromcore"
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
	fromcore.Install(catalog.Load())
	// Always exit 0 — we are observers only, never gatekeepers
	defer os.Exit(0)
	run()
}

const (
	maxPayloadBytes = 1 << 20
	readTimeout     = 2 * time.Second
)

// readPayload reads at most maxPayloadBytes from in and gives up after
// readTimeout, so a stalled or oversized stdin can never hang a spawn.
func readPayload(in io.Reader, timeout time.Duration) ([]byte, bool) {
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		data, err := io.ReadAll(io.LimitReader(in, maxPayloadBytes+1))
		done <- result{data, err}
	}()
	select {
	case r := <-done:
		if r.err != nil || len(r.data) > maxPayloadBytes {
			return nil, false
		}
		return r.data, true
	case <-time.After(timeout):
		return nil, false
	}
}

// buildEvent maps a hook payload to the agents.jsonl record. Untrusted
// fields (model, harness) are bounded before they reach durable storage;
// the monitors read this file.
func buildEvent(raw []byte) (agentEvent, bool) {
	var ev hookEvent
	if len(strings.TrimSpace(string(raw))) == 0 {
		return agentEvent{}, false
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		return agentEvent{}, false
	}
	if !spawnTools[ev.ToolName] {
		return agentEvent{}, false // not a spawn
	}

	var ti map[string]any
	json.Unmarshal(ev.ToolInput, &ti) //nolint:errcheck

	// Privacy: never persist task/prompt text. The task key stays in the
	// schema (empty) so old readers keep parsing new lines.
	model := ""
	if v, ok := ti["model"].(string); ok {
		model = telemetry.ModelOrUnknown(v)
	}
	harness := ""
	if v, ok := ti["harness"].(string); ok {
		harness = telemetry.ModelOrUnknown(v)
		if harness == "unknown" {
			harness = ""
		}
	}

	return agentEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Event:     "spawn",
		Tool:      ev.ToolName,
		Harness:   harness,
		Session:   telemetry.HashSessionID(os.Getenv("KIRO_SESSION_ID")),
		Task:      "",
		Model:     model,
	}, true
}

func run() {
	raw, ok := readPayload(os.Stdin, readTimeout)
	if !ok {
		return
	}
	ae, ok := buildEvent(raw)
	if !ok {
		return
	}

	path, err := paths.AgentsPath()
	if err != nil {
		return
	}
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
