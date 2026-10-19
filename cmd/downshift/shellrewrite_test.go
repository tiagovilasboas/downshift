// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// withExec pins the three inputs of shellWrapDecision for one test: the
// config flag, the kill-switch env and whether downshift is on PATH.
func withExec(t *testing.T, enabled bool, env map[string]string, onPath bool) {
	t.Helper()
	oldEnabled, oldLook, oldEnv := execConfigEnabled, execLookPath, hookGetenv
	execConfigEnabled = func() bool { return enabled }
	execLookPath = func(string) (string, error) {
		if onPath {
			return "/usr/local/bin/downshift", nil
		}
		return "", errors.New("not found")
	}
	hookGetenv = func(k string) string { return env[k] }
	t.Cleanup(func() { execConfigEnabled, execLookPath, hookGetenv = oldEnabled, oldLook, oldEnv })
}

func bashPayload(t *testing.T, command string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"session_id":      "s1",
		"tool_use_id":     "toolu_1",
		"tool_input": map[string]any{
			"command":           command,
			"description":       "Run the unit tests for the whole module",
			"timeout":           120000,
			"run_in_background": false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestShellRewrite_ThroughSharedRunner drives the real Claude Code hook entry
// (runHookAdapter with the claude-code adapter and wire).
func TestShellRewrite_ThroughSharedRunner(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		env     map[string]string
		onPath  bool
		command string
		want    string // rewritten command; "" means unchanged ({})
	}{
		{"enabled go test", true, nil, true, "go test ./...", "downshift exec -- go test ./..."},
		{"enabled git status", true, nil, true, "git status", "downshift exec -- git status"},
		{"disabled by default", false, nil, true, "go test ./...", ""},
		{"kill switch", true, map[string]string{"DOWNSHIFT_EXEC": "0"}, true, "go test ./...", ""},
		{"kill switch other value keeps it on", true, map[string]string{"DOWNSHIFT_EXEC": "1"}, true, "git status", "downshift exec -- git status"},
		{"metacharacter", true, nil, true, "go test ./... | tail -5", ""},
		{"list", true, nil, true, "git status && rm -rf x", ""},
		{"not allowlisted", true, nil, true, "git log", ""},
		{"already wrapped", true, nil, true, "downshift exec -- go test ./...", ""},
		{"downshift not on PATH", true, nil, false, "go test ./...", ""},
		{"empty command", true, nil, true, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			eventLog := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
			withExec(t, tc.enabled, tc.env, tc.onPath)

			var rc int
			out := captureStdout(func() {
				rc = harnessRegistry["claude-code"].Hook(strings.NewReader(bashPayload(t, tc.command)), cmdCat)
			})
			if rc != 0 {
				t.Fatalf("rc = %d, a shell call must never be denied", rc)
			}
			if tc.want == "" {
				if strings.TrimSpace(out) != "{}" {
					t.Fatalf("stdout = %q, want {}", out)
				}
			} else {
				assertShellRewrite(t, out, tc.want)
			}
			// A shell call is never a spawn: no routing decision is recorded,
			// even though tool_input carries a description.
			if _, err := os.Stat(eventLog); err == nil {
				data, _ := os.ReadFile(eventLog)
				t.Fatalf("shell call wrote routing telemetry: %s", data)
			}
		})
	}
}

