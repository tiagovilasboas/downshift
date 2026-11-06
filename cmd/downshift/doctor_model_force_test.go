// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// findCheck returns the row for harness and id, or false when doctor did not add one.
func findCheck(rep *doctorReport, harness, id string) (doctorCheck, bool) {
	for _, c := range rep.Checks {
		if c.Harness == harness && c.ID == id {
			return c, true
		}
	}
	return doctorCheck{}, false
}

// A forced subagent model makes Claude Code ignore both the per-call model and
// the hook's rewrite (observed 2026-10-29), so doctor must fail on it.
func TestDoctor_SubagentModelForceInSettingsFails(t *testing.T) {
	f := newDoctorFixture(t)
	settings := strings.Replace(claudeSettings(f.bin), `{"hooks":`, `{"env":{"CLAUDE_CODE_SUBAGENT_MODEL_FORCE":"1"},"hooks":`, 1)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), settings)

	rep := buildDoctorReport(f.env)
	c := f.check(t, rep, "claude-code", "subagent model")
	wantStatus(t, c, doctorFail)
	if !strings.Contains(c.Detail, "CLAUDE_CODE_SUBAGENT_MODEL_FORCE") || !strings.Contains(c.Detail, "settings.json") {
		t.Errorf("detail %q should name the variable and the file", c.Detail)
	}
	if !strings.Contains(c.Fix, "remove CLAUDE_CODE_SUBAGENT_MODEL_FORCE") {
		t.Errorf("fix %q should say to remove the variable", c.Fix)
	}
}

func TestDoctor_SubagentModelForceInEnvironmentFails(t *testing.T) {
	f := newDoctorFixture(t)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))
	f.env.Getenv = func(k string) string {
		if k == "CLAUDE_CODE_SUBAGENT_MODEL_FORCE" {
			return "true"
		}
		return ""
	}
	c := f.check(t, buildDoctorReport(f.env), "claude-code", "subagent model")
	wantStatus(t, c, doctorFail)
	if !strings.Contains(c.Detail, "environment") {
		t.Errorf("detail %q should say the variable comes from the environment", c.Detail)
	}
}

func TestDoctor_NoSubagentModelForceAddsNoRow(t *testing.T) {
	f := newDoctorFixture(t)
	settings := strings.Replace(claudeSettings(f.bin), `{"hooks":`, `{"env":{"CLAUDE_CODE_SUBAGENT_MODEL_FORCE":"0"},"hooks":`, 1)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), settings)
	if c, ok := findCheck(buildDoctorReport(f.env), "claude-code", "subagent model"); ok {
		t.Errorf("a value of 0 is off, but doctor reported %+v", c)
	}
}
