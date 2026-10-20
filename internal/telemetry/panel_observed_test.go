// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPanelObservedModel(t *testing.T) {
	const decided = "claude-sonnet-5-5"
	session := strings.Repeat("b", 64)
	decision := func() Event {
		return Event{
			Harness: "claude-code", SessionID: session, Outcome: OutcomeRewriteEmitted, Tier: "MID",
			FromModel: "claude-opus-5-5", ToModel: decided, CorrelationID: "corr-1",
			Timestamp: "2026-10-20T10:00:00Z", ToolUseHash: ToolUseKey("toolu_1"),
		}
	}
	link := func(outcome, model string) Event {
		return Event{
			Harness: "claude-code", SessionID: session, Outcome: outcome, LinkedDecision: "corr-1",
			FromModel: decided, ToModel: model, Timestamp: "2026-10-20T10:00:05Z",
		}
	}
	cases := []struct {
		name   string
		events []Event
		want   string
	}{
		{"no observed record", []Event{decision()}, ""},
		{"resolved equals decided", []Event{decision(), link(OutcomeResolved, decided)}, ""},
		{"resolved differs", []Event{decision(), link(OutcomeResolved, "claude-opus-5-5")}, "claude-opus-5-5"},
		{"usage differs", []Event{decision(), link(OutcomeUsage, "claude-haiku-5-5")}, "claude-haiku-5-5"},
		{"usage equals decided", []Event{decision(), link(OutcomeUsage, decided)}, ""},
		{"dated id is the same model", []Event{decision(), link(OutcomeResolved, "claude-sonnet-5-5-20261001")}, ""},
		{"unknown observed model", []Event{decision(), link(OutcomeResolved, "unknown")}, ""},
		{"resolved wins over usage", []Event{decision(), link(OutcomeUsage, "claude-haiku-5-5"), link(OutcomeResolved, decided)}, ""},
		{"usage after resolved keeps resolved", []Event{decision(), link(OutcomeResolved, "claude-opus-5-5"), link(OutcomeUsage, "claude-haiku-5-5")}, "claude-opus-5-5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := BuildPanel(c.events, session)
			if len(v.Spawns) != 1 {
				t.Fatalf("spawns = %d, want 1", len(v.Spawns))
			}
			sp := v.Spawns[0]
			if sp.ToModel != decided {
				t.Fatalf("to_model = %q, want the decided %q", sp.ToModel, decided)
			}
			if sp.ObservedModel != c.want {
				t.Fatalf("observed_model = %q, want %q", sp.ObservedModel, c.want)
			}
			raw, err := json.Marshal(sp)
			if err != nil {
				t.Fatal(err)
			}
			has := strings.Contains(string(raw), `"observed_model"`)
			if has != (c.want != "") {
				t.Fatalf("observed_model in JSON = %v, want %v: %s", has, c.want != "", raw)
			}
		})
	}
}
