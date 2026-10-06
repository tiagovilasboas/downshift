// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package antigravity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/adapters/antigravity"
	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

// stubResolver is a minimal core.Resolver modeling an Antigravity-native
// catalog: the wire aliases flash_lite/flash/pro/inherit in one harness.
// The embedded catalog has no Antigravity entries, so the real catalog
// always reports R4_FOREIGN_MODEL here; the stub gives tests a clean
// (Corrections-empty) decision for the allowed-rewrite path, and a
// foreign model on demand for the held-decision path.
type stubResolver struct {
	explicit map[string]bool
	foreign  bool
}

var stubModels = []core.Model{
	{ID: "flash_lite", Harness: "antigravity", Tier: core.TierSmall, InputM: 1, OutputM: 5},
	{ID: "flash", Harness: "antigravity", Tier: core.TierMid, InputM: 3, OutputM: 15},
	{ID: "pro", Harness: "antigravity", Tier: core.TierFrontier, InputM: 5, OutputM: 25},
	{ID: "inherit", Harness: "antigravity", Tier: core.TierMid, InputM: 3, OutputM: 15},
}

func (s stubResolver) ModelFor(harness string, t core.Tier) core.Model {
	if s.foreign {
		return core.Model{ID: "claude-opus-5-5", Harness: "claude-code", Tier: t, InputM: 5, OutputM: 25}
	}
	for _, m := range stubModels {
		if m.Tier == t {
			return m
		}
	}
	return core.Model{Tier: t, Harness: harness}
}

func (s stubResolver) LookupByID(harness, id string) (core.Model, bool) {
	for _, m := range stubModels {
		if m.ID == id {
			return m, true
		}
	}
	return core.Model{}, false
}

func (s stubResolver) SavingsRatio(from, to core.Model) float64 {
	if from.Tier <= to.Tier {
		return 0
	}
	return 0.5
}

func (s stubResolver) EffortFor(harness, id string, e core.Effort) string {
	return e.String()
}

func (s stubResolver) IsExplicitOnly(harness, id string) bool {
	return s.explicit[id]
}

var res = stubResolver{}

func withWireSession(ev antigravity.Event) antigravity.Event {
	ids := []string{"flash_lite", "flash", "pro"}
	ev.SessionModels = &ids
	return ev
}

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

func TestMain(m *testing.M) {
	os.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(os.TempDir(), "downshift-session-models-absent.json"))
	os.Exit(m.Run())
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
	out, note, d := antigravity.Handle(ev, res)
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
	out, _, _ := antigravity.Handle(ev, res)
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
	out, _, _ := antigravity.Handle(ev, res)
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
	out, note, d := antigravity.Handle(withWireSession(ev), res)
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
						"Model": "flash_lite"
					}
				]
			}`),
		},
	}
	out, _, d := antigravity.Handle(withWireSession(ev), res)
	subs := decodeOverwriteSubagents(t, out)
	if len(subs) != 1 {
		t.Fatalf("expected 1 subagent in overwrite, got %d", len(subs))
	}
	if subs[0]["Model"] != "pro" {
		t.Errorf("model = %v, want strongest session model pro for an upshift", subs[0]["Model"])
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
	out, _, d := antigravity.Handle(withWireSession(ev), res)
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

func TestHandle_UnknownSessionDoesNotRewrite(t *testing.T) {
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
	// No SessionModels on the event and TestMain points the user file at
	// an absent path, so the session is unknown and the hook must fail open.
	out, note, d := antigravity.Handle(ev, res)
	if len(d.Corrections) != 0 {
		t.Fatalf("expected a clean decision so the session is the only blocker, got %v", d.Corrections)
	}
	if note != "" {
		t.Errorf("expected no note for unknown session, got %q", note)
	}
	if out.Overwrite != nil {
		t.Errorf("unknown session must not rewrite, got %s", string(out.Overwrite))
	}
}

func TestHandle_HeldDecisionDoesNotRewrite(t *testing.T) {
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
	// The foreign resolver recommends a model from another harness, so the
	// guardrail holds the decision (R4_FOREIGN_MODEL) even though the plan
	// names a session target.
	out, note, d := antigravity.Handle(withWireSession(ev), stubResolver{foreign: true})
	if len(d.Corrections) == 0 {
		t.Fatal("expected a held decision, got none")
	}
	if note != "" {
		t.Errorf("expected no note for held decision, got %q", note)
	}
	if out.Overwrite != nil {
		t.Errorf("held decision must not rewrite, got %s", string(out.Overwrite))
	}
}

func TestHandle_ExplicitOnlyTargetIsSkippedForNextSessionModel(t *testing.T) {
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
	// The lowest-capability entry is explicit-only. Selection stays within
	// the session and skips it in favor of the next eligible model.
	explicitRes := stubResolver{explicit: map[string]bool{"flash_lite": true}}
	out, note, d := antigravity.Handle(withWireSession(ev), explicitRes)
	if len(d.Corrections) != 0 {
		t.Fatalf("expected a clean decision so explicit-only is the only blocker, got %v", d.Corrections)
	}
	if note == "" {
		t.Fatal("expected the next eligible session model to be selected")
	}
	subs := decodeOverwriteSubagents(t, out)
	if len(subs) != 1 || subs[0]["Model"] != "flash" {
		t.Errorf("expected non-explicit session model flash, got %v", subs)
	}
}

func TestHandle_ExplicitOnlyCurrentModelDoesNotRewrite(t *testing.T) {
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
	// The user's current model is explicit-only: the plan preserves it.
	explicitRes := stubResolver{explicit: map[string]bool{"inherit": true}}
	out, _, _ := antigravity.Handle(withWireSession(ev), explicitRes)
	if out.Overwrite != nil {
		t.Errorf("explicit-only current model must not rewrite, got %s", string(out.Overwrite))
	}
}
