// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestModelOverrideNotes(t *testing.T) {
	caps := core.AllHarnessCaps()
	if got := modelOverrideNotes(caps, envOf(nil)); len(got) != 0 {
		t.Fatalf("unset must be silent, got %v", got)
	}
	if got := modelOverrideNotes(caps, envOf(map[string]string{"CLAUDE_CODE_SUBAGENT_MODEL": "  "})); len(got) != 0 {
		t.Fatalf("blank must be silent, got %v", got)
	}
	got := modelOverrideNotes(caps, envOf(map[string]string{"CLAUDE_CODE_SUBAGENT_MODEL": "haiku"}))
	if len(got) != 1 || !strings.Contains(got[0], "CLAUDE_CODE_SUBAGENT_MODEL=haiku") || !strings.Contains(got[0], "claude-code") || !strings.Contains(got[0], "not verified") {
		t.Fatalf("notes = %v", got)
	}
	long := strings.Repeat("x", 200)
	got = modelOverrideNotes(caps, envOf(map[string]string{"CLAUDE_CODE_SUBAGENT_MODEL": long}))
	if len(got) != 1 || strings.Contains(got[0], long) || !strings.Contains(got[0], "...") {
		t.Fatalf("long value must be truncated, got %v", got)
	}
}

// A harness that declares a variable is reported without touching doctor.
func TestModelOverrideNotesAreDrivenByCapabilities(t *testing.T) {
	caps := []core.NamedCaps{
		{Harness: "future-harness", Caps: core.HarnessCapabilities{ModelOverrideEnv: "FUTURE_MODEL"}},
		{Harness: "quiet", Caps: core.HarnessCapabilities{}},
	}
	got := modelOverrideNotes(caps, envOf(map[string]string{"FUTURE_MODEL": "m1"}))
	if len(got) != 1 || !strings.Contains(got[0], "future-harness") {
		t.Fatalf("notes = %v", got)
	}
}

func TestAllHarnessCapsCoversEveryAdapter(t *testing.T) {
	want := map[string]bool{"claude-code": true, "cursor": true, "codex": true, "kirocrew": true, "antigravity": true}
	for _, c := range core.AllHarnessCaps() {
		delete(want, c.Harness)
	}
	if len(want) != 0 {
		t.Fatalf("harnesses missing from AllHarnessCaps: %v", want)
	}
}
