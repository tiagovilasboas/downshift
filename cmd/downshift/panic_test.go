// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/telemetry"
)

// A panic inside an adapter must fail open: exit 0, exactly one fail-open
// response on stdout, and an error event. Exit 2 would block the spawn.
func TestRunHookAdapter_PanicFailsOpen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", path)
	var rc int
	out := captureStdout(func() {
		captureStderr(func() {
			rc = runHookAdapter(
				strings.NewReader(`{"tool_name":"Task","tool_input":{"prompt":"x"}}`),
				func(b []byte) (claudecode.Event, error) { var e claudecode.Event; return e, json.Unmarshal(b, &e) },
				func(claudecode.Event) (any, string, core.Decision) { panic("boom") },
				printAllow,
			)
		})
	})
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if n := strings.Count(out, `"permissionDecision":"allow"`); n != 1 {
		t.Fatalf("want exactly one fail-open response, got %d:\n%s", n, out)
	}
	events, _ := telemetry.ReadEventsFrom(path)
	if len(events) != 1 || events[0].ErrorCode != "PANIC" {
		t.Fatalf("want one PANIC error event, got %#v", events)
	}
}

// A panic after the response was written must not print a second response.
func TestRunHookAdapter_PanicAfterResponseDoesNotRespondTwice(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	var rc int
	out := captureStdout(func() {
		captureStderr(func() {
			rc = runHookAdapter(
				strings.NewReader(`{"tool_name":"Task","tool_input":{"prompt":"x"}}`),
				func(b []byte) (claudecode.Event, error) { var e claudecode.Event; return e, json.Unmarshal(b, &e) },
				func(claudecode.Event) (any, string, core.Decision) {
					return claudecode.Output{}, "", core.Decision{Harness: "claude-code"}
				},
				printAllow,
				func(claudecode.Event) string { panic("late boom") },
			)
		})
	})
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if n := strings.Count(out, "\n"); n != 1 {
		t.Fatalf("want exactly one response line, got %d:\n%s", n, out)
	}
}

// panicResolver makes core.Route panic inside the KiroCrew runner.
type panicResolver struct{ core.Resolver }

func (panicResolver) ModelFor(string, core.Tier) core.Model { panic("resolver boom") }

func TestRunKiroCrewHook_PanicFailsOpen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	var rc int
	captureStderr(func() {
		rc = runKiroCrewHook(strings.NewReader(`{"tool_name":"spawn_run","tool_input":{"task":"rename x","model":"claude-opus-5-5"}}`), panicResolver{cmdCat})
	})
	if rc != 0 {
		t.Fatalf("rc = %d, want 0 (exit 2 would block the spawn)", rc)
	}
}
