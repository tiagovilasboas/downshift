// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// doctor prints, per detected harness, whether the end of a child is
// measurable: information, never a failure.
func TestDoctor_CompletionMeasurablePerHarness(t *testing.T) {
	f := newDoctorFixture(t)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))
	writeFile(t, filepath.Join(f.home, ".codex", "hooks.json"),
		fmt.Sprintf(`{"hooks":{"PreToolUse":[{"matcher":"x","hooks":[{"type":"command","command":"%s codex"}]}]}}`, f.bin))
	ts := f.started.Add(-time.Hour).Format(time.RFC3339Nano)
	writeFile(t, filepath.Join(f.home, ".downshift", "events.jsonl"),
		fmt.Sprintf(`{"correlation_id":"u1","timestamp":%q,"harness":"claude-code","outcome":"usage","final_model":"sonnet","input_tokens":10}`, ts)+"\n")

	rep := buildDoctorReport(f.env)
	claude := f.check(t, rep, "claude-code", "completion")
	wantStatus(t, claude, doctorOK)
	if !strings.Contains(claude.Detail, "measurable: 1 end record from SubagentStop") {
		t.Errorf("claude: %q", claude.Detail)
	}
	codex := f.check(t, rep, "codex", "completion")
	wantStatus(t, codex, doctorOK)
	if !strings.Contains(codex.Detail, "not yet measurable: no end record seen from PostToolUse") {
		t.Errorf("codex: %q", codex.Detail)
	}
	// The end hook setup now writes for codex is reported as missing, with the fix.
	hooks := f.check(t, rep, "codex", "hooks wired")
	if !strings.Contains(hooks.Detail, "missing: PostToolUse end codex") {
		t.Errorf("codex hooks: %q", hooks.Detail)
	}
}
