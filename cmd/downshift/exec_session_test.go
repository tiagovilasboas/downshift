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

// A run the hook did not tag (a direct `downshift compact -- …`, a compound
// the policy left alone) still belongs to the session in the harness's own
// session variable. The flag wins; no variable means no session. Only the
// hash is logged.
func TestRunExec_SessionFallsBackToHarnessEnv(t *testing.T) {
	flagHash := telemetry.HashSessionID("flag-session")
	cases := []struct {
		name string
		env  string
		args []string
		want string
	}{
		{"env only", "env-session-1", []string{"--"}, telemetry.HashSessionID("env-session-1")},
		{"flag wins over env", "env-session-1", []string{"--session=" + flagHash, "--"}, flagHash},
		{"invalid flag falls back to env", "env-session-1", []string{"--session=not-hex", "--"}, telemetry.HashSessionID("env-session-1")},
		{"neither", "", []string{"--"}, ""},
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
			t.Setenv(core.ClaudeCodeCaps.SessionEnv, tc.env)

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
			if evs[0].Session != tc.want {
				t.Fatalf("session = %q, want %q", evs[0].Session, tc.want)
			}
			if raw, _ := os.ReadFile(path); tc.env != "" && strings.Contains(string(raw), tc.env) {
				t.Fatal("the raw session id reached the log")
			}
		})
	}
}
