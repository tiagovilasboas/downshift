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
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

const stopAgent = "agent0123456789ab"

// writeTranscript writes a subagent transcript shaped like Claude Code's: one
// assistant message is repeated across lines while streaming, with usage that
// grows, and only the last line per id counts.
func writeTranscript(t *testing.T, model string) string {
	t.Helper()
	line := func(typ, id string, out int) string {
		row := map[string]any{"type": typ}
		if typ == "assistant" {
			row["message"] = map[string]any{
				"id": id, "model": model,
				"usage": map[string]any{
					"input_tokens": 10, "output_tokens": out,
					"cache_creation_input_tokens": 100, "cache_read_input_tokens": 50,
				},
				"content": []any{map[string]any{"type": "text", "text": "never stored"}},
			}
		}
		b, _ := json.Marshal(row)
		return string(b)
	}
	rows := []string{
		line("user", "", 0),
		line("assistant", "msg_1", 3), line("assistant", "msg_1", 200), // streamed twice
		line("assistant", "msg_2", 40),
	}
	p := filepath.Join(t.TempDir(), "agent-"+stopAgent+".jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(rows, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func stopPayload(path string) []byte {
	b, _ := json.Marshal(map[string]any{
		"hook_event_name": "SubagentStop", "session_id": "sess-stop",
		"agent_id": stopAgent, "agent_transcript_path": path,
	})
	return b
}

func stopSetup(t *testing.T) (log string, small, frontier string, res core.Resolver) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	c := catalog.Load()
	small = c.ModelFor("claude-code", core.TierSmall).ID
	frontier = c.ModelFor("claude-code", core.TierFrontier).ID
	log = filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	return log, small, frontier, c
}

// seedLaunch records the rewrite decision and the launch-time observation that
// ties it to the agent, the way the PreToolUse and PostToolUse hooks do.
func seedLaunch(t *testing.T, log, small, frontier string, res core.Resolver) {
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
	launch, _ := json.Marshal(map[string]any{
		"session_id": "sess-stop", "tool_name": "Agent",
		"tool_response": map[string]any{"isAsync": true, "agentId": stopAgent, "resolvedModel": small + "-20251001"},
	})
	claudecode.HandlePostToolUse(launch, "test", res)
}

func usageEvents(t *testing.T) []telemetry.Event {
	t.Helper()
	all, _ := telemetry.ReadEvents()
	var out []telemetry.Event
	for _, ev := range all {
		if ev.Outcome == telemetry.OutcomeUsage {
			out = append(out, ev)
		}
	}
	return out
}

func TestSubagentStop_PricesFromTranscriptAndLinksToTheDecision(t *testing.T) {
	log, small, frontier, res := stopSetup(t)
	seedLaunch(t, log, small, frontier, res)

	note := claudecode.HandleSubagentStop(stopPayload(writeTranscript(t, small+"-20251001")), "test", res)
	if !strings.Contains(note, "linked") {
		t.Fatalf("note = %q, want a linked decision", note)
	}
	got := usageEvents(t)
	if len(got) != 1 {
		t.Fatalf("usage events = %d, want 1", len(got))
	}
	ev := got[0]
	// msg_1 counts once (last line: 200 output), msg_2 once (40).
	if ev.InputTokens == nil || *ev.InputTokens != 20 || ev.OutputTokens == nil || *ev.OutputTokens != 240 {
		t.Fatalf("tokens in=%v out=%v, want 20/240", ev.InputTokens, ev.OutputTokens)
	}
	if ev.CachedTokens == nil || *ev.CachedTokens != 300 {
		t.Fatalf("cached = %v, want 300", ev.CachedTokens)
	}
	if ev.LinkedDecision != "decision-1" || ev.AgentHash != telemetry.HashSessionID(stopAgent) {
		t.Fatalf("link = %q agent=%q", ev.LinkedDecision, ev.AgentHash)
	}
	if !ev.HasTokenUsage() {
		t.Fatalf("want tokens on the linked usage record, got %+v", ev)
	}
}

