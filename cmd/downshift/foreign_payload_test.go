// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Cursor runs the Claude Code hooks in ~/.claude/settings.json when
// third-party configs are on, so one Cursor spawn reaches both the cursor hook
// and the claude-code hook. In 30 days of real use every one of the 79
// claude-code decisions labelled agent_type "custom" sat within 3 s of a cursor
// decision, and all 33 that carried a tool-call id shared it with one: the
// copy was routed and counted a second time, under the wrong harness, with
// Cursor's model "inherit" as an unknown baseline. The shared runners now
// leave a copy that carries another harness's payload markers to that
// harness's own hook: allowed unchanged, nothing recorded.
func TestSharedRunners_LeaveForeignPayloadToItsHarness(t *testing.T) {
	const cursorTask = `"tool_name":"Task","tool_use_id":"toolu_dup","tool_input":{"prompt":"rename the userId variable","description":"rename","model":"inherit","subagent_type":"explore"}`
	cases := []struct {
		name    string
		harness string
		run     func(harness string, payload string) int
		payload string
		// recorded is true when the payload is the harness's own and must be handled.
		recorded bool
	}{
		{"cursor copy at the claude-code hook", "claude-code", runHookFor,
			`{"hook_event_name":"preToolUse","conversation_id":"c1","generation_id":"g1",` + cursorTask + `}`, false},
		{"cursor copy with cursor_version only", "claude-code", runHookFor,
			`{"hook_event_name":"preToolUse","cursor_version":"3.13.0",` + cursorTask + `}`, false},
		{"cursor end copy at the claude-code end hook", "claude-code", runEndRunner,
			`{"hook_event_name":"subagentStop","workspace_roots":["/w"],"session_id":"s1","agent_id":"a1","usage":{"input_tokens":10}}`, false},
		{"cursor post copy at the claude-code honor hook", "claude-code", runHonorRunner,
			`{"hook_event_name":"postToolUse","generation_id":"g1","tool_name":"Task","tool_use_id":"toolu_dup","session_id":"s1","tool_response":{"agentId":"a1","resolvedModel":"claude-haiku-4-5"}}`, false},
		{"cursor payload at the cursor hook", "cursor", runHookFor,
			`{"hook_event_name":"preToolUse","conversation_id":"c1","generation_id":"g1",` + cursorTask + `}`, true},
		{"claude-code payload at the claude-code hook", "claude-code", runHookFor,
			`{"hook_event_name":"PreToolUse","tool_name":"Agent","session_id":"s1","tool_use_id":"toolu_1","tool_input":{"prompt":"rename the userId variable","subagent_type":"general-purpose"}}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			eventLog := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
			var rc int
			var stdout, stderr string
			stdout = captureStdout(func() {
				stderr = captureStderr(func() { rc = tc.run(tc.harness, tc.payload) })
			})
			if rc != 0 {
				t.Fatalf("rc = %d, want 0 (fail open)", rc)
			}
			events, _ := telemetry.ReadEventsFrom(eventLog)
			if tc.recorded {
				if len(events) == 0 {
					t.Fatalf("own payload recorded nothing; stderr:\n%s", stderr)
				}
				return
			}
			if len(events) != 0 {
				t.Fatalf("foreign copy recorded %d events: %+v", len(events), events)
			}
			if !strings.Contains(stderr, "cursor payload reached the claude-code hook") {
				t.Errorf("stderr lacks the foreign-payload note:\n%s", stderr)
			}
			if strings.Contains(stdout, `"model"`) {
				t.Errorf("foreign copy was rewritten: %s", stdout)
			}
		})
	}
}

func TestForeignPayloadOwner_UsesDeclaredMarkersOnly(t *testing.T) {
	has := func(keys ...string) func(string) bool {
		return func(k string) bool {
			for _, want := range keys {
				if k == want {
					return true
				}
			}
			return false
		}
	}
	cases := []struct {
		harness string
		keys    []string
		want    string
	}{
		{"claude-code", []string{"generation_id"}, "cursor"},
		{"codex", []string{"cursor_version"}, "cursor"},
		{"cursor", []string{"cursor_version", "workspace_roots"}, ""},
		{"claude-code", []string{"session_id", "tool_use_id", "transcript_path", "cwd"}, ""},
		{"claude-code", nil, ""},
	}
	for _, tc := range cases {
		if got := core.ForeignPayloadOwner(tc.harness, has(tc.keys...)); got != tc.want {
			t.Errorf("ForeignPayloadOwner(%s, %v) = %q, want %q", tc.harness, tc.keys, got, tc.want)
		}
	}
}

func runHookFor(harness, payload string) int {
	return harnessRegistry[harness].Hook(strings.NewReader(payload), cmdCat)
}

func runEndRunner(harness, payload string) int {
	return runEnd(bytes.NewReader([]byte(payload)), portFor(harness), cmdCat)
}

func runHonorRunner(harness, payload string) int {
	return runHonor(bytes.NewReader([]byte(payload)), portFor(harness), cmdCat)
}
