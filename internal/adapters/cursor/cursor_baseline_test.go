// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor_test

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
)

// Characterization: Cursor sends no transcript, so the requested model comes
// only from the payload: tool_input.model, then model_id, then model. The
// literal "inherit" is passed through unchanged; it is not resolvable here.
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
		{name: "inherit passes through unresolved", toolInput: `{"task":"t","model":"inherit"}`, modelID: "b", want: "inherit"},
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

// "inherit" is not a catalog id, so Handle routes with no recognised current
// model. That empty CurrentModel is what makes the runner log unknown.
func TestHandle_InheritBaselineIsUnrecognised(t *testing.T) {
	ev := cursor.Event{
		ToolName:  "Task",
		ToolInput: json.RawMessage(`{"task":"rename the userId variable","model":"inherit"}`),
	}
	_, _, d := cursor.Handle(ev, cat)
	if d.RequestedID != "inherit" {
		t.Fatalf("Decision.RequestedID = %q, want inherit", d.RequestedID)
	}
	if d.CurrentModel.ID != "" {
		t.Fatalf("Decision.CurrentModel.ID = %q, want empty (inherit is unresolvable)", d.CurrentModel.ID)
	}
}