// Inline usage (mod turn.complete → end claude-code) prices without a transcript.
func TestSubagentStop_PricesFromInlineUsage(t *testing.T) {
	log, small, frontier, res := stopSetup(t)
	seedLaunch(t, log, small, frontier, res)

	raw, _ := json.Marshal(map[string]any{
		"hook_event_name": "SubagentStop",
		"session_id":      "sess-stop",
		"agent_id":        stopAgent,
		"model":           small + "-20251001",
		"usage": map[string]any{
			"input_tokens":                20,
			"output_tokens":               240,
			"cache_creation_input_tokens": 100,
			"cache_read_input_tokens":     200,
		},
	})
	note := claudecode.HandleSubagentStop(raw, "test", res)
	if !strings.Contains(note, "linked") {
		t.Fatalf("note = %q, want a linked decision", note)
	}
	got := usageEvents(t)
	if len(got) != 1 {
		t.Fatalf("usage events = %d, want 1", len(got))
	}
	ev := got[0]
	if ev.InputTokens == nil || *ev.InputTokens != 20 || ev.OutputTokens == nil || *ev.OutputTokens != 240 {
		t.Fatalf("tokens in=%v out=%v, want 20/240", ev.InputTokens, ev.OutputTokens)
	}
	if ev.CachedTokens == nil || *ev.CachedTokens != 300 {
		t.Fatalf("cached = %v, want 300", ev.CachedTokens)
	}
	if ev.LinkedDecision != "decision-1" {
		t.Fatalf("link = %q", ev.LinkedDecision)
	}
}

// Inline usage wins over a transcript path when both are present.
func TestSubagentStop_InlineUsageBeatsTranscript(t *testing.T) {
	_, small, _, res := stopSetup(t)
	path := writeTranscript(t, small)
	raw, _ := json.Marshal(map[string]any{
		"hook_event_name":       "SubagentStop",
		"agent_id":              stopAgent,
		"agent_transcript_path": path,
		"model":                 "inline-model",
		"usage": map[string]any{
			"input_tokens": 7, "output_tokens": 3,
		},
	})
	claudecode.HandleSubagentStop(raw, "test", res)
	got := usageEvents(t)
	if len(got) != 1 {
		t.Fatalf("usage events = %d, want 1", len(got))
	}
	if got[0].InputTokens == nil || *got[0].InputTokens != 7 {
		t.Fatalf("want inline 7 input tokens, got %+v", got[0].InputTokens)
	}
}

func TestSubagentStop_IsIdempotent(t *testing.T) {
	log, small, frontier, res := stopSetup(t)
	seedLaunch(t, log, small, frontier, res)
	p := writeTranscript(t, small)

	claudecode.HandleSubagentStop(stopPayload(p), "test", res)
	if note := claudecode.HandleSubagentStop(stopPayload(p), "test", res); note != "" {
		t.Fatalf("second stop priced again: %q", note)
	}
	if n := len(usageEvents(t)); n != 1 {
		t.Fatalf("usage events = %d, want 1", n)
	}
}

// A spawn the hook never touched still gets its spend recorded, unlinked and
// priced as unrouted (no invented savings).
func TestSubagentStop_UnlinkedSpawnRecordsSpendWithoutSavings(t *testing.T) {
	_, small, _, res := stopSetup(t)

	note := claudecode.HandleSubagentStop(stopPayload(writeTranscript(t, small)), "test", res)
	if !strings.Contains(note, "without matching decision") {
		t.Fatalf("note = %q", note)
	}
	got := usageEvents(t)
	if len(got) != 1 || got[0].LinkedDecision != "" {
		t.Fatalf("want one unlinked event with zero savings, got %+v", got)
	}
}

func TestSubagentStop_FailsOpenWithoutWriting(t *testing.T) {
	_, small, _, res := stopSetup(t)
	good := writeTranscript(t, small)
	dir := t.TempDir()
	notJSONL := filepath.Join(dir, "agent.txt")
	os.WriteFile(notJSONL, []byte("x"), 0o600)
	empty := filepath.Join(dir, "empty.jsonl")
	os.WriteFile(empty, nil, 0o600)

	for name, raw := range map[string][]byte{
		"not json":       []byte(`{nope`),
		"no agent id":    []byte(`{"agent_transcript_path":"` + good + `"}`),
		"no path":        []byte(`{"agent_id":"` + stopAgent + `"}`),
		"missing file":   stopPayload(filepath.Join(dir, "gone.jsonl")),
		"wrong suffix":   stopPayload(notJSONL),
		"empty":          stopPayload(empty),
		"directory path": stopPayload(dir + ".jsonl"),
	} {
		if note := claudecode.HandleSubagentStop(raw, "test", res); note != "" {
			t.Errorf("%s: note %q, want none", name, note)
		}
	}
	if n := len(usageEvents(t)); n != 0 {
		t.Fatalf("usage events = %d, want 0", n)
	}
}

