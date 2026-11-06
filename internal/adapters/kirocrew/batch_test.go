// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kirocrew_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/kirocrew"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
)

const trivialTask = "rename the userId variable to userIdentifier"

// batchEvent builds a spawn_run call whose tasks field is the given JSON.
func batchEvent(tasksJSON string) kirocrew.Event {
	return kirocrew.Event{
		ToolName:  "spawn_run",
		ToolInput: json.RawMessage(`{"tasks": ` + tasksJSON + `}`),
	}
}

func entryJSON(task, model string) string {
	return `{"task": "` + task + `", "model": "` + model + `"}`
}

// A batch of two: the first trivial task sits on the frontier tier and must
// respawn on the small one; the second is already on the small tier.
func TestHandle_BatchBlocksOnlyTheWrongTierEntry(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	writeSessionFile(t, []string{smallID, midID, frontierID})
	ev := batchEvent("[" + entryJSON(trivialTask, frontierID) + "," + entryJSON(trivialTask, smallID) + "]")

	out, note, first := kirocrew.Handle(ev, cat)
	if !out.Block {
		t.Fatalf("expected the batch to block, got allow (message=%q)", out.Message)
	}
	if !strings.Contains(out.Message, "entry 1: respawn with model="+smallID) {
		t.Errorf("message must name entry 1 and its target %s, got:\n%s", smallID, out.Message)
	}
	if strings.Contains(out.Message, "entry 2") {
		t.Errorf("message must not name the fine entry 2, got:\n%s", out.Message)
	}
	if note == "" {
		t.Error("expected a batch note, got none")
	}
	if first.Harness == "" {
		t.Error("expected the first entry's decision as the returned decision")
	}
	if ds := out.Decisions(); len(ds) != 2 {
		t.Fatalf("expected one decision per entry (2), got %d", len(ds))
	}
}

func TestHandle_BatchAllowsWhenEveryEntryIsFine(t *testing.T) {
	writeSessionFile(t, []string{smallID, midID, frontierID})
	ev := batchEvent("[" + entryJSON(trivialTask, smallID) + "," + entryJSON(trivialTask, smallID) + "]")

	out, note, first := kirocrew.Handle(ev, cat)
	if out.Block {
		t.Fatalf("expected allow when every entry is fine, got message=%q", out.Message)
	}
	if out.Message != "" || note != "" {
		t.Errorf("allow must carry no message or note, got message=%q note=%q", out.Message, note)
	}
	if first.Harness == "" {
		t.Error("expected a routing decision for the batch")
	}
	if ds := out.Decisions(); len(ds) != 2 {
		t.Errorf("expected one decision per entry (2), got %d", len(ds))
	}
}

func TestHandle_BatchUnchangedWhenNoRoutableEntry(t *testing.T) {
	writeSessionFile(t, []string{smallID, midID, frontierID})
	cases := map[string]string{
		"empty tasks":          `[]`,
		"only blank tasks":     `[{"task": "   "}]`,
		"string not an array":  `"rename the userId variable"`,
		"number entry":         `[42]`,
		"object without task":  `[{"model": "` + frontierID + `"}]`,
		"non-string task":      `[{"task": 5, "model": "` + frontierID + `"}]`,
		"non-string model":     `[{"task": "` + trivialTask + `", "model": 3}]`,
		"object entry is null": `[null]`,
	}
	for name, tasks := range cases {
		t.Run(name, func(t *testing.T) {
			out, note, decision := kirocrew.Handle(batchEvent(tasks), cat)
			if out.Block || out.Message != "" {
				t.Errorf("must pass through unchanged, got block=%t message=%q", out.Block, out.Message)
			}
			if note != "" || decision.Harness != "" || out.Decisions() != nil {
				t.Errorf("must record nothing, got note=%q harness=%q decisions=%d",
					note, decision.Harness, len(out.Decisions()))
			}
		})
	}
}

// A tasks field that is null is absent: the single top-level spawn keeps today's behaviour.
func TestHandle_NullTasksKeepsSingleSpawn(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	writeSessionFile(t, []string{smallID, midID, frontierID})
	ev := kirocrew.Event{
		ToolName:  "spawn_run",
		ToolInput: json.RawMessage(`{"tasks": null, "task": "` + trivialTask + `", "model": "` + frontierID + `"}`),
	}
	out, _, decision := kirocrew.Handle(ev, cat)
	if decision.Harness == "" {
		t.Fatal("expected the single spawn to be routed")
	}
	if !out.Block || !strings.Contains(out.Message, "model="+smallID) {
		t.Errorf("expected the single spawn to block toward %s, got block=%t message=%q", smallID, out.Block, out.Message)
	}
	if out.Decisions() != nil {
		t.Errorf("a single spawn carries no per-entry decisions, got %d", len(out.Decisions()))
	}
}

// spawn_sub_agents takes agents[] with no per-entry model (prompt and
// agent_or_mode only). Downshift cannot route it, so it passes through.
func TestHandle_SpawnSubAgentsBatchPassesThrough(t *testing.T) {
	writeSessionFile(t, []string{smallID, midID, frontierID})
	ev := kirocrew.Event{
		ToolName:  "spawn_sub_agents",
		ToolInput: json.RawMessage(`{"agents": [{"agent_or_mode": "reviewer", "prompt": "` + trivialTask + `"}]}`),
	}
	out, note, decision := kirocrew.Handle(ev, cat)
	if out.Block || note != "" || decision.Harness != "" {
		t.Errorf("spawn_sub_agents must pass through unchanged, got block=%t note=%q harness=%q",
			out.Block, note, decision.Harness)
	}
}
