// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package skills

import (
	"regexp"
	"strings"
	"testing"
)

func effortText(t *testing.T) string {
	t.Helper()
	b, err := FS.ReadFile(Effort + "/SKILL.md")
	if err != nil {
		t.Fatalf("effort skill is not embedded: %v", err)
	}
	return string(b)
}

func TestEffortSkillHasThreeStakeLevels(t *testing.T) {
	text := effortText(t)
	for _, want := range []string{
		"name: downshift-effort",
		"Mechanical and reversible",
		"Ordinary code change with tests",
		"High stakes",
		"auth, security, money, production data",
		"check it directly before you report",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("effort skill lacks %q", want)
		}
	}
}

func TestEffortSkillStatesO8(t *testing.T) {
	text := effortText(t)
	if !strings.Contains(text, "It never sets an effort value") || !strings.Contains(text, "O8") {
		t.Error("effort skill must say it never sets an effort value (O8)")
	}
}

func TestEffortSkillHasNoDollarsOrModelIDs(t *testing.T) {
	text := effortText(t)
	if strings.Contains(text, "$") {
		t.Error("effort skill must keep accounting in tokens, found a dollar sign")
	}
	ids := regexp.MustCompile(`(?i)claude-|gpt-|gemini-|\bopus\b|\bsonnet\b|\bhaiku\b`)
	if m := ids.FindString(text); m != "" {
		t.Errorf("effort skill names a model id: %q", m)
	}
	if n := strings.Count(text, "\n"); n >= 80 {
		t.Errorf("effort skill has %d lines, want under 80", n)
	}
}
