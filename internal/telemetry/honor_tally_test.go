// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func tallyShift(id, session string) telemetry.Event {
	return telemetry.Event{CorrelationID: id, Harness: "claude-code", SessionID: session,
		Outcome: telemetry.OutcomeRewriteEmitted, FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5"}
}

func tallyResolved(link, got string, honored bool) telemetry.Event {
	return telemetry.Event{Harness: "claude-code", SessionID: "s", Outcome: telemetry.OutcomeResolved,
		LinkedDecision: link, ToModel: got, RewriteHonored: &honored}
}

func TestTallyHonored(t *testing.T) {
	followUp := tallyShift("later", "s")
	followUp.Outcome, followUp.FromModel, followUp.ToModel = telemetry.OutcomeAllow, "claude-haiku-5-5", "claude-haiku-5-5"
	noSession := tallyShift("n", "")

	cases := []struct {
		name                    string
		events                  []telemetry.Event
		shifted, honored, refut int
	}{
		{"resolved confirms", []telemetry.Event{tallyShift("a", "s"), tallyResolved("a", "claude-haiku-5-5", true)}, 1, 1, 0},
		{"resolved refutes", []telemetry.Event{tallyShift("a", "s"), tallyResolved("a", "claude-opus-5-5", false)}, 1, 0, 1},
		{"nothing observed", []telemetry.Event{tallyShift("a", "s")}, 1, 0, 0},
		{"later spawn infers", []telemetry.Event{tallyShift("a", "s"), followUp}, 1, 1, 0},
		{"no session is not counted", []telemetry.Event{noSession}, 0, 0, 0},
		{"refuted beats later spawn", []telemetry.Event{tallyShift("a", "s"), tallyResolved("a", "claude-opus-5-5", false), followUp}, 1, 0, 1},
		{"mixed", []telemetry.Event{tallyShift("a", "s"), tallyResolved("a", "claude-haiku-5-5", true), tallyShift("b", "s"), tallyResolved("b", "claude-opus-5-5", false), tallyShift("c", "s2")}, 3, 1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := telemetry.TallyHonored(c.events)
			if got.Shifted != c.shifted || got.Honored != c.honored || got.Refuted != c.refut {
				t.Fatalf("got %+v, want shifted=%d honored=%d refuted=%d", got, c.shifted, c.honored, c.refut)
			}
			if got.Unobserved() != c.shifted-c.honored-c.refut {
				t.Fatalf("unobserved %d", got.Unobserved())
			}
			s, h := telemetry.CountInferredHonored(c.events)
			if s != got.Shifted || h != got.Honored {
				t.Fatalf("CountInferredHonored diverged: %d/%d", h, s)
			}
		})
	}
}
