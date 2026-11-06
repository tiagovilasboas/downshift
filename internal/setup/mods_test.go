// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"strings"
	"testing"
)

const (
	agentsManifest = ".claude/skills/downshift-agents/.claude-plugin/plugin.json"
	agentsHooks    = ".claude/skills/downshift-agents/hooks/hooks.json"
)

func TestSetupInstallsTheAgentsModAsASkillsDirPlugin(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	if code, out, errOut := e.run("--yes", "claude-code"); code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if got := e.read(agentsManifest); !strings.Contains(got, `"name": "downshift-agents"`) {
		t.Fatalf("manifest not installed:\n%s", got)
	}
	if e.read(agentsHooks) == "" {
		t.Fatal("hooks.json not installed")
	}
	files, err := modFiles("downshift-agents")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if isModTestFile(f) || strings.Contains(f, ".claude-plugin/types") {
			t.Fatalf("a test or generated file would ship: %s", f)
		}
	}
	if code, out, _ := e.run("--yes", "claude-code"); code != 0 || !strings.Contains(out, "already up to date") {
		t.Fatalf("second run exit %d, want idempotent\n%s", code, out)
	}
}

func TestSetupNoModsInstallsNoMod(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	if code, out, errOut := e.run("--yes", "--no-mods", "claude-code"); code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if _, err := os.Stat(e.path(agentsManifest)); !os.IsNotExist(err) {
		t.Fatalf("--no-mods installed the mod: %v", err)
	}
}

func TestSetupModDryRunWritesNothing(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	if code, out, errOut := e.run("--dry-run", "claude-code"); code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	} else if !strings.Contains(out, "downshift-agents") {
		t.Fatalf("the dry run must show the mod:\n%s", out)
	}
	if _, err := os.Stat(e.path(agentsManifest)); !os.IsNotExist(err) {
		t.Fatalf("a dry run wrote the mod: %v", err)
	}
}

// The marketplace copy already loads the mod; a second copy would run it twice.
func TestSetupModLeavesTheMarketplaceCopy(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.write(".claude/settings.json", `{"enabledPlugins": {"downshift-agents@downshift": true}}`)
	code, out, errOut := e.run("--yes", "claude-code")
	if code != 0 || !strings.Contains(out, "already carries the mod") {
		t.Fatalf("exit %d, want a skip\n%s\n%s", code, out, errOut)
	}
	if _, err := os.Stat(e.path(agentsManifest)); !os.IsNotExist(err) {
		t.Fatalf("the mod was installed twice: %v", err)
	}
}

func TestSetupModRefusesAForeignFolder(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.write(".claude/skills/downshift-agents/notes.md", "mine\n")
	code, _, errOut := e.run("--yes", "claude-code")
	if code == 0 || !strings.Contains(errOut, "is not the downshift-agents mod") {
		t.Fatalf("exit %d, want a refusal\n%s", code, errOut)
	}
	if _, err := os.Stat(e.path(agentsManifest)); !os.IsNotExist(err) {
		t.Fatalf("setup wrote into a foreign folder: %v", err)
	}
}

func TestSetupModUndoRemovesTheFolder(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	if code, out, errOut := e.run("--yes", "claude-code"); code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if code, out, errOut := e.run("--undo", "--yes", "claude-code"); code != 0 {
		t.Fatalf("undo exit %d\n%s\n%s", code, out, errOut)
	}
	if _, err := os.Stat(e.path(".claude/skills/downshift-agents")); !os.IsNotExist(err) {
		t.Fatalf("undo left the mod folder: %v", err)
	}
}
