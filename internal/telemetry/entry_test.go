// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestNormalizeEntry(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", telemetry.EntryHook},
		{"hook", telemetry.EntryHook},
		{"mod", telemetry.EntryMod},
		{"other", telemetry.EntryHook},
	}
	for _, c := range cases {
		if got := telemetry.NormalizeEntry(c.in); got != c.want {
			t.Errorf("NormalizeEntry(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDuplicateToolUseCount(t *testing.T) {
	events := []telemetry.Event{
		{Harness: "claude-code", Outcome: telemetry.OutcomeAllow, ToolUseHash: "h1", Entry: telemetry.EntryHook},
		{Harness: "claude-code", Outcome: telemetry.OutcomeAllow, ToolUseHash: "h1", Entry: telemetry.EntryMod},
		{Harness: "claude-code", Outcome: telemetry.OutcomeAllow, ToolUseHash: "h2", Entry: telemetry.EntryHook},
		{Harness: "claude-code", Outcome: telemetry.OutcomeUsage, ToolUseHash: "h1"}, // not a decision
		{Harness: "cursor", Outcome: telemetry.OutcomeAllow, ToolUseHash: "h1", Entry: telemetry.EntryHook},
		{Harness: "cursor", Outcome: telemetry.OutcomeAllow, ToolUseHash: "h1", Entry: telemetry.EntryMod},
	}
	if got := telemetry.DuplicateToolUseCount(events); got != 2 {
		t.Fatalf("DuplicateToolUseCount = %d, want 2 (one per harness)", got)
	}
	if got := telemetry.DuplicateToolUseCount(events[:1]); got != 0 {
		t.Fatalf("single decision = %d, want 0", got)
	}
}

func TestAggregate_ByEntryAndDuplicate(t *testing.T) {
	events := []telemetry.Event{
		{Harness: "claude-code", Outcome: telemetry.OutcomeAllow, Complexity: "TRIVIAL", ToolUseHash: "h1", Entry: telemetry.EntryHook},
		{Harness: "claude-code", Outcome: telemetry.OutcomeAllow, Complexity: "TRIVIAL", ToolUseHash: "h1", Entry: telemetry.EntryMod},
		{Harness: "claude-code", Outcome: telemetry.OutcomeAllow, Complexity: "MEDIUM"}, // empty entry → hook
	}
	s := telemetry.Aggregate(events)
	if s.ByEntry[telemetry.EntryHook] != 2 || s.ByEntry[telemetry.EntryMod] != 1 {
		t.Fatalf("ByEntry = %#v", s.ByEntry)
	}
	if s.DuplicateToolUse != 1 {
		t.Fatalf("DuplicateToolUse = %d, want 1", s.DuplicateToolUse)
	}
}
