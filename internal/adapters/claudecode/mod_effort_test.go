// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/core"
)

func TestHandle_ModEntry_EmitsReasoningEffortOnDownshift(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := claudecode.Event{
		ToolName:  "Task",
		Model:     frontierID,
		Entry:     "mod",
		ToolInput: json.RawMessage(`{"prompt": "fix a typo in the readme"}`),
	}
	out, _, _ := claudecode.Handle(withCatalogSession(ev), cat)
	m := decodeUpdated(t, out)
	if m == nil {
		t.Fatal("expected updatedInput for mod downshift")
	}
	if m["reasoning_effort"] != "low" {
		t.Errorf("reasoning_effort = %v, want low", m["reasoning_effort"])
	}
}

func TestHandle_SettingsEntry_NoReasoningEffortOnDownshift(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := claudecode.Event{
		ToolName:  "Task",
		Model:     frontierID,
		ToolInput: json.RawMessage(`{"prompt": "fix a typo in the readme"}`),
	}
	out, _, _ := claudecode.Handle(withCatalogSession(ev), cat)
	m := decodeUpdated(t, out)
	if m == nil {
		t.Fatal("expected model rewrite")
	}
	if _, ok := m["reasoning_effort"]; ok {
		t.Errorf("settings hook path must not write reasoning_effort, got %v", m["reasoning_effort"])
	}
}

func TestHandle_ModEntry_NoEffortWhenHeld(t *testing.T) {
	midID := catID(core.TierMid)
	ev := claudecode.Event{
		ToolName:  "Task",
		Model:     midID,
		Entry:     "mod",
		ToolInput: json.RawMessage(`{"prompt": "look into why the build is slow"}`),
	}
	out, _, d := claudecode.Handle(withCatalogSession(ev), cat)
	if d.Checked && d.SafeVerdict != d.Verdict {
		m := decodeUpdated(t, out)
		if m != nil {
			if _, ok := m["reasoning_effort"]; ok {
				t.Fatalf("held decision must not emit reasoning_effort, input=%v", m)
			}
		}
	}
}
