// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPanelSummary(t *testing.T) {
	cases := []struct {
		name string
		v    PanelView
		want string
	}{
		{"nothing to say", PanelView{}, ""},
		{"all three", PanelView{ModelSwitched: 2, EffortChanged: 1, Held: 1}, "Downshift · 2 model · 1 effort · 1 held"},
		{"models only", PanelView{ModelSwitched: 3}, "Downshift · 3 model"},
		{"held only", PanelView{Held: 4}, "Downshift · 4 held"},
		{"effort and held", PanelView{EffortChanged: 1, Held: 2}, "Downshift · 1 effort · 2 held"},
		{"guardrail counts do not add a segment", PanelView{HeldGuardrail: 1, UpshiftNotApplied: 2}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PanelSummary(c.v); got != c.want {
				t.Fatalf("PanelSummary = %q, want %q", got, c.want)
			}
		})
	}
}

// BuildPanel carries the summary on the session and the saved figures on each
// spawn, and every key is present in the JSON even when empty.
func TestBuildPanelSummaryAndSaved(t *testing.T) {
	session := strings.Repeat("c", 64)
	ev := Event{
		Harness: "claude-code", SessionID: session, Outcome: OutcomeRewriteEmitted, Tier: "MID",
		FromModel: "claude-opus-5-5", ToModel: "claude-sonnet-5-5", CorrelationID: "corr-2",
		Timestamp: "2026-10-20T10:00:00Z", ToolUseHash: ToolUseKey("toolu_2"),
	}
	v := BuildPanel([]Event{ev}, session)
	if v.Summary != "Downshift · 1 model" {
		t.Fatalf("summary = %q, want %q", v.Summary, "Downshift · 1 model")
	}
	if len(v.Spawns) != 1 {
		t.Fatalf("spawns = %d, want 1", len(v.Spawns))
	}
	if sp := v.Spawns[0]; sp.Tokens != (RoutingRow{}) || sp.TokensLabel != "" {
		t.Fatalf("unobserved child tokens = (%+v, %q), want zero", sp.Tokens, sp.TokensLabel)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"summary":"Downshift · 1 model"`, `"below_billable":0`, `"tokens_label":""`} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("panel JSON lacks %s: %s", key, raw)
		}
	}
}
