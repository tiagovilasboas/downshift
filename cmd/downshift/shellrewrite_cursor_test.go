// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
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
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", t.TempDir()+"/events.jsonl")
	withExec(t, true, nil, true)
	var rc int
	out := captureStdout(func() {
		rc = harnessRegistry["cursor"].Hook(strings.NewReader(payload), cmdCat)
	})
	return rc, strings.TrimSpace(out)
}

func TestCursorShellRewrite_WrapsAndKeepsSiblings(t *testing.T) {
	rc, out := runCursorHook(t, cursorShellPayload(t, "Shell", "go test ./..."))
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
	if !strings.HasPrefix(cmd, "'"+testExe+"' compact") || !strings.HasSuffix(cmd, "-- go test ./...") {
		t.Fatalf("command = %q", cmd)
	}
	if ti["description"] != "run the tests" || ti["timeout"] != float64(60) || len(ti) != 3 {
		t.Fatalf("siblings not preserved: %v", ti)
	}
}

func TestCursorShellRewrite_UnchangedAndFailOpen(t *testing.T) {
	cases := map[string]string{
		"not allowlisted": cursorShellPayload(t, "Shell", "ls"),
		"other tool":      cursorShellPayload(t, "Read", "go test ./..."),
		"garbage":         "not json",
		"shell no object": `{"tool_name":"Shell","tool_input":"go test"}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			rc, out := runCursorHook(t, payload)
			if rc != 0 || out != `{"permission":"allow"}` {
				t.Fatalf("rc=%d out=%q, want unchanged allow", rc, out)
			}
		})
	}
}
