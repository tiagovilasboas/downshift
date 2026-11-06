// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetupDryRunWritesNothing(t *testing.T) {
	e := newSetupEnv(t, ".claude", ".cursor")
	e.write(".claude/settings.json", `{"model":"opus"}`)
	code, out, _ := e.run("--dry-run")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if e.read(".claude/settings.json") != `{"model":"opus"}` {
		t.Error("existing file modified")
	}
	if _, err := os.Stat(e.path(".cursor/hooks.json")); err == nil {
		t.Error("new file created")
	}
	for _, want := range []string{"would change", "+++ b/~/.claude/settings.json", "\"matcher\": \"Agent|Task\"", "Dry run: nothing was written."} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if m, _ := filepath.Glob(e.path(".claude/*downshift*")); len(m) != 0 {
		t.Errorf("backup written in dry run: %v", m)
	}
}

func TestSetupUndoRestoresLatestBackup(t *testing.T) {
	e := newSetupEnv(t, ".claude", ".cursor")
	original := "{\n  \"model\": \"opus\"\n}\n"
	e.write(".claude/settings.json", original)
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if e.read(".claude/settings.json") == original {
		t.Fatal("setup changed nothing")
	}
	if m, _ := filepath.Glob(e.path(".claude/settings.json.downshift-bak-*")); len(m) != 1 {
		t.Fatalf("want one backup, got %v", m)
	}
	// A dry-run undo changes nothing.
	if code, _, _ := e.run("--undo", "--dry-run"); code != 0 || e.read(".claude/settings.json") == original {
		t.Fatal("dry-run undo modified the file")
	}
	code, out, _ := e.run("--undo", "--yes")
	if code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if e.read(".claude/settings.json") != original {
		t.Errorf("not restored:\n%s", e.read(".claude/settings.json"))
	}
	// The cursor file did not exist before setup: undo removes it.
	if _, err := os.Stat(e.path(".cursor/hooks.json")); err == nil {
		t.Error("file created by setup survived undo")
	}
}

func TestSetupUndoPicksNewestBackup(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.write(".claude/settings.json", `{"a":1}`)
	e.run("--yes")
	// Simulate the user changing the file, then running setup with another binary path.
	e.write(".claude/settings.json", `{"a":2}`)
	e.run("--yes")
	if code, _, _ := e.run("--undo", "--yes"); code != 0 {
		t.Fatal("undo failed")
	}
	if got := e.read(".claude/settings.json"); got != `{"a":2}` {
		t.Errorf("restored %q, want the newest backup", got)
	}
}

func TestSetupMalformedJSONAbortsWithoutWriting(t *testing.T) {
	e := newSetupEnv(t, ".claude", ".cursor")
	e.write(".claude/settings.json", `{"hooks": {`)
	code, out, errOut := e.run("--yes")
	if code == 0 {
		t.Fatalf("want non-zero exit\n%s", out)
	}
	if e.read(".claude/settings.json") != `{"hooks": {` {
		t.Error("malformed file was modified")
	}
	if m, _ := filepath.Glob(e.path(".claude/*downshift*")); len(m) != 0 {
		t.Errorf("backup or temp written: %v", m)
	}
	if !strings.Contains(errOut, "not valid JSON") {
		t.Errorf("stderr: %s", errOut)
	}
	// The healthy harness is still wired.
	if _, err := os.Stat(e.path(".cursor/hooks.json")); err != nil {
		t.Errorf("cursor not wired: %v", err)
	}
}

func TestSetupWrongShapeAborts(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.write(".claude/settings.json", `{"hooks":{"PreToolUse":"nope"}}`)
	if code, _, errOut := e.run("--yes"); code == 0 || !strings.Contains(errOut, "not a JSON array") {
		t.Fatalf("exit %d, stderr %s", code, errOut)
	}
	if e.read(".claude/settings.json") != `{"hooks":{"PreToolUse":"nope"}}` {
		t.Error("file modified")
	}
}

func TestSetupDeclinedPromptWritesNothing(t *testing.T) {
	e := newSetupEnv(t, ".codex")
	var o, er bytes.Buffer
	code := Run(Options{Home: e.home, Binary: testBin, SkipDiscovery: true, In: strings.NewReader("n\n"), Out: &o, Err: &er, Now: time.Now})
	if code == 0 {
		t.Error("declined prompt should not exit 0")
	}
	if _, err := os.Stat(e.path(".codex/hooks.json")); err == nil {
		t.Error("written without consent")
	}
}
