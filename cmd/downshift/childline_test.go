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
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

const childSession = "sess-childline"
const childAgent = "agent-childline-1"

// childLineEnv isolates state and returns the small and frontier model ids.
func childLineEnv(t *testing.T) (log, small, frontier string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_QUIET", "")
	log = filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	small = cmdCat.ModelFor("claude-code", core.TierSmall).ID
	mid := cmdCat.ModelFor("claude-code", core.TierMid).ID
	frontier = cmdCat.ModelFor("claude-code", core.TierFrontier).ID
	setSessionAllowlist(t, "claude-code", []string{small, mid, frontier})
	return log, small, frontier
}

// spawn runs the real PreToolUse hook for a trivial task requested on the
// frontier model, then the launch PostToolUse (honor) runner, like a session.
func spawn(t *testing.T, small, frontier string) (preOut string) {
	t.Helper()
	pre, _ := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse", "tool_name": "Agent", "session_id": childSession,
		"tool_use_id": "toolu_childline",
		"tool_input":  map[string]any{"prompt": "rename the variable userId to userIdentifier", "model": frontier},
	})
	preOut = captureStdout(func() {
		runHookAdapter(
			"claude-code",
			bytes.NewReader(pre),
			func(b []byte) (claudecode.Event, error) { var e claudecode.Event; return e, json.Unmarshal(b, &e) },
			func(e claudecode.Event) (any, string, core.Decision) { return claudecode.Handle(e, cmdCat) },
			printAllow,
		)
	})
	launch, _ := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse", "tool_name": "Agent", "session_id": childSession,
		"tool_use_id": "toolu_childline",
		"tool_response": map[string]any{
			"isAsync": true, "status": "async_launched", "agentId": childAgent, "resolvedModel": small,
		},
	})
	if out := captureStdout(func() { runHonor(bytes.NewReader(launch), portFor("claude-code"), cmdCat) }); strings.TrimSpace(out) != "{}" {
		t.Fatalf("an async launch must not show a child-end line, got %q", out)
	}
	return preOut
}

