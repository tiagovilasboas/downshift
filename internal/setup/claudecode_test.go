// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"strings"
	"testing"
)

// Claude Code starts in plan mode with opusplan only for keys the user has not
// set; anything already set is kept and setup says how to switch.
func TestSetupClaudeCodeMainSessionPolicy(t *testing.T) {
	policy := func(e *setupEnv) (model any, mode any) {
		doc := parseDoc(t, e.read(".claude/settings.json"))
		perms, _ := doc["permissions"].(map[string]any)
		return doc["model"], perms["defaultMode"]
	}

	fresh := newSetupEnv(t, ".claude")
	if code, out, _ := fresh.run("claude-code", "--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if m, d := policy(fresh); m != "opusplan" || d != "plan" {
		t.Errorf("fresh = model %v, defaultMode %v; want opusplan, plan", m, d)
	}
	if _, ok := parseDoc(t, fresh.read(".claude/settings.json"))["autoCompactWindow"]; ok {
		t.Error("setup wrote autoCompactWindow; the owner keeps it outside setup")
	}

	pinned := newSetupEnv(t, ".claude")
	pinned.write(".claude/settings.json", `{"model":"claude-sonnet-5-5","autoCompactWindow":250000,"enabledPlugins":{"cc-plugin-you-should-know@builtin":false},"permissions":{"allow":["Bash(ls:*)"],"defaultMode":"acceptEdits"}}`)
	code, out, _ := pinned.run("claude-code", "--yes", "--you-should-know")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if m, d := policy(pinned); m != "claude-sonnet-5-5" || d != "acceptEdits" {
		t.Errorf("pinned = model %v, defaultMode %v; want both kept", m, d)
	}
	if !strings.Contains(out, `"model": "opusplan"`) || !strings.Contains(out, "permissions.defaultMode") || !strings.Contains(out, "cc-plugin-you-should-know@builtin") {
		t.Errorf("no notes for the kept values:\n%s", out)
	}
	if w := parseDoc(t, pinned.read(".claude/settings.json"))["autoCompactWindow"]; w != float64(250000) {
		t.Errorf("pinned autoCompactWindow = %v; want 250000 kept (setup never touches it)", w)
	}
	if perms := parseDoc(t, pinned.read(".claude/settings.json"))["permissions"].(map[string]any); len(perms["allow"].([]any)) != 1 {
		t.Errorf("existing permissions rules changed: %v", perms)
	}

	already := newSetupEnv(t, ".claude")
	already.write(".claude/settings.json", `{"model":"opusplan","enabledPlugins":{"cc-plugin-you-should-know@builtin":true},"permissions":{"defaultMode":"plan"}}`)
	if _, out, _ := already.run("claude-code", "--yes"); strings.Contains(out, "keeps its pinned") || strings.Contains(out, "keep their") {
		t.Errorf("note printed although the policy is already set:\n%s", out)
	}

	notObject := newSetupEnv(t, ".claude")
	notObject.write(".claude/settings.json", `{"permissions":"strict"}`)
	if code, out, errOut := notObject.run("claude-code", "--yes"); code == 0 || !strings.Contains(out+errOut, `"permissions" is not a JSON object`) {
		t.Errorf("non-object permissions: exit %d, want an error naming it\n%s%s", code, out, errOut)
	}
	if got := parseDoc(t, notObject.read(".claude/settings.json"))["permissions"]; got != "strict" {
		t.Errorf("non-object permissions overwritten: %v", got)
	}
}

// --no-session-policy wires the hooks and leaves the main-session policy alone.
func TestSetupNoSessionPolicySkipsThePolicy(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	if code, out, _ := e.run("claude-code", "--yes", "--no-session-policy"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	doc := parseDoc(t, e.read(".claude/settings.json"))
	if _, ok := doc["model"]; ok {
		t.Errorf("model written with --no-session-policy: %v", doc["model"])
	}
	if _, ok := doc["permissions"]; ok {
		t.Errorf("permissions written with --no-session-policy: %v", doc["permissions"])
	}
	if _, ok := doc["hooks"].(map[string]any)["PreToolUse"]; !ok {
		t.Errorf("hooks not wired: %v", doc["hooks"])
	}
}
