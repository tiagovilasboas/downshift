// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckSecurityAuditSkill(t *testing.T) {
	home := t.TempDir()
	// Claude Code has the skill, Codex has a config dir but no skill, and no
	// other harness directory exists, so only those two rows are printed.
	writeFile(t, filepath.Join(home, ".claude", "skills", "security-audit", "SKILL.md"), "---\nname: security-audit\n---\n")
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}

	rep := &doctorReport{}
	checkSecurityAuditSkill(rep, doctorEnv{Home: home})

	got := map[string]doctorCheck{}
	for _, c := range rep.Checks {
		if c.ID != "security skill" {
			t.Fatalf("unexpected check %+v", c)
		}
		got[c.Harness] = c
	}
	if len(got) != 2 {
		t.Fatalf("want rows for claude-code and codex, got %+v", rep.Checks)
	}
	claude := got["claude-code"]
	wantStatus(t, claude, doctorOK)
	if !strings.HasPrefix(claude.Detail, "found") {
		t.Errorf("claude-code: %q", claude.Detail)
	}
	codex := got["codex"]
	wantStatus(t, codex, doctorOK)
	if !strings.Contains(codex.Detail, "not installed: npx skills add") {
		t.Errorf("codex: %q", codex.Detail)
	}
}

func TestCheckSecurityAuditSkillNoHome(t *testing.T) {
	rep := &doctorReport{}
	checkSecurityAuditSkill(rep, doctorEnv{})
	if len(rep.Checks) != 0 {
		t.Fatalf("want no rows without a home, got %+v", rep.Checks)
	}
}
