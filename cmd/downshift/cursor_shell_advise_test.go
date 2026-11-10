// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
	"strings"
	"testing"
)

// cursorShellExecPayload is Cursor's beforeShellExecution input for one command.
func cursorShellExecPayload(t *testing.T, command string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"hook_event_name": "beforeShellExecution",
		"command":         command,
		"cwd":             t.TempDir(),
		"sandbox":         false,
		"conversation_id": "c1",
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// runCursorShellExec runs the registered cursor hook on one beforeShellExecution payload.
func runCursorShellExec(t *testing.T, payload string, env map[string]string) (int, string) {
	t.Helper()
	withExec(t, true, env, true)
	testenv.SetHome(t, t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", t.TempDir()+"/events.jsonl")
	var rc int
	out := captureStdout(func() {
		rc = harnessRegistry["cursor"].Hook(strings.NewReader(payload), cmdCat)
	})
	return rc, strings.TrimSpace(out)
}

// A compressing command gets allow plus agent_message naming the compact form.
func TestCursorShellAdvise_CompressingFamilyAllowsWithAdvice(t *testing.T) {
	rc, out := runCursorShellExec(t, cursorShellExecPayload(t, "go test ./..."), nil)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	var resp map[string]string
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("stdout %q: %v", out, err)
	}
	if len(resp) != 1 {
		t.Fatalf("want only agent_message and no permission, got %s", out)
	}
	msg := resp["agent_message"]
	if !strings.Contains(msg, " compact --harness=cursor -- go test ./...") || !strings.Contains(msg, "Run it again exactly as:") {
		t.Errorf("agent_message = %q", msg)
	}
}

// Anything that is not a compressing family, and every fail-open case, is a
// bare allow. Cursor's answer is never deny.
func TestCursorShellAdvise_SilentAllowOtherwise(t *testing.T) {
	cases := map[string]struct {
		payload string
		env     map[string]string
	}{
		"not allowlisted": {payload: cursorShellExecPayload(t, "cat go.mod")},
		"small family":    {payload: cursorShellExecPayload(t, "git status")},
		"kill switch":     {payload: cursorShellExecPayload(t, "go test ./..."), env: map[string]string{"DOWNSHIFT_EXEC": "0"}},
		"already wrapped": {payload: cursorShellExecPayload(t, testExe+" compact -- go test ./...")},
		"garbage":         {payload: "not json"},
		"empty command":   {payload: cursorShellExecPayload(t, "")},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			rc, out := runCursorShellExec(t, c.payload, c.env)
			if rc != 0 || out != `{}` {
				t.Fatalf("rc=%d out=%q, want the neutral {}", rc, out)
			}
		})
	}
}

// When the preToolUse Shell rewrite was applied, the command Cursor then
// reports is already wrapped: beforeShellExecution stays silent, no double advice.
func TestCursorShellAdvise_NoAdviceAfterPreToolUseRewrite(t *testing.T) {
	_, pre := runCursorHook(t, cursorShellPayload(t, "Shell", "git status"))
	var resp struct {
		UpdatedInput struct {
			Command string `json:"command"`
		} `json:"updated_input"`
	}
	if err := json.Unmarshal([]byte(pre), &resp); err != nil || resp.UpdatedInput.Command == "" {
		t.Fatalf("preToolUse did not rewrite: %q (%v)", pre, err)
	}
	rc, out := runCursorShellExec(t, cursorShellExecPayload(t, resp.UpdatedInput.Command), nil)
	if rc != 0 || out != `{}` {
		t.Fatalf("rc=%d out=%q, want the neutral {} for the rewritten command", rc, out)
	}
}
