// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

var smokeHarnesses = []string{"claude-code", "cursor", "codex", "antigravity", "kirocrew", "opencode"}

func TestSmokeCheckAllHarnesses(t *testing.T) {
	for _, h := range smokeHarnesses {
		t.Run(h, func(t *testing.T) {
			res := runSmokeCheck(h, cmdCat)
			if !res.Decided {
				t.Fatalf("runSmokeCheck(%s) not decided: %s", h, res.Detail)
			}
			if res.Elapsed > 500*time.Millisecond {
				t.Errorf("runSmokeCheck(%s) took %v, want < 500ms", h, res.Elapsed)
			}
		})
	}
}

func TestSmokeCheckLeavesUserEventLogUntouched(t *testing.T) {
	userLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv(eventLogEnv, userLog)
	for _, h := range smokeHarnesses {
		runSmokeCheck(h, cmdCat)
	}
	if _, err := os.Stat(userLog); !os.IsNotExist(err) {
		t.Fatalf("smoke check wrote to the user event log: stat err = %v", err)
	}
}

func TestSmokeCheckRejectsFailOpenHook(t *testing.T) {
	saved := smokeSpawns["claude-code"]
	smokeSpawns["claude-code"] = "not json"
	t.Cleanup(func() { smokeSpawns["claude-code"] = saved })

	res := runSmokeCheck("claude-code", cmdCat)
	if res.Decided {
		t.Fatal("a hook that failed open and exited 0 was reported as a validated handshake")
	}
	if !strings.Contains(res.Detail, "failed open") {
		t.Errorf("Detail = %q, want it to name the fail-open", res.Detail)
	}
}

func TestJudgeSmoke(t *testing.T) {
	hookAllow := telemetry.Event{Source: "hook", Outcome: telemetry.OutcomeAllow}
	hookFailOpen := telemetry.Event{Source: "hook", Outcome: telemetry.OutcomeFailOpen, ErrorCode: "PANIC"}
	hookError := telemetry.Event{Source: "hook", Outcome: "error", ErrorCode: "INVALID_EVENT"}
	cliAllow := telemetry.Event{Source: "cli", Outcome: telemetry.OutcomeAllow}

	cases := []struct {
		name    string
		events  []telemetry.Event
		decided bool
	}{
		{"no events", nil, false},
		{"only a cli event", []telemetry.Event{cliAllow}, false},
		{"fail-open alone", []telemetry.Event{hookFailOpen}, false},
		{"error alone", []telemetry.Event{hookError}, false},
		{"decision", []telemetry.Event{hookAllow}, true},
		{"decision before a failure still fails", []telemetry.Event{hookAllow, hookError}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := judgeSmoke(time.Millisecond, tc.events)
			if res.Decided != tc.decided {
				t.Errorf("Decided = %v, want %v (detail %q)", res.Decided, tc.decided, res.Detail)
			}
		})
	}
}
