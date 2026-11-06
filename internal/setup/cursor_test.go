// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"strings"
	"testing"
)

// hasMatcher reports whether event carries an entry with the given matcher.
func hasMatcher(doc map[string]any, event, matcher string) bool {
	for _, e := range doc["hooks"].(map[string]any)[event].([]any) {
		if e.(map[string]any)["matcher"] == matcher {
			return true
		}
	}
	return false
}

func TestSetupCursorShellHook(t *testing.T) {
	e := newSetupEnv(t, ".cursor")
	code, out, _ := e.run("--dry-run")
	if code != 0 || !strings.Contains(out, `"matcher": "Shell"`) {
		t.Fatalf("dry-run exit %d, want the Shell entry\n%s", code, out)
	}
	if _, err := os.Stat(e.path(".cursor/hooks.json")); err == nil {
		t.Fatal("dry-run wrote the file")
	}
	for i := 0; i < 2; i++ { // second run must be idempotent
		if code, out, _ := e.run("--yes"); code != 0 {
			t.Fatalf("exit %d\n%s", code, out)
		}
	}
	doc := parseDoc(t, e.read(".cursor/hooks.json"))
	var shell, task int
	for _, h := range doc["hooks"].(map[string]any)["preToolUse"].([]any) {
		m := h.(map[string]any)
		if m["command"] != testBin+" cursor" {
			continue
		}
		switch m["matcher"] {
		case "Shell":
			shell++
		case "Task":
			task++
		}
	}
	if shell != 1 || task != 1 {
		t.Errorf("Shell entries %d, Task entries %d, want 1 each: %s", shell, task, e.read(".cursor/hooks.json"))
	}
}

// The cursor setup writes one beforeShellExecution entry running `downshift cursor`
// (shell advise fallback), idempotently, with matcher "*" (every shell command).
func TestSetupCursorShellExecHook(t *testing.T) {
	e := newSetupEnv(t, ".cursor")
	if code, out, _ := e.run("--dry-run"); code != 0 || !strings.Contains(out, "beforeShellExecution") {
		t.Fatalf("dry-run exit %d, want the beforeShellExecution entry\n%s", code, out)
	}
	for i := 0; i < 2; i++ {
		if code, out, _ := e.run("--yes"); code != 0 {
			t.Fatalf("exit %d\n%s", code, out)
		}
	}
	doc := parseDoc(t, e.read(".cursor/hooks.json"))
	entries, _ := doc["hooks"].(map[string]any)["beforeShellExecution"].([]any)
	if len(entries) != 1 {
		t.Fatalf("beforeShellExecution entries = %d, want 1: %s", len(entries), e.read(".cursor/hooks.json"))
	}
	m := entries[0].(map[string]any)
	if m["command"] != testBin+" cursor" || m["matcher"] != "*" {
		t.Errorf("entry = %v, want %q with matcher *", m, testBin+" cursor")
	}
}

func TestSetupCursorTouchHooks(t *testing.T) {
	e := newSetupEnv(t, ".cursor")
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	doc := parseDoc(t, e.read(".cursor/hooks.json"))
	hooks := doc["hooks"].(map[string]any)
	want := testBin + " cursor-touch"
	for _, ev := range []struct {
		event   string
		matcher string
	}{
		{event: "sessionStart", matcher: ""},
		{event: "beforeSubmitPrompt", matcher: "UserPromptSubmit"},
	} {
		list, _ := hooks[ev.event].([]any)
		var found bool
		for _, h := range list {
			m := h.(map[string]any)
			if m["command"] != want {
				continue
			}
			if ev.matcher == "" || m["matcher"] == ev.matcher {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: missing %q (matcher %q): %s", ev.event, want, ev.matcher, e.read(".cursor/hooks.json"))
		}
	}
}
