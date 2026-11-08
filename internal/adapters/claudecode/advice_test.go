// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/core/decidertest"
)

// A decision that keeps the model but carries advice reaches the orchestrator
// as additionalContext, with no permissionDecision and no changed input.
func TestHandle_AdviceIsAdditionalContextOnly(t *testing.T) {
	keep := decidertest.Keep()
	decidertest.Use(t, core.DeciderFunc(func(req core.Request) core.Decision {
		d := keep.Decide(req)
		d.Advice = core.AdviceComplexOnSmall
		return d
	}))
	ev := claudecode.Event{
		ToolName:  "Task",
		ToolInput: json.RawMessage(`{"prompt": "design the distributed ledger", "model": "` + catID(core.TierSmall) + `"}`),
	}
	out, note, _ := claudecode.Handle(withCatalogSession(ev), cat)
	if note != "" {
		t.Fatalf("advice is not a rewrite: note %q", note)
	}
	raw, _ := json.Marshal(out)
	s := string(raw)
	if !strings.Contains(s, `"additionalContext":"downshift: this subagent task reads as complex`) {
		t.Fatalf("output %s lacks the advice", s)
	}
	if strings.Contains(s, "permissionDecision") || strings.Contains(s, "updatedInput") {
		t.Fatalf("advice must not decide permission or change input: %s", s)
	}
}

// Without advice a kept spawn still answers the empty object.
func TestHandle_NoAdviceIsEmpty(t *testing.T) {
	decidertest.Use(t, decidertest.Keep())
	ev := claudecode.Event{
		ToolName:  "Task",
		ToolInput: json.RawMessage(`{"prompt": "design the distributed ledger", "model": "` + catID(core.TierSmall) + `"}`),
	}
	out, _, _ := claudecode.Handle(withCatalogSession(ev), cat)
	if raw, _ := json.Marshal(out); string(raw) != "{}" {
		t.Fatalf("kept spawn without advice = %s, want {}", raw)
	}
}

// A pinned-above-default advice reaches the orchestrator as the same kind of
// note: additionalContext only, nothing decided, nothing rewritten.
func TestHandle_AgentPinAdviceIsAdditionalContext(t *testing.T) {
	keep := decidertest.Keep()
	decidertest.Use(t, core.DeciderFunc(func(req core.Request) core.Decision {
		d := keep.Decide(req)
		d.Advice = core.AdviceAboveAgentDefault
		d.AgentPin = core.AgentPin{Agent: "coder", Default: core.TierMid, Pinned: core.TierFrontier}
		return d
	}))
	ev := claudecode.Event{
		ToolName:  "Task",
		ToolInput: json.RawMessage(`{"prompt": "plain task", "model": "` + catID(core.TierFrontier) + `"}`),
	}
	out, _, _ := claudecode.Handle(withCatalogSession(ev), cat)
	raw, _ := json.Marshal(out)
	s := string(raw)
	if !strings.Contains(s, `"additionalContext":"downshift: coder defaults to mid; you pinned frontier.`) {
		t.Fatalf("output %s lacks the pin note", s)
	}
	if strings.Contains(s, "permissionDecision") || strings.Contains(s, "updatedInput") {
		t.Fatalf("the note must not decide permission or change input: %s", s)
	}
}
