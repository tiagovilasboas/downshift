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