// childTranscript writes a subagent transcript of in+out tokens on model.
func childTranscript(t *testing.T, model string, in, out int) string {
	t.Helper()
	row, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{
		"id": "msg_1", "model": model,
		"usage": map[string]any{"input_tokens": in, "output_tokens": out},
	}})
	p := filepath.Join(t.TempDir(), "agent-"+childAgent+".jsonl")
	if err := os.WriteFile(p, append(row, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func stopRaw(transcript string) []byte {
	b, _ := json.Marshal(map[string]any{
		"hook_event_name": "SubagentStop", "session_id": childSession,
		"agent_id": childAgent, "agent_transcript_path": transcript,
	})
	return b
}

func runStop(raw []byte) (stdout string, rc int) {
	stdout = captureStdout(func() { rc = runUsage(bytes.NewReader(raw), portFor("claude-code"), cmdCat) })
	return stdout, rc
}

func systemMessage(t *testing.T, stdout string) string {
	t.Helper()
	var out struct {
		SystemMessage string `json:"systemMessage"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out); err != nil {
		t.Fatalf("stdout %q is not JSON: %v", stdout, err)
	}
	return out.SystemMessage
}

func TestChildLine_SubagentStopFullData(t *testing.T) {
	_, small, frontier := childLineEnv(t)
	spawn(t, small, frontier)

	stdout, rc := runStop(stopRaw(childTranscript(t, small, 300000, 20000)))
	if rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	msg := systemMessage(t, stdout)
	t.Logf("child-end line: %s", msg)
	for _, want := range []string{"↳ downshift: finished on haiku", "320k tokens", "saved from opus"} {
		if !strings.Contains(msg, want) {
			t.Errorf("line %q lacks %q", msg, want)
		}
	}
	if strings.Contains(msg, "$") {
		t.Errorf("the live child-end line carries a dollar figure: %q", msg)
	}
	if strings.Contains(msg, "\n") {
		t.Errorf("line must be one line: %q", msg)
	}
	// Idempotent: a second SubagentStop for the same agent shows nothing.
	if again, _ := runStop(stopRaw(childTranscript(t, small, 12000, 2000))); again != "" {
		t.Errorf("second stop printed %q", again)
	}
}

func TestChildLine_PartialDataOmitsWhatIsUnknown(t *testing.T) {
	_, small, _ := childLineEnv(t)
	// No decision was recorded: no requested model, so no saving is claimed.
	stdout, _ := runStop(stopRaw(childTranscript(t, small, 12000, 2000)))
	msg := systemMessage(t, stdout)
	t.Logf("partial line: %s", msg)
	if msg != "↳ downshift: finished on haiku · 14k tokens (work 14k, cache 0)" {
		t.Errorf("partial line = %q", msg)
	}
}

func TestChildLine_QuietSilencesChildAndSpawnMessage(t *testing.T) {
	_, small, frontier := childLineEnv(t)
	loud := spawn(t, small, frontier)
	if !strings.Contains(loud, "systemMessage") {
		t.Fatalf("expected a spawn message without DOWNSHIFT_QUIET: %s", loud)
	}

	_, small, frontier = childLineEnv(t)
	t.Setenv("DOWNSHIFT_QUIET", "1")
	quiet := spawn(t, small, frontier)
	if strings.Contains(quiet, "systemMessage") || !strings.Contains(quiet, "updatedInput") {
		t.Fatalf("quiet must drop only the message, keep the rewrite: %s", quiet)
	}
	if stdout, _ := runStop(stopRaw(childTranscript(t, small, 12000, 2000))); stdout != "" {
		t.Errorf("quiet printed a child line: %q", stdout)
	}
	// The cost is still recorded.
	events, _ := telemetry.ReadEvents()
	n := 0
	for _, ev := range events {
		if ev.Outcome == telemetry.OutcomeUsage {
			n++
		}
	}
	if n != 1 {
		t.Errorf("usage events = %d, want 1 even when quiet", n)
	}
}

func TestChildLine_FailurePathsPrintNothingAndExitZero(t *testing.T) {
	childLineEnv(t)
	for name, raw := range map[string][]byte{
		"garbage":            []byte("not json"),
		"empty":              nil,
		"missing transcript": stopRaw(filepath.Join(t.TempDir(), "gone.jsonl")),
		"wrong extension":    stopRaw(filepath.Join(t.TempDir(), "x.txt")),
	} {
		stdout, rc := runStop(raw)
		if rc != 0 || stdout != "" {
			t.Errorf("%s: rc=%d stdout=%q, want 0 and no output", name, rc, stdout)
		}
	}
}

func TestChildLine_PostToolUseForegroundChild(t *testing.T) {
	_, small, _ := childLineEnv(t)
	post := func(tool string) string {
		raw, _ := json.Marshal(map[string]any{
			"hook_event_name": "PostToolUse", "tool_name": tool, "session_id": childSession,
			"model": small, "usage": map[string]any{"input_tokens": 3000, "output_tokens": 500},
			"tool_response": map[string]any{"status": "completed", "agentId": childAgent},
		})
		return captureStdout(func() { runHonor(bytes.NewReader(raw), portFor("claude-code"), cmdCat) })
	}
	if out := post("Bash"); strings.Contains(out, "systemMessage") {
		t.Errorf("a non-Agent tool got a child line: %s", out)
	}
	msg := systemMessage(t, post("Agent"))
	t.Logf("post-tool-use line: %s", msg)
	if !strings.HasPrefix(msg, "↳ downshift: finished on haiku · 3.5k tokens (work 3.5k, cache 0)") {
		t.Errorf("line = %q", msg)
	}
	// SubagentStop for the same child must not announce or price it again.
	if stdout, _ := runStop(stopRaw(childTranscript(t, small, 3000, 500))); stdout != "" {
		t.Errorf("stop repeated the line: %q", stdout)
	}
}

func TestChildLine_CapabilityIsDeclaredAsData(t *testing.T) {
	for _, n := range core.AllHarnessCaps() {
		if want := n.Harness == "claude-code"; n.Caps.CanShowChildLine != want {
			t.Errorf("%s CanShowChildLine = %v, want %v", n.Harness, n.Caps.CanShowChildLine, want)
		}
	}
}