// writeVerifyTranscript builds a transcript of Bash tool calls. Each step is a
// command plus whether its tool_result was an error.
func writeVerifyTranscript(t *testing.T, steps ...[2]any) string {
	t.Helper()
	var rows []string
	for i, s := range steps {
		id := "toolu_" + string(rune('a'+i))
		use, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{
			"id": "m" + id, "model": "m",
			"usage": map[string]any{"input_tokens": 10, "output_tokens": 5},
			"content": []any{map[string]any{"type": "tool_use", "id": id, "name": "Bash",
				"input": map[string]any{"command": s[0]}}},
		}})
		res, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{
			"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id,
				"is_error": s[1], "content": "never stored"}},
		}})
		rows = append(rows, string(use), string(res))
	}
	p := filepath.Join(t.TempDir(), "agent-"+stopAgent+".jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(rows, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSubagentStop_RecordsVerificationOutcome(t *testing.T) {
	cases := []struct {
		name  string
		steps [][2]any
		want  string
	}{
		{"no check", [][2]any{{"ls -la", false}}, "none"},
		{"passing check", [][2]any{{"go test ./...", false}}, "passed"},
		{"failing check", [][2]any{{"cd x && pytest -q", true}}, "failed"},
		{"fixed then rerun", [][2]any{{"go vet ./...", true}, {"go vet ./...", false}}, "passed"},
		{"unrecognised command is not a check", [][2]any{{"echo go testing", true}}, "none"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log, small, frontier, res := stopSetup(t)
			seedLaunch(t, log, small, frontier, res)
			claudecode.HandleSubagentStop(stopPayload(writeVerifyTranscript(t, tc.steps...)), "test", res)
			got := usageEvents(t)
			if len(got) != 1 {
				t.Fatalf("want 1 usage event, got %d", len(got))
			}
			if got[0].Verification != tc.want {
				t.Fatalf("verification = %q, want %q", got[0].Verification, tc.want)
			}
		})
	}
}

// A held spawn (allow) has no rewrite, but its exact tool_use_id still links
// the launch and the final cost to its decision, without claiming honor.
func TestSubagentStop_HeldSpawnLinksByToolUseID(t *testing.T) {
	log, _, frontier, res := stopSetup(t)
	dec := telemetry.Event{
		CorrelationID: "held-1", Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Harness: "claude-code", Outcome: "allow", Verdict: "DOWNSHIFT", Corrections: []string{"R1_UNCONFIDENT_DOWNSHIFT"},
		Complexity: "MEDIUM", Tier: "mid", FromModel: frontier, ToModel: frontier,
		SessionID: telemetry.HashSessionID("sess-stop"), ToolUseHash: telemetry.HashSessionID("toolu_held"),
	}
	if err := telemetry.AppendTo(log, dec); err != nil {
		t.Fatal(err)
	}
	launch, _ := json.Marshal(map[string]any{
		"session_id": "sess-stop", "tool_name": "Agent", "tool_use_id": "toolu_held",
		"tool_response": map[string]any{"isAsync": true, "status": "async_launched", "agentId": stopAgent, "resolvedModel": frontier},
	})
	claudecode.HandlePostToolUse(launch, "test", res)
	claudecode.HandleSubagentStop(stopPayload(writeTranscript(t, frontier)), "test", res)

	events, _ := telemetry.ReadEventsFrom(log)
	var resolved, usage *telemetry.Event
	for i := range events {
		switch events[i].Outcome {
		case telemetry.OutcomeResolved:
			resolved = &events[i]
		case telemetry.OutcomeUsage:
			usage = &events[i]
		}
	}
	if resolved == nil || resolved.LinkedDecision != "held-1" || resolved.RewriteHonored != nil {
		t.Fatalf("resolved = %+v, want linked to held-1 with no honor claim", resolved)
	}
	if usage == nil || usage.LinkedDecision != "held-1" || usage.ToModel != frontier {
		t.Fatalf("usage = %+v, want linked to held-1 on %s", usage, frontier)
	}
}
