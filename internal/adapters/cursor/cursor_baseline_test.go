// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor_test

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// Characterization: baseline is tool_input.model (not inherit), then model_id,
// payload model, then parent transcript at transcript_path when present.
func TestRequestedModel_Baseline(t *testing.T) {
	cases := []struct {
		name      string
		toolInput string
		modelID   string
		model     string
		want      string
	}{
		{name: "tool_input model wins", toolInput: `{"task":"t","model":"a"}`, modelID: "b", model: "c", want: "a"},
		{name: "model_id when tool_input has none", toolInput: `{"task":"t"}`, modelID: "b", model: "c", want: "b"},
		{name: "payload model last", toolInput: `{"task":"t"}`, model: "c", want: "c"},
		{name: "empty tool_input model falls through", toolInput: `{"task":"t","model":""}`, modelID: "b", want: "b"},
		{name: "inherit uses parent model_id", toolInput: `{"task":"t","model":"inherit"}`, modelID: "b", want: "b"},
		{name: "nothing sent", toolInput: `{"task":"t"}`, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := cursor.Event{
				ToolName:  "Task",
				ModelID:   tc.modelID,
				Model:     tc.model,
				ToolInput: json.RawMessage(tc.toolInput),
			}
			if got := ev.RequestedModel(); got != tc.want {
				t.Fatalf("RequestedModel() = %q, want %q", got, tc.want)
			}
		})
	}
}

// inherit must resolve like a missing tool_input.model: baseline is model_id.
func TestHandle_InheritUsesParentModelID(t *testing.T) {
	ev := withCatalogSession(cursor.Event{
		ToolName:  "Task",
		ModelID:   catID(core.TierFrontier),
		ToolInput: json.RawMessage(`{"task":"rename the userId variable to userIdentifier","model":"inherit"}`),
	})
	out, _, d := cursor.Handle(ev, cat)
	wantParent := catID(core.TierFrontier)
	if d.Baseline.RequestedID != wantParent {
		t.Fatalf("Decision.RequestedID = %q, want parent %q", d.Baseline.RequestedID, wantParent)
	}
	if d.Baseline.CurrentModel.ID != wantParent {
		t.Fatalf("Decision.CurrentModel.ID = %q, want parent %q", d.Baseline.CurrentModel.ID, wantParent)
	}
	if d.Verdict != core.VerdictDownshift || d.Tier != core.TierSmall {
		t.Fatalf("want small-tier downshift on parent baseline, got verdict=%v tier=%v", d.Verdict, d.Tier)
	}
	if m := decodeUpdated(t, out); m == nil {
		t.Fatal("inherit on parent frontier must still write a small-tier downshift")
	}
}

// The parent's model_id is a guessed baseline: a mid-tier downshift on it is
// held, since the child may already run on something cheaper.
func TestHandle_ParentModelIDHoldsMidTierDownshift(t *testing.T) {
	ev := withCatalogSession(cursor.Event{
		ToolName:  "Task",
		ModelID:   catID(core.TierFrontier),
		ToolInput: json.RawMessage(`{"task":"implement the CSV export feature"}`),
	})
	out, _, d := cursor.Handle(ev, cat)
	if d.Verdict != core.VerdictDownshift {
		t.Fatalf("precondition: want a downshift verdict, got %v", d.Verdict)
	}
	if !d.Baseline.GuessHeld {
		t.Errorf("held mid-tier downshift must set GuessHeld for its reason code")
	}
	if m := decodeUpdated(t, out); m != nil {
		t.Fatalf("mid-tier downshift on the parent's model_id wrote %v", m["model"])
	}
}

// A small-tier downshift on the parent's model_id is still written.
func TestHandle_ParentModelIDStillWritesSmallTier(t *testing.T) {
	ev := withCatalogSession(cursor.Event{
		ToolName:  "Task",
		ModelID:   catID(core.TierFrontier),
		ToolInput: json.RawMessage(`{"task":"rename the userId variable to userIdentifier"}`),
	})
	out, note, d := cursor.Handle(ev, cat)
	if d.Verdict != core.VerdictDownshift || d.Tier != core.TierSmall {
		t.Fatalf("precondition: want a small-tier downshift, got verdict=%v tier=%v", d.Verdict, d.Tier)
	}
	if m := decodeUpdated(t, out); m == nil {
		t.Fatalf("small-tier downshift on the parent's model_id must be written (note %q)", note)
	}
}
