// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codex_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// Full-history forks must remain valid inherited spawns even when the shared
// router recommends another model or an effort-only change.
func TestHandle_FullHistoryForkCannotOverrideModelOrEffort(t *testing.T) {
	for _, scenario := range []struct {
		name, message, model string
	}{
		{"model and effort", "rename the userId variable to userIdentifier", catID(core.TierFrontier)},
		{"effort only", "implement the CSV export feature", catID(core.TierMid)},
	} {
		for _, fork := range []struct {
			name    string
			value   any
			omitted bool
		}{
			{"all", "all", false},
			{"trimmed mixed case", "  AlL\t", false},
			{"legacy positive count", " 2 ", false},
			{"positive count with plus", "+1", false},
			{"empty default", "", false},
			{"blank default", " \t", false},
			{"null default", nil, false},
			{"omitted default", nil, true},
		} {
			t.Run(scenario.name+"/"+fork.name, func(t *testing.T) {
				input := map[string]any{
					"message": scenario.message, "task_name": "worker",
					"custom": map[string]any{"keep": true},
				}
				if !fork.omitted {
					input["fork_turns"] = fork.value
				}
				raw, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}
				ev := withCatalogSession(codex.Event{ToolName: "spawn_agent", Model: scenario.model, ToolInput: raw})
				out, note, d := codex.Handle(ev, cat)
				wire, err := json.Marshal(out)
				if err != nil {
					t.Fatal(err)
				}
				if string(wire) != "{}" || note != "" {
					t.Fatalf("full-history fork must emit no override or permission decision: wire=%s note=%q", wire, note)
				}
				if d.Harness != "codex" || d.Baseline.RequestedID != scenario.model || d.Session.Unknown {
					t.Fatalf("routing observation and inherited baseline must remain available: %+v", d)
				}
				if string(ev.ToolInput) != string(raw) {
					t.Fatal("full-history fork input was modified")
				}

				// Prove the task exercises a real rewrite path when independent
				// context permits it, rather than merely asserting a held decision.
				input["fork_turns"] = "none"
				ev.ToolInput, err = json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}
				allowed, _, _ := codex.Handle(ev, cat)
				if m := decodeUpdated(t, allowed); m == nil || m["reasoning_effort"] == nil {
					t.Fatalf("independent spawn should exercise model/effort routing, got %v", m)
				}
			})
		}
	}
}

func TestHandle_IndependentAndLegacyForksPreserveInputFields(t *testing.T) {
	for _, fork := range []string{"none", "  NoNe\t", ""} {
		t.Run("fork="+fork, func(t *testing.T) {
			input := map[string]any{
				"message": "rename the userId variable to userIdentifier",
				"custom":  map[string]any{"keep": true},
			}
			if fork != "" {
				input["task_name"] = "worker"
				input["fork_turns"] = fork
			}
			raw, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			ev := withCatalogSession(codex.Event{ToolName: "spawn_agent", Model: catID(core.TierFrontier), ToolInput: raw})
			out, _, _ := codex.Handle(ev, cat)
			updated := decodeUpdated(t, out)
			if updated == nil || updated["model"] != catID(core.TierSmall) {
				t.Fatalf("independent/legacy spawn must retain routing: %v", updated)
			}
			for key, value := range input {
				if !reflect.DeepEqual(updated[key], value) {
					t.Errorf("field %s changed: got %v, want %v", key, updated[key], value)
				}
			}
			if fork == "" {
				if _, exists := updated["fork_turns"]; exists {
					t.Fatal("routing must not introduce a context mode")
				}
			}
		})
	}
}
