// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// A hook-wrapped run passes --session=<hash> as the first argument; the exec
// event records entry="hook" and the harness that set the session env var.
// A direct call (no --session flag) records entry="direct" and carries no
// session even when a harness env var is present; this prevents attributing
// a direct `downshift compactor -- …` call to whatever harness happened to be
// running in the same terminal.
func TestRunExec_EntryAndHarness(t *testing.T) {
	flagHash := telemetry.HashSessionID("flag-session")
	cases := []struct {
		name        string
		env         string
		envVar      string // which SessionEnv var to set (default: ClaudeCodeCaps.SessionEnv)
		args        []string
		wantSession string
		wantEntry   string
		wantHarness string
	}{
		{
			name:        "hook with flag and env",
			env:         "env-session-1",
			args:        []string{"--session=" + flagHash, "--"},
			wantSession: flagHash,
			wantEntry:   "hook",
			wantHarness: "claude-code",
		},
		{
			name:        "direct — env present but no flag",
			env:         "env-session-1",
			args:        []string{"--"},
			wantSession: "",
			wantEntry:   "direct",
			wantHarness: "claude-code",
		},
		{
			name:        "direct — no env no flag",
			env:         "",
			args:        []string{"--"},
			wantSession: "",
			wantEntry:   "direct",
			wantHarness: "unknown",
		},
		{
			name:        "invalid flag is ignored — direct",
			env:         "env-session-1",
			args:        []string{"--session=not-hex", "--"},
			wantSession: "",
			wantEntry:   "direct",
			wantHarness: "claude-code",
		},
		{
			name:        "kirocrew — detected via KIRO_SESSION_ID",
			env:         "70156318-1c75-44f0-ab31-e6adb7ebe0f5",
			envVar:      "KIRO_SESSION_ID",
			args:        []string{"--"},
			wantSession: "",
			wantEntry:   "direct",
			wantHarness: "kirocrew",
		},
		{
			name:        "kirocrew hook — session from flag",
			env:         "70156318-1c75-44f0-ab31-e6adb7ebe0f5",
			envVar:      "KIRO_SESSION_ID",
			args:        []string{"--session=" + flagHash, "--"},
			wantSession: flagHash,
			wantEntry:   "hook",
			wantHarness: "kirocrew",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := t.TempDir()
			t.Setenv(paths.EnvStateDir, state)
			t.Setenv(execwrap.DepthEnv, "")
			for _, n := range core.AllHarnessCaps() {
				if n.Caps.SessionEnv != "" {
					t.Setenv(n.Caps.SessionEnv, "")
				}
			}
			// Set the appropriate env var (default to Claude Code)
			envVar := tc.envVar
			if envVar == "" {
				envVar = core.ClaudeCodeCaps.SessionEnv
			}
			t.Setenv(envVar, tc.env)

			var out, errb bytes.Buffer
			runExec(append(tc.args, "downshift-test-no-such-program"), strings.NewReader(""), &out, &errb)

			path, err := execwrap.EventsPath()
			if err != nil {
				t.Fatal(err)
			}
			evs, err := execwrap.ReadEvents(path)
			if err != nil || len(evs) != 1 {
				t.Fatalf("events = %+v, err = %v", evs, err)
			}
			if evs[0].Session != tc.wantSession {
				t.Errorf("session = %q, want %q", evs[0].Session, tc.wantSession)
			}
			if evs[0].Entry != tc.wantEntry {
				t.Errorf("entry = %q, want %q", evs[0].Entry, tc.wantEntry)
			}
			if evs[0].Harness != tc.wantHarness {
				t.Errorf("harness = %q, want %q", evs[0].Harness, tc.wantHarness)
			}
			if raw, _ := os.ReadFile(path); tc.env != "" && strings.Contains(string(raw), tc.env) {
				t.Fatal("the raw session id reached the log")
			}
		})
	}
}

// Old rows written before the harness/entry fields were added must be readable
// and treated as entry="unknown". The omitempty on both fields makes them
// absent from the JSON of old rows.
func TestRunExec_OldRowsReadAsUnknown(t *testing.T) {
	state := t.TempDir()
	t.Setenv(paths.EnvStateDir, state)

	path, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	// Write an old-style row without harness or entry fields.
	oldRow := `{"ts":"2026-01-01T00:00:00Z","family":"git-status","raw_bytes":100,"emitted_bytes":80,"exit_class":"ok","compacted":true,"session":"aabbccdd"}` + "\n"
	if err := os.WriteFile(path, []byte(oldRow), 0o600); err != nil {
		t.Fatal(err)
	}

	evs, err := execwrap.ReadEvents(path)
	if err != nil || len(evs) != 1 {
		t.Fatalf("events = %+v, err = %v", evs, err)
	}
	if evs[0].Entry != "" {
		t.Errorf("old row entry = %q, want empty string (caller treats as unknown)", evs[0].Entry)
	}
	if evs[0].Harness != "" {
		t.Errorf("old row harness = %q, want empty string (caller treats as unknown)", evs[0].Harness)
	}
}
