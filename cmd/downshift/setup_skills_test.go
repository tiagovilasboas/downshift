// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const auditSkill = "downshift-audit/SKILL.md"

func TestSetupInstallsTheAuditSkillForClaudeCodeAndCodex(t *testing.T) {
	e := newSetupEnv(t, ".claude", ".codex")
	if code, out, _ := e.run("--yes", "claude-code", "codex"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	for _, dir := range []string{".claude/skills", ".codex/skills"} {
		got, err := os.ReadFile(e.path(dir + "/" + auditSkill))
		if err != nil || !setupOwns(got) || !strings.HasPrefix(string(got), "---\n") {
			t.Fatalf("%s: skill missing, unmarked or frontmatter displaced: %v\n%.200s", dir, err, got)
		}
		if _, err := os.Stat(e.path(dir + "/downshift-audit/references/interpretation.md")); err != nil {
			t.Errorf("%s: reference file not installed: %v", dir, err)
		}
	}
	if code, out, _ := e.run("--yes", "claude-code", "codex"); code != 0 || !strings.Contains(out, "already up to date") {
		t.Fatalf("second run exit %d, want idempotent\n%s", code, out)
	}
}

func TestSetupDryRunWritesNoSkill(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	if code, out, _ := e.run("--dry-run", "claude-code"); code != 0 {
		t.Fatalf("dry run exit %d\n%s", code, out)
	}
	if _, err := os.Stat(e.path(".claude/skills")); !os.IsNotExist(err) {
		t.Fatalf("dry run created the skills folder: %v", err)
	}
}

// A skill folder a person made is not ours: setup writes nothing into it.
func TestSetupRefusesAForeignSkillFolder(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	foreign := e.path(".claude/skills/" + auditSkill)
	if err := os.MkdirAll(filepath.Dir(foreign), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := e.run("--yes", "claude-code")
	if code == 0 || !strings.Contains(errOut, "was not installed by downshift setup") {
		t.Fatalf("setup exit %d, want a refusal\n%s", code, errOut)
	}
	if got, _ := os.ReadFile(foreign); string(got) != "mine\n" {
		t.Fatalf("setup wrote into a foreign skill: %q", got)
	}
}

func TestSetupUndoRemovesTheSkillAndItsFolders(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	if code, out, _ := e.run("--yes", "claude-code"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	if code, out, _ := e.run("--undo", "--yes"); code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if _, err := os.Stat(e.path(".claude/skills/downshift-audit")); !os.IsNotExist(err) {
		t.Fatalf("skill folder left behind: %v", err)
	}
	// Reinstalling after an undo must not see a leftover folder as foreign.
	if code, out, _ := e.run("--yes", "claude-code"); code != 0 {
		t.Fatalf("reinstall exit %d\n%s", code, out)
	}
}

func TestSetupUndoLeavesAnEditedSkill(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.run("--yes", "claude-code")
	path := e.path(".claude/skills/" + auditSkill)
	if err := os.WriteFile(path, []byte("rewritten by me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := e.run("--undo", "--yes"); code != 0 || !strings.Contains(out, "edited after setup") {
		t.Fatalf("undo exit %d, want a skip note\n%s", code, out)
	}
	if got, _ := os.ReadFile(path); string(got) != "rewritten by me\n" {
		t.Fatalf("undo removed an edited skill: %q", got)
	}
}
