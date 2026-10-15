// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codex_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
)

// Characterization: the requested model is the baseline a rewrite is
// compared against. A spawn_agent without "model" inherits the parent model,
// which Codex only exposes through the rollout's last turn_context. If this
// lookup breaks, the baseline silently becomes unknown.
func TestRequestedModel_Baseline(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	rollout := write("rollout.jsonl", `{"type":"session_meta","payload":{"id":"x"}}
{"type":"turn_context","payload":{"model":"gpt-6-sol"}}
{"type":"response_item","payload":{"type":"message"}}
{"type":"turn_context","payload":{"model":"gpt-6-luna"}}
`)
	empty := write("empty.jsonl", "")
	noTurn := write("noturn.jsonl", `{"type":"session_meta","payload":{"model":"gpt-6-sol"}}
`)
	notJSONL := write("rollout.json", `{"type":"turn_context","payload":{"model":"gpt-6-luna"}}
`)

	cases := []struct {
		name       string
		toolInput  string
		model      string
		transcript string
		want       string
	}{
		{name: "inherits last turn_context", toolInput: `{"message":"rename a variable"}`, transcript: rollout, want: "gpt-6-luna"},
		{name: "explicit tool_input model wins over transcript", toolInput: `{"message":"rename a variable","model":"gpt-6-sol"}`, transcript: rollout, want: "gpt-6-sol"},
		{name: "payload model wins over transcript", toolInput: `{"message":"rename a variable"}`, model: "gpt-6-sol", transcript: rollout, want: "gpt-6-sol"},
		{name: "empty model field falls through to transcript", toolInput: `{"message":"rename a variable","model":""}`, transcript: rollout, want: "gpt-6-luna"},
		{name: "no transcript path", toolInput: `{"message":"rename a variable"}`, want: ""},
		{name: "missing transcript file", toolInput: `{"message":"rename a variable"}`, transcript: filepath.Join(dir, "absent.jsonl"), want: ""},
		{name: "empty transcript file", toolInput: `{"message":"rename a variable"}`, transcript: empty, want: ""},
		{name: "transcript without turn_context", toolInput: `{"message":"rename a variable"}`, transcript: noTurn, want: ""},
		{name: "non-jsonl transcript is ignored", toolInput: `{"message":"rename a variable"}`, transcript: notJSONL, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := codex.Event{
				ToolName:       "spawn_agent",
				Model:          tc.model,
				TranscriptPath: tc.transcript,
				ToolInput:      json.RawMessage(tc.toolInput),
			}
			if got := ev.RequestedModel(); got != tc.want {
				t.Fatalf("RequestedModel() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Handle must route against the same baseline RequestedModel reports, so the
// decision and the telemetry row describe the same requested model.
func TestHandle_BaselineMatchesRequestedModel(t *testing.T) {
	rollout := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(rollout, []byte(`{"type":"turn_context","payload":{"model":"gpt-6-luna"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ev := codex.Event{
		ToolName:       "spawn_agent",
		TranscriptPath: rollout,
		ToolInput:      json.RawMessage(`{"message":"rename the userId variable"}`),
	}
	_, _, d := codex.Handle(ev, cat)
	if d.RequestedID != "gpt-6-luna" {
		t.Fatalf("Decision.RequestedID = %q, want gpt-6-luna", d.RequestedID)
	}
	if d.CurrentModel.ID != "gpt-6-luna" {
		t.Fatalf("Decision.CurrentModel.ID = %q, want gpt-6-luna (catalog-recognised)", d.CurrentModel.ID)
	}
}
