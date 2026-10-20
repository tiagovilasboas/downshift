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

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// withExec pins the three inputs of shellWrapDecision for one test: the
// config flag, the kill-switch env and whether downshift is on PATH.
func withExec(t *testing.T, enabled bool, env map[string]string, onPath bool) {
	t.Helper()
	oldEnabled, oldExe, oldEnv := execConfigEnabled, execExecutable, hookGetenv
	execConfigEnabled = func() bool { return enabled }
	execExecutable = func() (string, error) {
		if onPath {
			return testExe, nil
		}
		return "", errors.New("not executable")
	}
	hookGetenv = func(k string) string { return env[k] }
	t.Cleanup(func() { execConfigEnabled, execExecutable, hookGetenv = oldEnabled, oldExe, oldEnv })
}

// userSettings points CLAUDE_CONFIG_DIR at a temp dir whose settings.json
// carries the given permission rules, and hides the machine's managed policy.
// It returns a cwd with no settings of its own.
func userSettings(t *testing.T, allow, deny, ask []string) string {
	t.Helper()
	dir := t.TempDir()
	body, err := json.Marshal(map[string]any{"permissions": map[string]any{"allow": allow, "deny": deny, "ask": ask}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	t.Cleanup(claudecode.WithManagedSettings(nil))
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o700); err != nil { // stop the settings walk here
		t.Fatal(err)
	}
	return cwd
}

const testExe = "/opt/down shift/bin/downshift"

// testSession is the hashed id of the session_id "s1" that bashPayload carries.
var testSession = telemetry.HashSessionID("s1")

func bashPayload(t *testing.T, command, cwd string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"session_id":      "s1",
		"tool_use_id":     "toolu_1",
		"cwd":             cwd,
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
var okRules = []string{"Bash(go test:*)", "Bash(git status)"}

func TestShellRewrite_ThroughSharedRunner(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		env     map[string]string
		onPath  bool
		command string
		allow   []string
		deny    []string
		ask     []string
		want    string // rewritten command; "" means unchanged ({})
	}{
		{"enabled go test", true, nil, true, "go test ./...", okRules, nil, nil, "'" + testExe + "' compact --session=" + testSession + " -- go test ./..."},
		{"enabled git status", true, nil, true, "git status", okRules, nil, nil, "'" + testExe + "' compact --session=" + testSession + " -- git status"},
		{"no allow rule", true, nil, true, "go test ./...", nil, nil, nil, ""},
		{"allow rule for another command", true, nil, true, "go test ./...", []string{"Bash(git status)"}, nil, nil, ""},
		{"deny rule wins", true, nil, true, "go test ./...", okRules, []string{"Bash(go test:*)"}, nil, ""},
		{"ask rule wins", true, nil, true, "go test ./...", okRules, nil, []string{"Bash(go test ./...)"}, ""},
		{"bare Bash deny", true, nil, true, "git status", okRules, []string{"Bash"}, nil, ""},
		{"disabled by default", false, nil, true, "go test ./...", okRules, nil, nil, ""},
		{"kill switch", true, map[string]string{"DOWNSHIFT_EXEC": "0"}, true, "go test ./...", okRules, nil, nil, ""},
		{"kill switch other value keeps it on", true, map[string]string{"DOWNSHIFT_EXEC": "1"}, true, "git status", okRules, nil, nil, "'" + testExe + "' compact --session=" + testSession + " -- git status"},
		{"metacharacter", true, nil, true, "go test ./... | tail -5", okRules, nil, nil, ""},
		{"list", true, nil, true, "git status && rm -rf x", okRules, nil, nil, ""},
		{"not allowlisted", true, nil, true, "git log", okRules, nil, nil, ""},
		{"already wrapped", true, nil, true, "downshift exec -- go test ./...", okRules, nil, nil, ""},
		{"binary not executable", true, nil, false, "go test ./...", okRules, nil, nil, ""},
		{"empty command", true, nil, true, "", okRules, nil, nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			eventLog := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
			withExec(t, tc.enabled, tc.env, tc.onPath)
			cwd := userSettings(t, tc.allow, tc.deny, tc.ask)

			var rc int
			out := captureStdout(func() {
				rc = harnessRegistry["claude-code"].Hook(strings.NewReader(bashPayload(t, tc.command, cwd)), cmdCat)
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
func (fakeShellEvent) ShellAllowed(string) bool       { return true }
func (e fakeShellEvent) ShellRewrite(c string) (any, error) {
	return map[string]string{"command": c}, nil
}

func TestShellRewrite_HarnessWithoutCapabilityIsUnchanged(t *testing.T) {
	withExec(t, true, nil, true)
	for _, n := range []string{"cursor", "codex", "kirocrew", "antigravity", "opencode", "unknown"} {
		d := shellWrapDecision(n, "go test ./...", func(string) bool { return true })
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
	if d := shellWrapDecision("claude-code", "go test ./...", func(string) bool { return true }); !d.Rewrite {
		t.Fatalf("claude-code decision = %+v", d)
	}
}

func TestShellRewrite_CompactIsOnUnlessFileSaysFalse(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	if !execConfigEnabled() {
		t.Fatal("compact must be on with no config file")
	}
	cfg := contextopt.DefaultConfig()
	off := false
	cfg.Exec = &off
	if err := contextopt.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if execConfigEnabled() {
		t.Fatal(`"exec": false in the config must turn compact off`)
	}
}

func TestContextDisableRefusesNative(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	var out, errb strings.Builder
	if rc := runContext([]string{"disable"}, &out, &errb); rc != 2 {
		t.Fatalf("disable with no companion: rc=%d, want 2", rc)
	}
	if !strings.Contains(errb.String(), "always on") {
		t.Fatalf("refusal must say compact is always on:\n%s", errb.String())
	}
}

// Any settings read or parse error means no rewrite, in any scope.
func TestShellRewrite_UnreadableSettingsMeanNoRewrite(t *testing.T) {
	withExec(t, true, nil, true)
	cwd := userSettings(t, okRules, nil, nil)
	if err := os.MkdirAll(filepath.Join(cwd, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".claude", "settings.local.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(func() {
		harnessRegistry["claude-code"].Hook(strings.NewReader(bashPayload(t, "go test ./...", cwd)), cmdCat)
	})
	if strings.TrimSpace(out) != "{}" {
		t.Fatalf("stdout = %q, want {}", out)
	}
}

// A deny rule in the project or local scope, or a managed deny, blocks the
// rewrite even when the user scope allows the command.
func TestShellRewrite_DenyInAnyScope(t *testing.T) {
	withExec(t, true, nil, true)
	scopes := map[string]func(t *testing.T, cwd string) string{
		"project": func(t *testing.T, cwd string) string { return filepath.Join(cwd, ".claude", "settings.json") },
		"local":   func(t *testing.T, cwd string) string { return filepath.Join(cwd, ".claude", "settings.local.json") },
		"managed": func(t *testing.T, cwd string) string { return filepath.Join(t.TempDir(), "managed-settings.json") },
	}
	for name, path := range scopes {
		t.Run(name, func(t *testing.T) {
			cwd := userSettings(t, okRules, nil, nil)
			file := path(t, cwd)
			if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte(`{"permissions":{"deny":["Bash(go test:*)"]}}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if name == "managed" {
				t.Cleanup(claudecode.WithManagedSettings([]string{file}))
			}
			out := captureStdout(func() {
				harnessRegistry["claude-code"].Hook(strings.NewReader(bashPayload(t, "go test ./...", cwd)), cmdCat)
			})
			if strings.TrimSpace(out) != "{}" {
				t.Fatalf("stdout = %q, want {}", out)
			}
		})
	}
}
