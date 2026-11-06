// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// runExecOneEvent runs a no-op command through runExec in an isolated state
// directory and returns the single exec event it writes. It clears every
// harness session variable first so the environment is known.
func runExecOneEvent(t *testing.T) execwrap.Event {
	t.Helper()
	t.Setenv(paths.EnvStateDir, t.TempDir())
	t.Setenv(execwrap.DepthEnv, "")
	for _, n := range core.AllHarnessCaps() {
		if n.Caps.SessionEnv != "" {
			t.Setenv(n.Caps.SessionEnv, "")
		}
	}

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
	return evs[0]
}

// With CLAUDE_CODE_ENTRYPOINT set to a valid client, the exec event records it.
func TestRunExec_ClientFromEntrypoint(t *testing.T) {
	t.Setenv(claudeClientEnv, "claude-desktop")

	ev := runExecOneEvent(t)
	if ev.Client != "claude-desktop" {
		t.Errorf("client = %q, want %q", ev.Client, "claude-desktop")
	}
}

// Without the variable the client field stays empty, so omitempty drops it.
func TestRunExec_ClientEmptyWhenUnset(t *testing.T) {
	t.Setenv(claudeClientEnv, "")

	ev := runExecOneEvent(t)
	if ev.Client != "" {
		t.Errorf("client = %q, want empty when %s is unset", ev.Client, claudeClientEnv)
	}
}

// An untrusted value goes through the same telemetry.ClientOrDrop sanitizer the
// decision path uses, so a value with a space or uppercase is dropped, not
// stored. This proves the reuse end to end, not only in the sanitizer's own
// unit test.
func TestRunExec_ClientDroppedWhenInvalid(t *testing.T) {
	for _, bad := range []string{"claude desktop", "CLAUDE-DESKTOP", "claude/desktop", strings.Repeat("a", 33)} {
		t.Run(bad, func(t *testing.T) {
			t.Setenv(claudeClientEnv, bad)

			ev := runExecOneEvent(t)
			if ev.Client != "" {
				t.Errorf("client = %q, want empty for untrusted value %q", ev.Client, bad)
			}
		})
	}
}

// envClient reuses telemetry.ClientOrDrop rather than reimplementing the rule:
// a valid token is kept and an invalid one is dropped.
func TestEnvClient(t *testing.T) {
	t.Setenv(claudeClientEnv, "cli")
	if got := envClient(); got != "cli" {
		t.Errorf("envClient() = %q, want %q", got, "cli")
	}
	t.Setenv(claudeClientEnv, "CLI")
	if got := envClient(); got != "" {
		t.Errorf("envClient() = %q, want empty for untrusted value", got)
	}
}
