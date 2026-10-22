// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

func TestAggregateByHarness_Empty(t *testing.T) {
	result := AggregateByHarness(nil)

	if result.Total != 0 {
		t.Errorf("Total = %d, want 0", result.Total)
	}
	if len(result.ByHarness) != 0 {
		t.Errorf("ByHarness len = %d, want 0", len(result.ByHarness))
	}
}

func TestAggregateByHarness_SingleHarness(t *testing.T) {
	events := []execwrap.Event{
		{Harness: "kirocrew", Entry: "hook", Family: "go_test", RawBytes: 1000, EmittedBytes: 200, Compacted: true},
		{Harness: "kirocrew", Entry: "hook", Family: "go_test", RawBytes: 500, EmittedBytes: 100, Compacted: true},
		{Harness: "kirocrew", Entry: "direct", Family: "gh_cli", RawBytes: 200, EmittedBytes: 200, Compacted: false},
	}

	result := AggregateByHarness(events)

	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}
	if len(result.ByHarness) != 1 {
		t.Fatalf("ByHarness len = %d, want 1", len(result.ByHarness))
	}

	m := result.ByHarness[0]
	if m.Harness != "kirocrew" {
		t.Errorf("Harness = %q, want %q", m.Harness, "kirocrew")
	}
	if m.Commands != 3 {
		t.Errorf("Commands = %d, want 3", m.Commands)
	}
	if m.Compacted != 2 {
		t.Errorf("Compacted = %d, want 2", m.Compacted)
	}
	if m.RawBytes != 1700 {
		t.Errorf("RawBytes = %d, want 1700", m.RawBytes)
	}
	if m.SavedBytes != 1200 {
		t.Errorf("SavedBytes = %d, want 1200", m.SavedBytes)
	}
	if m.ByEntry["hook"] != 2 {
		t.Errorf("ByEntry[hook] = %d, want 2", m.ByEntry["hook"])
	}
	if m.ByEntry["direct"] != 1 {
		t.Errorf("ByEntry[direct] = %d, want 1", m.ByEntry["direct"])
	}
	if m.ByFamily["go_test"] != 2 {
		t.Errorf("ByFamily[go_test] = %d, want 2", m.ByFamily["go_test"])
	}
}

func TestAggregateByHarness_MultipleHarnesses(t *testing.T) {
	events := []execwrap.Event{
		{Harness: "kirocrew", Entry: "hook", Family: "go_test", RawBytes: 1000, EmittedBytes: 200, Compacted: true},
		{Harness: "claude-code", Entry: "hook", Family: "go_test", RawBytes: 500, EmittedBytes: 100, Compacted: true},
		{Harness: "claude-code", Entry: "hook", Family: "gh_cli", RawBytes: 300, EmittedBytes: 50, Compacted: true},
		{Harness: "cursor", Entry: "direct", Family: "npm", RawBytes: 200, EmittedBytes: 200, Compacted: false},
	}

	result := AggregateByHarness(events)

	if result.Total != 4 {
		t.Errorf("Total = %d, want 4", result.Total)
	}
	if len(result.ByHarness) != 3 {
		t.Fatalf("ByHarness len = %d, want 3", len(result.ByHarness))
	}

	// Should be sorted by commands desc: claude-code (2), kirocrew (1), cursor (1).
	if result.ByHarness[0].Harness != "claude-code" {
		t.Errorf("First harness = %q, want claude-code (most commands)", result.ByHarness[0].Harness)
	}
	if result.ByHarness[0].Commands != 2 {
		t.Errorf("claude-code commands = %d, want 2", result.ByHarness[0].Commands)
	}
}

func TestAggregateByHarness_UnknownHarness(t *testing.T) {
	events := []execwrap.Event{
		{Harness: "", Entry: "direct", Family: "go_test", RawBytes: 1000, EmittedBytes: 200, Compacted: true},
		{Harness: "", Entry: "", Family: "", RawBytes: 500, EmittedBytes: 500, Compacted: false},
	}

	result := AggregateByHarness(events)

	if len(result.ByHarness) != 1 {
		t.Fatalf("ByHarness len = %d, want 1", len(result.ByHarness))
	}

	m := result.ByHarness[0]
	if m.Harness != "unknown" {
		t.Errorf("Harness = %q, want %q", m.Harness, "unknown")
	}
	if m.Commands != 2 {
		t.Errorf("Commands = %d, want 2", m.Commands)
	}
	if m.ByEntry["direct"] != 1 {
		t.Errorf("ByEntry[direct] = %d, want 1", m.ByEntry["direct"])
	}
	if m.ByEntry["unknown"] != 1 {
		t.Errorf("ByEntry[unknown] = %d, want 1", m.ByEntry["unknown"])
	}
	if m.ByFamily["go_test"] != 1 {
		t.Errorf("ByFamily[go_test] = %d, want 1", m.ByFamily["go_test"])
	}
	if m.ByFamily["other"] != 1 {
		t.Errorf("ByFamily[other] = %d, want 1", m.ByFamily["other"])
	}
}

func TestAggregateByHarness_SortStability(t *testing.T) {
	// Three harnesses with same command count.
	events := []execwrap.Event{
		{Harness: "a", RawBytes: 100, EmittedBytes: 100},
		{Harness: "b", RawBytes: 100, EmittedBytes: 100},
		{Harness: "c", RawBytes: 100, EmittedBytes: 100},
	}

	result := AggregateByHarness(events)

	if len(result.ByHarness) != 3 {
		t.Fatalf("ByHarness len = %d, want 3", len(result.ByHarness))
	}
	// All have 1 command, sort is stable (insertion sort preserves original order).
	for _, m := range result.ByHarness {
		if m.Commands != 1 {
			t.Errorf("Harness %q: Commands = %d, want 1", m.Harness, m.Commands)
		}
	}
}
