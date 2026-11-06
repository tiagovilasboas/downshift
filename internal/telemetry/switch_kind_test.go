// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"testing"
	"time"
)

// effortOnly is a rewrite that kept the model and changed only the effort.
func effortOnly(id string, d time.Duration) Event {
	return Event{
		CorrelationID: id, Timestamp: at(d), SessionID: sessA, Harness: "claude-code",
		Outcome: OutcomeRewriteEmitted, Tier: "mid", Verdict: "DOWNSHIFT",
		FromModel: "claude-haiku-5-5", ToModel: "claude-haiku-5-5",
		RequestedEffort: "high", FinalEffort: "low",
	}
}

// modelSwitch is a rewrite from opus to haiku.
func modelSwitch(id string, d time.Duration) Event {
	e := effortOnly(id, d)
	e.FromModel = "claude-opus-5-5"
	return e
}

// heldRewrite is a rewrite a guardrail kept as it was.
func heldRewrite(id string, d time.Duration) Event {
	e := modelSwitch(id, d)
	e.ReasonCodes = []string{ReasonGuessHeld}
	return e
}

func TestSwitchKind_PanelCounts(t *testing.T) {
	cases := []struct {
		name                          string
		events                        []Event
		switched, model, effort, held int
	}{
		{"two effort-only rewrites", []Event{effortOnly("a", 0), effortOnly("b", time.Minute)}, 2, 0, 2, 0},
		{"one model rewrite", []Event{modelSwitch("a", 0)}, 1, 1, 0, 0},
		{"mixed with a held row", []Event{effortOnly("a", 0), modelSwitch("b", time.Minute), heldRewrite("c", 2*time.Minute)}, 2, 1, 1, 1},
		{"held only", []Event{heldRewrite("a", 0)}, 0, 0, 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := BuildPanel(c.events, sessA)
			if v.Switched != c.switched || v.ModelSwitched != c.model || v.EffortChanged != c.effort || v.Held != c.held {
				t.Fatalf("switched=%d model=%d effort=%d held=%d, want %d %d %d %d",
					v.Switched, v.ModelSwitched, v.EffortChanged, v.Held, c.switched, c.model, c.effort, c.held)
			}
		})
	}
}

func TestSwitchKind_TreeHeader(t *testing.T) {
	cases := []struct {
		name   string
		events []Event
		want   string
	}{
		{"zero effort keeps the old header", []Event{modelSwitch("a", 0)},
			"session · 1 subagent · 1 model changed · tokens not observed"},
		{"held only keeps the old header", []Event{heldRewrite("a", 0)},
			"session · 1 subagent · no model changed · 1 kept as requested (why: --why) · tokens not observed"},
		{"effort-only rewrites split the header", []Event{effortOnly("a", 0), effortOnly("b", time.Minute)},
			"session · 2 subagents · no model changed · 2 effort changed · tokens not observed"},
		{"mixed header", []Event{effortOnly("a", 0), modelSwitch("b", time.Minute), heldRewrite("c", 2*time.Minute)},
			"session · 3 subagents · 1 model changed · 1 effort changed · 1 kept as requested (why: --why) · tokens not observed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := lines(SessionTree(c.events, sessA, TreeOptions{Now: base}))
			if got[0] != c.want {
				t.Fatalf("header = %q, want %q", got[0], c.want)
			}
		})
	}
}

func TestSwitchKind_TreeCounts(t *testing.T) {
	v := BuildSessionTree([]Event{effortOnly("a", 0), effortOnly("b", time.Minute)}, sessA, TreeOptions{Now: base})
	if v.Switched != 2 || v.ModelSwitched != 0 || v.EffortChanged != 2 {
		t.Fatalf("switched=%d model=%d effort=%d, want 2 0 2", v.Switched, v.ModelSwitched, v.EffortChanged)
	}
	if !strings.Contains(v.Render(), "no model changed") {
		t.Fatalf("effort-only tree must say no model changed: %q", v.Render())
	}
}
