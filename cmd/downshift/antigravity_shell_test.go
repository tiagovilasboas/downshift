// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

func antigravityShellPayload(t *testing.T, tool, command string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"toolCall": map[string]any{
			"name": tool,
			"args": map[string]any{"CommandLine": command, "Cwd": "/work", "SafeToAutoRun": true},
		},
		"workspacePaths": []string{t.TempDir()},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func runAntigravityHook(t *testing.T, payload string) (int, string) {
	t.Helper()
	testenv.SetHome(t, t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	withExec(t, true, nil, true)
	var rc int
	out := captureStdout(func() {
		rc = harnessRegistry["antigravity"].Hook(strings.NewReader(payload), cmdCat)
	})
	return rc, out
}

func TestAntigravityShellRewrite_ReadOnlyCommand(t *testing.T) {
	rc, out := runAntigravityHook(t, antigravityShellPayload(t, "run_command", "git status"))
	if rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	var resp struct {
		Decision  string            `json:"decision"`
		Overwrite map[string]string `json:"overwrite"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("stdout %q: %v", out, err)
	}
	want := "'" + testExe + "' compact -- git status"
	if resp.Decision != "allow" || resp.Overwrite["CommandLine"] != want || len(resp.Overwrite) != 1 {
		t.Fatalf("answer = %s, want allow + overwrite.CommandLine %q only", out, want)
	}
}

func TestAntigravityShellUnchanged(t *testing.T) {
	for name, payload := range map[string]string{
		"not read-only":   antigravityShellPayload(t, "run_command", "go test ./..."),
		"chain rider":     antigravityShellPayload(t, "run_command", "git status ; perl x.pl"),
		"other tool":      antigravityShellPayload(t, "view_file", "git status"),
		"empty command":   antigravityShellPayload(t, "run_command", ""),
		"already wrapped": antigravityShellPayload(t, "run_command", testExe+" compact -- git status"),
	} {
		rc, out := runAntigravityHook(t, payload)
		if rc != 0 || strings.TrimSpace(out) != `{"decision":"allow"}` {
			t.Errorf("%s: rc %d, stdout %q, want a plain allow", name, rc, out)
		}
	}
}

func TestAntigravityShellMalformedFailsOpen(t *testing.T) {
	for name, payload := range map[string]string{
		"not json":        `{"toolCall":`,
		"args not object": `{"toolCall":{"name":"run_command","args":"git status"}}`,
		"no args":         `{"toolCall":{"name":"run_command"}}`,
	} {
		rc, out := runAntigravityHook(t, payload)
		if rc != 0 || strings.TrimSpace(out) != `{"decision":"allow"}` {
			t.Errorf("%s: rc %d, stdout %q, want a plain allow", name, rc, out)
		}
	}
}
