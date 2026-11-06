// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/discovery"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// TestRunExec_KiroCrewModel verifies that when KIRO_SESSION_ID is set and
// KiroCrew session files exist, the exec event records the inferred model.
func TestRunExec_KiroCrewModel(t *testing.T) {
	sessionID := "test-session-for-model-inference"
	wantModel := "claude-sonnet-4.6"

	// Set up a fake KiroCrew directory with session files.
	crewDir := t.TempDir()
	t.Setenv(discovery.EnvKiroCrewDir, crewDir)

	// Create subagents/<id>/state.json with the session_id and resolved_model.
	subagentDir := filepath.Join(crewDir, "subagents", "abc123")
	if err := os.MkdirAll(subagentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stateJSON := `{"session_id":"` + sessionID + `","resolved_model":"` + wantModel + `"}`
	if err := os.WriteFile(filepath.Join(subagentDir, "state.json"), []byte(stateJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	// Clear any cached model from prior tests.
	discovery.ResetKiroModelCache()

	// Set up exec environment.
	state := t.TempDir()
	t.Setenv(paths.EnvStateDir, state)
	t.Setenv(execwrap.DepthEnv, "")

	// Clear all harness env vars, then set KIRO_SESSION_ID.
	for _, n := range core.AllHarnessCaps() {
		if n.Caps.SessionEnv != "" {
			t.Setenv(n.Caps.SessionEnv, "")
		}
	}
	t.Setenv(core.KiroCrewCaps.SessionEnv, sessionID)

	var out, errb bytes.Buffer
	runExec([]string{"--", "downshift-test-no-such-program"}, strings.NewReader(""), &out, &errb)

	path, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	evs, err := execwrap.ReadEvents(path)
	if err != nil || len(evs) != 1 {
		t.Fatalf("events = %+v, err = %v", evs, err)
	}
	if evs[0].Harness != "kirocrew" {
		t.Errorf("harness = %q, want %q", evs[0].Harness, "kirocrew")
	}
	if evs[0].Model != wantModel {
		t.Errorf("model = %q, want %q", evs[0].Model, wantModel)
	}
}

// TestRunExec_ModelEmptyWhenNotKiroCrew verifies that for non-KiroCrew harnesses,
// the model field remains empty (those harnesses do not expose model info).
func TestRunExec_ModelEmptyWhenNotKiroCrew(t *testing.T) {
	state := t.TempDir()
	t.Setenv(paths.EnvStateDir, state)
	t.Setenv(execwrap.DepthEnv, "")

	// Clear all harness env vars, then set Claude Code's.
	for _, n := range core.AllHarnessCaps() {
		if n.Caps.SessionEnv != "" {
			t.Setenv(n.Caps.SessionEnv, "")
		}
	}
	t.Setenv(core.ClaudeCodeCaps.SessionEnv, "some-claude-session")

	var out, errb bytes.Buffer
	runExec([]string{"--", "downshift-test-no-such-program"}, strings.NewReader(""), &out, &errb)

	path, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	evs, err := execwrap.ReadEvents(path)
	if err != nil || len(evs) != 1 {
		t.Fatalf("events = %+v, err = %v", evs, err)
	}
	if evs[0].Harness != "claude-code" {
		t.Errorf("harness = %q, want %q", evs[0].Harness, "claude-code")
	}
	if evs[0].Model != "" {
		t.Errorf("model = %q, want empty (Claude Code does not expose model)", evs[0].Model)
	}
}
