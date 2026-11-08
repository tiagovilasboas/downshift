// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

func TestTriggerTextNamesStrainCheckAndCap(t *testing.T) {
	skill, err := os.ReadFile("../../skills/downshift-orchestrator/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"agent-triggers.md": setup.AgentTriggers, "SKILL.md": string(skill)} {
		for _, want := range []string{"`providers`", "`strain`", "5 ", "Do not explore alone"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s lacks %q", name, want)
			}
		}
		if strings.Contains(body, "plan alone first") {
			t.Errorf("%s still forbids planning first", name)
		}
	}
}