func assertShellRewrite(t *testing.T, out, want string) {
	t.Helper()
	var resp struct {
		HookSpecificOutput map[string]json.RawMessage `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("stdout %q: %v", out, err)
	}
	h := resp.HookSpecificOutput
	if _, has := h["permissionDecision"]; has {
		t.Fatalf("shell rewrite must not carry a permissionDecision: %s", out)
	}
	if string(h["hookEventName"]) != `"PreToolUse"` {
		t.Fatalf("hookEventName = %s", h["hookEventName"])
	}
	var ti map[string]any
	if err := json.Unmarshal(h["updatedInput"], &ti); err != nil {
		t.Fatal(err)
	}
	if ti["command"] != want {
		t.Fatalf("command = %v, want %q", ti["command"], want)
	}
	if ti["description"] != "Run the unit tests for the whole module" || ti["timeout"] != float64(120000) || ti["run_in_background"] != false {
		t.Fatalf("sibling fields not preserved: %v", ti)
	}
	if len(ti) != 4 {
		t.Fatalf("updatedInput has extra or missing fields: %v", ti)
	}
}

// fakeShellEvent is a shell-capable payload for a harness that declares no
// shell rewrite: the decision is computed, nothing is written.
type fakeShellEvent struct{ Command string }

func (e fakeShellEvent) ShellCommand() (string, bool) { return e.Command, true }
func (e fakeShellEvent) ShellRewrite(c string) (any, error) {
	return map[string]string{"command": c}, nil
}

func TestShellRewrite_HarnessWithoutCapabilityIsUnchanged(t *testing.T) {
	withExec(t, true, nil, true)
	for _, n := range []string{"cursor", "codex", "kirocrew", "antigravity", "opencode", "unknown"} {
		d := shellWrapDecision(n, "go test ./...")
		if d.Rewrite || d.Reason != "harness_cannot_rewrite" || d.Family != execwrap.FamilyGoTest {
			t.Fatalf("%s: decision = %+v", n, d)
		}
	}
	out := captureStdout(func() {
		rc := runHookAdapter("cursor", strings.NewReader(`{"Command":"go test ./..."}`),
			func(b []byte) (fakeShellEvent, error) { var e fakeShellEvent; return e, json.Unmarshal(b, &e) },
			func(fakeShellEvent) (any, string, core.Decision) {
				t.Fatal("a shell call must not be routed")
				return nil, "", core.Decision{}
			},
			printCursorAllow)
		if rc != 0 {
			t.Fatalf("rc = %d", rc)
		}
	})
	if strings.TrimSpace(out) != `{"permission":"allow"}` {
		t.Fatalf("stdout = %q, want the harness fail-open answer", out)
	}
}

// Claude Code is the only harness that declares the capability today; the
// fact is data, read through AllHarnessCaps.
func TestShellRewrite_CapabilityDeclaredOnlyForClaudeCode(t *testing.T) {
	withExec(t, true, nil, true)
	if d := shellWrapDecision("claude-code", "go test ./..."); !d.Rewrite {
		t.Fatalf("claude-code decision = %+v", d)
	}
}

func TestShellRewrite_ConfigFlag(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	if execConfigEnabled() {
		t.Fatal("exec must be off with no config file")
	}
	cfg := contextopt.DefaultConfig()
	cfg.Exec = true
	if err := contextopt.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if !execConfigEnabled() {
		t.Fatal("exec should be on after cfg.Exec = true")
	}
	cfg.Enabled = false
	if err := contextopt.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if execConfigEnabled() {
		t.Fatal("exec must be off while context optimization is disabled")
	}
}

func TestContextEnableExecFlag(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	var out, errb strings.Builder
	if rc := runContext([]string{"enable", "native", "--exec"}, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, errb.String())
	}
	cfg, _ := contextopt.LoadConfig()
	if !cfg.Exec || !cfg.Enabled {
		t.Fatalf("cfg = %+v", cfg)
	}
	if !strings.Contains(out.String(), `"matcher": "Bash"`) || !strings.Contains(out.String(), "Permission rules match the rewritten text") {
		t.Fatalf("enable --exec must print the hook entry and the permission note:\n%s", out.String())
	}
	if rc := runContext([]string{"enable", "--no-exec"}, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, errb.String())
	}
	if cfg, _ := contextopt.LoadConfig(); cfg.Exec {
		t.Fatal("--no-exec must turn it off")
	}
	runContext([]string{"enable", "--exec"}, &out, &errb)
	runContext([]string{"disable"}, &out, &errb)
	if cfg, _ := contextopt.LoadConfig(); cfg.Exec {
		t.Fatal("disable must turn exec off")
	}
	if rc := runContext([]string{"enable", "headroom", "--exec"}, &out, &errb); rc != 2 {
		t.Fatalf("--exec on a companion: rc=%d, want 2", rc)
	}
}
