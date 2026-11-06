// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package skills

import (
	"strings"
	"testing"
)

// The orchestrator skill must name the four MCP tools and the shell fallback,
// or the lead session never learns to call them.
func TestOrchestratorSkillNamesTheMCPTools(t *testing.T) {
	b, err := FS.ReadFile(Orchestrator + "/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{"`providers`", "`apply`", "`playbook`", "`status`", "downshift models available", "[tier=small]"} {
		if !strings.Contains(text, want) {
			t.Errorf("orchestrator skill lacks %s", want)
		}
	}
	if strings.Contains(text, "$") {
		t.Errorf("orchestrator skill must keep accounting in tokens, found a dollar sign")
	}
}

// Harnesses pick a skill from its description alone; a `triggers:` key is
// ignored. The description must name the activation phrases users type, both
// spaced and hyphenated, with and without accents.
func TestOrchestratorDescriptionNamesTheTriggers(t *testing.T) {
	desc := skillDescription(t, Orchestrator)
	for _, want := range []string{
		"downshift full power", "full-power",
		"potência total", "potencia-total",
		"força total", "forca-total",
		"downshift full force", "full-force",
		"downshift shift <squad>",
	} {
		if !strings.Contains(desc, want) {
			t.Errorf("orchestrator description lacks %q", want)
		}
	}
}

// The squad skill points full power at the orchestrator, its single owner.
func TestSquadDescriptionPointsFullPowerToOrchestrator(t *testing.T) {
	desc := skillDescription(t, Squad)
	for _, want := range []string{"downshift full power", Orchestrator} {
		if !strings.Contains(desc, want) {
			t.Errorf("squad description lacks %q", want)
		}
	}
}

func skillDescription(t *testing.T, skill string) string {
	t.Helper()
	b, err := FS.ReadFile(skill + "/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if rest, ok := strings.CutPrefix(line, "description: "); ok {
			return rest
		}
	}
	t.Fatalf("%s has no description line", skill)
	return ""
}
