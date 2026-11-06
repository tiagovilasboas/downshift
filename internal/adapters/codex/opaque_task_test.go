// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codex_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"github.com/tiagovilasboas/downshift/internal/telemetry/fromcore"
)

// A synthetic Fernet-shaped envelope, not an encrypted real instruction. The
// hook has no key and cannot authenticate or interpret its contents.
const opaqueTask = "gAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="

func TestHandle_OpaqueTaskCannotRouteFromTaskName(t *testing.T) {
	for _, name := range []string{"rename the userId variable", "review authentication security and migrate payments"} {
		for _, fork := range []string{"all", "none"} {
			t.Run(name+"/"+fork, func(t *testing.T) {
				for _, message := range []string{opaqueTask, strings.TrimRight(opaqueTask, "=")} {
					raw, err := json.Marshal(map[string]any{"message": message, "task_name": name, "fork_turns": fork})
					if err != nil {
						t.Fatal(err)
					}
					ev := withCatalogSession(codex.Event{ToolName: "collaborationspawn_agent", Model: catID(core.TierFrontier), ToolInput: raw})
					if task := ev.TaskText(); task != "" {
						t.Fatalf("opaque instruction must produce no training features: %q", task)
					}
					out, note, d := codex.Handle(ev, cat)
					wire, err := json.Marshal(out)
					if err != nil {
						t.Fatal(err)
					}
					if string(wire) != "{}" || note != "" {
						t.Fatalf("opaque task cannot change model, effort or permissions: wire=%s note=%q", wire, note)
					}
					if !d.TaskNotVisible || !d.NoSignal || d.Confident || len(d.Risk) != 0 || d.Harness != "codex" {
						t.Fatalf("decision must observe unavailable task without classifying name/ciphertext: %+v", d)
					}
					if !slices.Contains(fromcore.ReasonCodes(d), telemetry.ReasonTaskNotVisible) {
						t.Fatalf("missing task visibility reason: %v", fromcore.ReasonCodes(d))
					}
					stored, err := json.Marshal(fromcore.FromDecision(d, "test", "test"))
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(string(stored), message) || strings.Contains(string(stored), name) {
						t.Fatal("telemetry contains ciphertext or task name")
					}
				}
			})
		}
	}
}

func TestHandle_ReadableTaskWithTokenPrefixStillRoutes(t *testing.T) {
	raw := json.RawMessage(`{"message":"gAAAAA is a token prefix; rename the userId variable to userIdentifier","task_name":"worker","fork_turns":"none"}`)
	ev := withCatalogSession(codex.Event{ToolName: "spawn_agent", Model: catID(core.TierFrontier), ToolInput: raw})
	if ev.TaskText() == "" {
		t.Fatal("a readable instruction must not be hidden by a prefix match")
	}
	out, _, d := codex.Handle(ev, cat)
	if d.TaskNotVisible || decodeUpdated(t, out) == nil {
		t.Fatalf("readable task must retain routing, got decision %+v output %+v", d, out)
	}
}
