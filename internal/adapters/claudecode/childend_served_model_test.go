// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// seedDecisionOnly records the rewrite decision for stopAgent's session but,
// unlike seedLaunch, fires no launch. A single PostToolUse launch (the one each
// test sends) then produces exactly one child_ended, so the test reads the
// served model that launch carried rather than a launch seedLaunch fired first.
func seedDecisionOnly(t *testing.T, log, small, frontier string) {
	t.Helper()
	dec := telemetry.Event{
		CorrelationID: "decision-1", Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Harness: "claude-code", Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT",
		Complexity: "TRIVIAL", Tier: "small", FromModel: frontier, ToModel: small,
		SessionID: telemetry.HashSessionID("sess-stop"),
	}
	if err := telemetry.AppendTo(log, dec); err != nil {
		t.Fatal(err)
	}
}

// launchPayload is a PostToolUse async-launch for stopAgent, optionally naming
// a resolved model and a transcript path, the shape Claude Code sends when it
// reports a subagent launch.
func launchPayload(resolved, transcript string) []byte {
	resp := map[string]any{"isAsync": true, "status": "async_launched", "agentId": stopAgent}
	if resolved != "" {
		resp["resolvedModel"] = resolved
	}
	p := map[string]any{
		"hook_event_name": "PostToolUse", "session_id": "sess-stop",
		"tool_name": "Agent", "tool_response": resp,
	}
	if transcript != "" {
		p["agent_transcript_path"] = transcript
	}
	raw, _ := json.Marshal(p)
	return raw
}

// writeFirstAssistantTranscript writes a subagent transcript whose first
// assistant line names model. A leading user line proves the reader skips
// non-assistant rows and that a later assistant model never shadows the first.
func writeFirstAssistantTranscript(t *testing.T, model string) string {
	t.Helper()
	row := func(typ, m string) string {
		r := map[string]any{"type": typ}
		if typ == "assistant" {
			r["message"] = map[string]any{"model": m, "id": "msg_1"}
		}
		b, _ := json.Marshal(r)
		return string(b)
	}
	rows := []string{
		row("user", ""),
		row("assistant", model),
		row("assistant", "claude-opus-5-5"), // a later line must not win
	}
	p := filepath.Join(t.TempDir(), "agent-"+stopAgent+".jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(rows, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestChildEnd_ServedModelFromTranscript(t *testing.T) {
	log, small, frontier, res := stopSetup(t)
	seedDecisionOnly(t, log, small, frontier)

	// A transcript whose first assistant message names a dated id: child_ended
	// records the catalog id, not the dated one, not the resolved, not unknown.
	path := writeFirstAssistantTranscript(t, small+"-20251001")
	claudecode.HandlePostToolUse(launchPayload(frontier, path), "test", res)

	ev := childEndEvents(t)
	if len(ev) != 1 {
		t.Fatalf("child_ended events = %d, want 1", len(ev))
	}
	if ev[0].ToModel != small {
		t.Fatalf("final_model = %q, want the served model %q read from the transcript (not the resolved %q)", ev[0].ToModel, small, frontier)
	}
}

func TestChildEnd_ServedModelFallsBackToResolvedWithoutTranscriptModel(t *testing.T) {
	log, small, frontier, res := stopSetup(t)
	seedDecisionOnly(t, log, small, frontier)

	// A transcript with no assistant model line: the served model is unknown
	// from the transcript, so child_ended falls back to the resolved model.
	rows := []string{`{"type":"user"}`, `{"type":"assistant","message":{"id":"m"}}`}
	path := filepath.Join(t.TempDir(), "agent-"+stopAgent+".jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(rows, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	claudecode.HandlePostToolUse(launchPayload(small, path), "test", res)

	ev := childEndEvents(t)
	if len(ev) != 1 || ev[0].ToModel != small {
		t.Fatalf("final_model = %v, want fallback to resolved %q", ev, small)
	}
}

func TestChildEnd_ServedModelMissingFileKeepsResolved(t *testing.T) {
	log, small, frontier, res := stopSetup(t)
	seedDecisionOnly(t, log, small, frontier)

	// A transcript path that does not exist: reading it must not panic and
	// must not record a model from it; child_ended falls back to the resolved.
	missing := filepath.Join(t.TempDir(), "agent-does-not-exist.jsonl")
	claudecode.HandlePostToolUse(launchPayload(small, missing), "test", res)

	ev := childEndEvents(t)
	if len(ev) != 1 || ev[0].ToModel != small {
		t.Fatalf("final_model = %v, want fallback to resolved %q on a missing transcript", ev, small)
	}
}

func TestChildEnd_ServedModelUnknownWhenNeitherKnown(t *testing.T) {
	log, small, frontier, res := stopSetup(t)
	seedDecisionOnly(t, log, small, frontier)

	// No transcript path and no resolved model on this launch: child_ended
	// keeps "unknown", the only honest value when nothing names the model.
	claudecode.HandlePostToolUse(launchPayload("", ""), "test", res)

	ev := childEndEvents(t)
	if len(ev) != 1 || ev[0].ToModel != "unknown" {
		t.Fatalf("final_model = %v, want \"unknown\" when neither transcript nor resolved names a model", ev)
	}
}
