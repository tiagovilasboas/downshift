// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func codexBashPayload(t *testing.T, tool, command string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       tool,
		"session_id":      "s1",
		"tool_use_id":     "call_1",
		"cwd":             t.TempDir(),
		"model":           "gpt-5.6-sol",
		"tool_input": map[string]any{
			"command":    command,
			"workdir":    "/work",
			"timeout_ms": 120000,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func runCodexHook(t *testing.T, payload string) (int, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	withExec(t, true, nil, true)
	var rc int
	out := captureStdout(func() {
		rc = harnessRegistry["codex"].Hook(strings.NewReader(payload), cmdCat)
	})
	return rc, out
}

func TestCodexShellRewrite_ThroughSharedRunner(t *testing.T) {
	want := "'" + testExe + "' compact --session=" + testSession + " -- go test ./..."
	rc, out := runCodexHook(t, codexBashPayload(t, "Bash", "go test ./..."))
	if rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	var resp struct {
		HookSpecificOutput map[string]json.RawMessage `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("stdout %q: %v", out, err)
	}
	h := resp.HookSpecificOutput
	if string(h["hookEventName"]) != `"PreToolUse"` || string(h["permissionDecision"]) != `"allow"` {
		t.Fatalf("wrong envelope: %s", out)
	}
	var ti map[string]any
	if err := json.Unmarshal(h["updatedInput"], &ti); err != nil {
		t.Fatal(err)
	}
	if ti["command"] != want {
		t.Fatalf("command = %v, want %q", ti["command"], want)
	}
	if ti["workdir"] != "/work" || ti["timeout_ms"] != float64(120000) || len(ti) != 3 {
		t.Fatalf("sibling fields not preserved: %v", ti)
	}
}

func TestCodexShellUnchanged(t *testing.T) {
	for name, payload := range map[string]string{
		"not allowlisted": codexBashPayload(t, "Bash", "ls"),
		"metacharacter":   codexBashPayload(t, "Bash", "go test ./... | grep FAIL"),
		"other tool":      codexBashPayload(t, "apply_patch", "go test ./..."),
		"garbage":         "not json",
		"no tool_input":   `{"hook_event_name":"PreToolUse","tool_name":"Bash"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rc, out := runCodexHook(t, payload)
			if rc != 0 {
				t.Fatalf("rc = %d, must fail open", rc)
			}
			if strings.Contains(out, "permissionDecision") || strings.Contains(out, "updatedInput") {
				t.Fatalf("unchanged call must not carry a decision or rewrite: %s", out)
			}
		})
	}
}

func TestCodexSetupAddsBashHookIdempotently(t *testing.T) {
	e := newSetupEnv(t, ".codex")
	if code, out, _ := e.run("--dry-run"); code != 0 || !strings.Contains(out, "^Bash$") {
		t.Fatalf("dry-run does not show the Bash hook (exit %d): %s", code, out)
	}
	if _, err := os.Stat(e.path(".codex/hooks.json")); err == nil {
		t.Fatal("dry-run wrote hooks.json")
	}
	for i := 0; i < 2; i++ {
		if code, _, errOut := e.run("--yes"); code != 0 {
			t.Fatalf("setup #%d failed: %s", i+1, errOut)
		}
	}
	raw := e.read(".codex/hooks.json")
	if n := strings.Count(raw, "^Bash$"); n != 1 {
		t.Fatalf("Bash matcher appears %d times, want 1:\n%s", n, raw)
	}
	if got := hookCommands(t, parseDoc(t, raw), "PreToolUse"); len(got) != 2 {
		t.Fatalf("PreToolUse commands = %v", got)
	}
}
