// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package antigravity_test

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/adapters/antigravity"
	"github.com/tiagovilasboas/harness-downshift/internal/catalog"
	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

var cat = catalog.Load()

func decodeOverwriteSubagents(t *testing.T, out antigravity.Output) []map[string]any {
	t.Helper()
	if out.Overwrite == nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(out.Overwrite, &m); err != nil {
		t.Fatalf("overwrite is not valid JSON: %v", err)
	}
	rawList, ok := m["Subagents"].([]any)
	if !ok {
		t.Fatalf("overwrite has no Subagents array: %v", m)
	}
	res := make([]map[string]any, len(rawList))
	for i, r := range rawList {
		sub, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("subagent item %d is not a map: %v", i, r)
		}
		res[i] = sub
	}
	return res
}

func TestEvent_TaskText(t *testing.T) {
	ev := antigravity.Event{
		ToolCall: antigravity.ToolCall{
			Name: "invoke_subagent",
			Args: json.RawMessage(`{"Subagents":[{"Prompt":"check logs"}]}`),
		},
	}
	if got := ev.TaskText(); got != "check logs" {
		t.Fatalf("TaskText() = %q, want %q", got, "check logs")
	}
}

func TestHandle_NonSubagentToolIgnored(t *testing.T) {
	ev := antigravity.Event{
		ToolCall: antigravity.ToolCall{
			Name: "run_command",
			Args: json.RawMessage(`{"CommandLine":"ls"}`),
		},
	}
	out, note, d := antigravity.Handle(ev, cat)
	if out.Decision != "allow" {
		t.Errorf("decision = %q, want allow", out.Decision)
	}
	if out.Overwrite != nil {
		t.Errorf("expected nil overwrite, got %s", string(out.Overwrite))
	}
	if note != "" || d.Harness != "" {
		t.Errorf("expected empty note and decision, got note=%q decision=%+v", note, d)
	}
}

func TestHandle_InvalidArgsIgnored(t *testing.T) {
	ev := antigravity.Event{
		ToolCall: antigravity.ToolCall{
			Name: "invoke_subagent",
			Args: json.RawMessage(`invalid-json`),
		},
	}
	out, _, _ := antigravity.Handle(ev, cat)
	if out.Decision != "allow" || out.Overwrite != nil {
		t.Errorf("expected allow without overwrite, got %+v", out)
	}
}

func TestHandle_EmptySubagentsIgnored(t *testing.T) {
	ev := antigravity.Event{
		ToolCall: antigravity.ToolCall{
			Name: "invoke_subagent",
			Args: json.RawMessage(`{"Subagents":[]}`),
		},
	}
	out, _, _ := antigravity.Handle(ev, cat)
	if out.Decision != "allow" || out.Overwrite != nil {
		t.Errorf("expected allow without overwrite, got %+v", out)
	}
}

func TestHandle_DownshiftsTrivialToFlashLite(t *testing.T) {
	ev := antigravity.Event{
		ToolCall: antigravity.ToolCall{
			Name: "invoke_subagent",
			Args: json.RawMessage(`{
				"Subagents": [
					{
						"Role": "Typo Fixer",
						"TypeName": "research",
						"Prompt": "fix typo in README",
						"Model": "inherit"
					}
				]
			}`),
		},
	}
	out, note, d := antigravity.Handle(ev, cat)
	if out.Decision != "allow" {
		t.Fatalf("decision = %s, want allow", out.Decision)
	}
	if note == "" {
		t.Fatal("expected a downshift note, got empty")
	}
	subs := decodeOverwriteSubagents(t, out)
	if len(subs) != 1 {
		t.Fatalf("expected 1 subagent in overwrite, got %d", len(subs))
	}
	if subs[0]["Model"] != "flash_lite" {
		t.Errorf("model = %v, want flash_lite", subs[0]["Model"])
	}
	if subs[0]["Role"] != "Typo Fixer" {
		t.Errorf("role preserved = %v", subs[0]["Role"])
	}
	if subs[0]["TypeName"] != "research" {
		t.Errorf("typeName preserved = %v", subs[0]["TypeName"])
	}
	if d.Tier != core.TierSmall {
		t.Errorf("tier = %s, want SMALL", d.Tier)
	}
}

func TestHandle_MidTaskToFlash(t *testing.T) {
	ev := antigravity.Event{
		ToolCall: antigravity.ToolCall{
			Name: "invoke_subagent",
			Args: json.RawMessage(`{
				"Subagents": [
					{
						"Role": "Backend Implementer",
						"TypeName": "self",
						"Prompt": "implement the CSV export handler and parse fields",
						"Model": "inherit"
					}
				]
			}`),
		},
	}
	out, _, d := antigravity.Handle(ev, cat)
	subs := decodeOverwriteSubagents(t, out)
	if len(subs) != 1 {
		t.Fatalf("expected 1 subagent in overwrite, got %d", len(subs))
	}
	if subs[0]["Model"] != "flash" {
		t.Errorf("model = %v, want flash", subs[0]["Model"])
	}
	if d.Tier != core.TierMid {
		t.Errorf("tier = %s, want MID", d.Tier)
	}
}

func TestHandle_FrontierTaskToPro(t *testing.T) {
	ev := antigravity.Event{
		ToolCall: antigravity.ToolCall{
			Name: "invoke_subagent",
			Args: json.RawMessage(`{
				"Subagents": [
					{
						"Role": "Chief Architect",
						"TypeName": "self",
						"Prompt": "rearchitect the distributed consensus protocol and multi-region database replication",
						"Model": "inherit"
					}
				]
			}`),
		},
	}
	out, _, d := antigravity.Handle(ev, cat)
	subs := decodeOverwriteSubagents(t, out)
	if len(subs) != 1 {
		t.Fatalf("expected 1 subagent in overwrite, got %d", len(subs))
	}
	if subs[0]["Model"] != "pro" {
		t.Errorf("model = %v, want pro", subs[0]["Model"])
	}
	if d.Tier != core.TierFrontier {
		t.Errorf("tier = %s, want FRONTIER", d.Tier)
	}
}
