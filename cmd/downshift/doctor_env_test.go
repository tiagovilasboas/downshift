// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

func TestSubagentModelEnvNote(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	if got := subagentModelEnvNote(env(nil)); got != "" {
		t.Fatalf("unset must be silent, got %q", got)
	}
	if got := subagentModelEnvNote(env(map[string]string{"CLAUDE_CODE_SUBAGENT_MODEL": "  "})); got != "" {
		t.Fatalf("blank must be silent, got %q", got)
	}
	got := subagentModelEnvNote(env(map[string]string{"CLAUDE_CODE_SUBAGENT_MODEL": "haiku"}))
	if !strings.Contains(got, "CLAUDE_CODE_SUBAGENT_MODEL=haiku") || !strings.Contains(got, "not verified") {
		t.Fatalf("note = %q", got)
	}
	long := strings.Repeat("x", 200)
	got = subagentModelEnvNote(env(map[string]string{"CLAUDE_CODE_SUBAGENT_MODEL": long}))
	if strings.Contains(got, long) || !strings.Contains(got, "...") {
		t.Fatalf("long value must be truncated, got %q", got)
	}
}
