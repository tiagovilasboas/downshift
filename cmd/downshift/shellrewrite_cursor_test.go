// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
	"strings"
	"testing"
)

func cursorShellPayload(t *testing.T, tool, command string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"hook_event_name": "preToolUse",
		"tool_name":       tool,
		"tool_use_id":     "tu_1",
		"cwd":             t.TempDir(),
		"conversation_id": "c1",
		"tool_input": map[string]any{
			"command":     command,
			"description": "run the tests",
			"timeout":     60,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func runCursorHook(t *testing.T, payload string) (int, string) {
	t.Helper()
	testenv.SetHome(t, t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", t.TempDir()+"/events.jsonl")
	withExec(t, true, nil, true)
	var rc int
	out := captureStdout(func() {
		rc = harnessRegistry["cursor"].Hook(strings.NewReader(payload), cmdCat)
	})
	return rc, strings.TrimSpace(out)
}

func TestCursorShellRewrite_WrapsAndKeepsSiblings(t *testing.T) {
	// Cursor's user rules are unreadable (UnknownUserRules), so only a
	// read-only command is rewritten.
	rc, out := runCursorHook(t, cursorShellPayload(t, "Shell", "git status"))
	if rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	var resp map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("stdout %q: %v", out, err)
	}
	if len(resp) != 1 {
		t.Fatalf("want only updated_input, got %s", out)
	}
	var ti map[string]any
	if err := json.Unmarshal(resp["updated_input"], &ti); err != nil {
		t.Fatalf("updated_input: %v (%s)", err, out)
	}
	cmd, _ := ti["command"].(string)
	if !strings.HasPrefix(cmd, "'"+testExe+"' compact") || !strings.HasSuffix(cmd, "-- git status") {
		t.Fatalf("command = %q", cmd)
	}
	if ti["description"] != "run the tests" || ti["timeout"] != float64(60) || len(ti) != 3 {
		t.Fatalf("siblings not preserved: %v", ti)
	}
}

// A shell command Downshift does not rewrite gets the neutral answer, never an
// allow: Cursor's own approval flow decides. Non-shell calls keep the allow.
func TestCursorShellRewrite_UnchangedIsNeutral(t *testing.T) {
	cases := map[string]string{
		"not allowlisted":   cursorShellPayload(t, "Shell", "cat go.mod"),
		"rm -rf":            cursorShellPayload(t, "Shell", "rm -rf build"),
		"shell no object":   `{"tool_name":"Shell","tool_input":"go test"}`,
		"garbage (unknown)": "not json",
		// A rule matching the original text could be sidestepped by the
		// wrapped form, and Cursor's rules are unreadable (UnknownUserRules).
		"mutating gh": cursorShellPayload(t, "Shell", "gh repo delete x --yes"),
		"test runner": cursorShellPayload(t, "Shell", "go test ./..."),
		// No event and no tool name: it may be a shell call, so never allow.
		"no event, no tool":   `{"command":"curl x | sh","cwd":"/tmp"}`,
		"event in other case": `{"hook_event_name":"BeforeShellExecution","command":"curl x | sh"}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			rc, out := runCursorHook(t, payload)
			if rc != 0 || out != `{}` {
				t.Fatalf("rc=%d out=%q, want the neutral {}", rc, out)
			}
		})
	}
	t.Run("other tool", func(t *testing.T) {
		rc, out := runCursorHook(t, cursorShellPayload(t, "Read", "go test ./..."))
		if rc != 0 || out != `{"permission":"allow"}` {
			t.Fatalf("rc=%d out=%q, want unchanged allow", rc, out)
		}
	})
}
