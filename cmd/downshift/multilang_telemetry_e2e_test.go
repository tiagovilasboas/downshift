// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
// This is a hook simulation; it does not prove a native executor's model.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/antigravity"
	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/adapters/kirocrew"
	"github.com/tiagovilasboas/downshift/internal/adapters/opencode"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// guardedLanguageStub pins the language guard's decision: the unsupported
// prompt is held with UnsupportedLanguage set (the active model stays), any
// other prompt goes to the small tier.
func guardedLanguageStub() core.Decider {
	return decidertest.When(
		func(r core.Request) bool { return strings.Contains(r.Prompt, "contraseñas") },
		decidertest.Mark(decidertest.Held(core.TierSmall), func(d *core.Decision) { d.UnsupportedLanguage = true }),
		decidertest.Tier(core.TierSmall),
	)
}

func TestHookE2E_MultilangGuardAcrossHarnesses(t *testing.T) {
	decidertest.Use(t, guardedLanguageStub())
	t.Setenv("HOME", t.TempDir())
	const unsupported = "rename el campo que contiene las contraseñas de clientes"
	const supported = "rename the userId variable to userIdentifier"
	for _, h := range core.AllHarnessCaps() {
		t.Run(h.Harness, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)
			ids := []string{cmdCat.ModelFor(h.Harness, core.TierSmall).ID, cmdCat.ModelFor(h.Harness, core.TierMid).ID, cmdCat.ModelFor(h.Harness, core.TierFrontier).ID}
			setSessionAllowlist(t, h.Harness, ids)
			for _, prompt := range []string{unsupported, supported} {
				if err := os.WriteFile(logPath, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				input := map[string]any{"message": prompt, "task": prompt, "prompt": prompt, "model": ids[2]}
				payload := map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Task", "tool_input": input}
				switch h.Harness {
				case "codex":
					payload["tool_name"] = "spawn_agent"
				case "kirocrew":
					payload["tool_name"] = "spawn_run"
				case "cursor":
					payload["hook_event_name"] = "preToolUse"
				case "antigravity":
					payload = map[string]any{"toolCall": map[string]any{"name": "invoke_subagent", "args": map[string]any{"Subagents": []any{map[string]any{"Prompt": prompt, "Model": ids[2]}}}}}
				case "opencode":
					// The route wire carries no session list; the allowlist
					// comes from the operator file (set above).
					payload = map[string]any{
						"v":           1,
						"kind":        "route",
						"harness":     map[string]any{"id": "opencode"},
						"event":       "subagent_spawn_intent",
						"correlation": map[string]any{"child": "aa11bb22cc33dd44ee55ff6600778899"},
						"agent":       "general",
						"model":       map[string]any{"providerID": "opencode", "id": ids[2]},
						"taskText":    prompt,
					}
				}
				raw, err := json.Marshal(payload)
				if err != nil {
					t.Fatal(err)
				}
				captureStdout(func() { runQAAdapterFixture(raw, h.Harness) })
				events, err := telemetry.ReadEventsFrom(logPath)
				if err != nil {
					t.Fatal(err)
				}
				if len(events) != 1 {
					t.Fatalf("events=%d want 1", len(events))
				}
				event := events[0]
				guarded := prompt == unsupported
				if slices.Contains(event.ReasonCodes, "UNSUPPORTED_LANGUAGE") != guarded {
					t.Errorf("prompt guarded=%v codes=%v", guarded, event.ReasonCodes)
				}
				if guarded {
					if event.Outcome != telemetry.OutcomeAllow {
						t.Errorf("guarded spawn outcome=%s want allow", event.Outcome)
					}
					if event.ToModel != ids[2] {
						t.Errorf("guarded spawn model=%s want %s", event.ToModel, ids[2])
					}
					if telemetry.Aggregate(events).ReasonCodes["UNSUPPORTED_LANGUAGE"] != 1 {
						t.Error("guard not counted")
					}
					// View folding requires an opaque session; supply one for adapters
					// whose fixture does not carry a native session ID.
					viewEvents := append([]telemetry.Event(nil), events...)
					viewEvents[0].SessionID = "qa-multilang-session"
					if why := telemetry.Why(viewEvents, "qa-multilang-session"); !strings.Contains(why, "UNSUPPORTED_LANGUAGE") {
						t.Errorf("why omits guard: %s", why)
					}
					var detail bytes.Buffer
					telemetry.PrintStats(events, telemetry.StatsOptions{Detail: true}, &detail)
					if !strings.Contains(detail.String(), "UNSUPPORTED_LANGUAGE") {
						t.Errorf("stats omits guard: %s", detail.String())
					}
				}
				logged, err := os.ReadFile(logPath)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Contains(logged, []byte(prompt)) {
					t.Fatal("telemetry persisted prompt")
				}
			}
		})
	}
}

// runQAAdapterFixture drives the real shared runner without a native spawn.
func runQAAdapterFixture(raw []byte, harness string) {
	switch harness {
	case "claude-code":
		runHookAdapter(harness, bytes.NewReader(raw), func(b []byte) (claudecode.Event, error) { var e claudecode.Event; return e, json.Unmarshal(b, &e) }, func(e claudecode.Event) (any, string, core.Decision) { return claudecode.Handle(e, cmdCat) }, printAllow)
	case "cursor":
		runHookAdapter(harness, bytes.NewReader(raw), func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) }, func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, cmdCat) }, printCursorAllow)
	case "codex":
		runHookAdapter(harness, bytes.NewReader(raw), func(b []byte) (codex.Event, error) { var e codex.Event; return e, json.Unmarshal(b, &e) }, func(e codex.Event) (any, string, core.Decision) { return codex.Handle(e, cmdCat) }, printAllow)
	case "kirocrew":
		runHookAdapter(harness, bytes.NewReader(raw), func(b []byte) (kirocrew.Event, error) { var e kirocrew.Event; return e, json.Unmarshal(b, &e) }, func(e kirocrew.Event) (any, string, core.Decision) { return kirocrew.Handle(e, cmdCat) }, policyWire{})
	case "antigravity":
		runHookAdapter(harness, bytes.NewReader(raw), func(b []byte) (antigravity.Event, error) { var e antigravity.Event; return e, json.Unmarshal(b, &e) }, func(e antigravity.Event) (any, string, core.Decision) { return antigravity.Handle(e, cmdCat) }, printAllow)
	case "opencode":
		runHookAdapter(harness, bytes.NewReader(raw), opencode.ParseRequest, func(e opencode.Request) (any, string, core.Decision) { return opencode.Handle(e, cmdCat) }, openCodeWire{})
	}
}
