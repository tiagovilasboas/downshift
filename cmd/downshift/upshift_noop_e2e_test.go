// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
)

// An UPSHIFT whose only reachable target is the requested model is not a
// rewrite. Before the fix the hook wrote "model":"sonnet" over "sonnet" and the
// event said rewrite_emitted, so every such spawn counted as an applied upshift
// while the child ran on the requested model. The spawn must go out unchanged,
// be recorded as allow, and final_model must be what the hook emitted.
func TestHookE2E_ClaudeCodeUpshiftToRequestedModelIsNotRewritten(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierFrontier))
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")
	const task = "Rearchitect the payment flow across services, migrate the auth tokens and redesign the webhook idempotency guarantees for the distributed system"

	for _, alias := range []string{"haiku", "sonnet"} {
		if err := os.WriteFile(eventLog, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		stdin, _ := json.Marshal(map[string]any{
			"hook_event_name": "PreToolUse", "tool_name": "Agent",
			"tool_input": map[string]any{"subagent_type": "general-purpose", "prompt": task, "model": alias},
		})
		stdout := captureStdout(func() {
			runHookAdapter(
				"claude-code",
				bytes.NewReader(stdin),
				func(b []byte) (claudecode.Event, error) { var e claudecode.Event; return e, json.Unmarshal(b, &e) },
				func(e claudecode.Event) (any, string, core.Decision) { return claudecode.Handle(e, cmdCat) },
				printAllow,
			)
		})
		if strings.Contains(stdout, "updatedInput") {
			t.Errorf("%s: hook emitted a rewrite that keeps the requested model: %s", alias, stdout)
		}
		raw, err := os.ReadFile(eventLog)
		if err != nil {
			t.Fatal(err)
		}
		var ev map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(raw), &ev); err != nil {
			t.Fatalf("%s: one event expected, got %q", alias, raw)
		}
		if ev["verdict"] != "UPSHIFT" {
			t.Fatalf("%s: want an UPSHIFT verdict to exercise the path, got %v", alias, ev["verdict"])
		}
		if ev["outcome"] == "rewrite_emitted" {
			t.Errorf("%s: outcome rewrite_emitted for a spawn the hook left on %v", alias, ev["requested_model"])
		}
		if ev["final_model"] != ev["requested_model"] {
			t.Errorf("%s: final_model %v, want the emitted (requested) model %v", alias, ev["final_model"], ev["requested_model"])
		}
	}
}
