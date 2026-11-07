// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/testenv"
)

// bashCallPayload is a Claude Code Bash PreToolUse payload with the given
// permission mode and run_in_background value.
func bashCallPayload(t *testing.T, command, cwd, mode string, background bool) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"session_id":      "s1",
		"cwd":             cwd,
		"permission_mode": mode,
		"tool_input": map[string]any{
			"command":           command,
			"description":       "Run the unit tests for the whole module",
			"timeout":           120000,
			"run_in_background": background,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// In a mode where Claude Code does not ask, a deny rule Downshift cannot read
// (--disallowedTools, --settings, server-managed policy) is the only guard and
// would not match the wrapped command, so only one read-only command is
// wrapped. A background call is never wrapped: the compactor holds output
// until the command exits.
func TestShellRewrite_PermissionModeAndBackground(t *testing.T) {
	wrapped := func(cmd string) string {
		return "'" + testExe + "' compact --session=" + testSession + " -- " + cmd
	}
	cases := []struct {
		name       string
		mode       string
		background bool
		command    string
		want       string // rewritten command; "" means unchanged ({})
	}{
		{"bypassPermissions mutating gh", "bypassPermissions", false, "gh pr merge 12 --squash", ""},
		{"bypassPermissions test runner", "bypassPermissions", false, "go test ./...", ""},
		{"bypassPermissions read-only git", "bypassPermissions", false, "git status", wrapped("git status")},
		{"dontAsk test runner", "dontAsk", false, "go test ./...", ""},
		{"unknown mode fails safe", "someFutureMode", false, "go test ./...", ""},
		{"default mode test runner", "default", false, "go test ./...", wrapped("go test ./...")},
		{"absent mode test runner", "", false, "go test ./...", wrapped("go test ./...")},
		{"plan mode test runner", "plan", false, "go test ./...", wrapped("go test ./...")},
		{"acceptEdits test runner", "acceptEdits", false, "go test ./...", wrapped("go test ./...")},
		{"background test runner", "default", true, "go test ./...", ""},
		{"background read-only git", "bypassPermissions", true, "git status", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testenv.SetHome(t, t.TempDir())
			t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
			withExec(t, true, nil, true)
			cwd := userSettings(t, nil, nil, nil)
			var rc int
			out := captureStdout(func() {
				rc = harnessRegistry["claude-code"].Hook(strings.NewReader(bashCallPayload(t, tc.command, cwd, tc.mode, tc.background)), cmdCat)
			})
			if rc != 0 {
				t.Fatalf("rc = %d, a shell call must never be denied", rc)
			}
			if tc.want == "" {
				if strings.TrimSpace(out) != "{}" {
					t.Fatalf("stdout = %q, want {}", out)
				}
				return
			}
			assertShellRewrite(t, out, tc.want)
		})
	}
}
