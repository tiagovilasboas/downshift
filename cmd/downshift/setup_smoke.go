// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// eventLogEnv is the telemetry override that the smoke check points at a
// scratch file, so a check never writes a decision to the user's event log.
const eventLogEnv = "DOWNSHIFT_EVENT_LOG"

// smokeSpawns is one real subagent spawn per harness, shaped the way that
// harness's adapter detects a spawn. A payload that is not a spawn gets exit 0
// and no decision, which is exactly the false OK this check must not report.
var smokeSpawns = map[string]string{
	"claude-code": `{"tool_name":"Agent","tool_input":{"prompt":"ping"}}`,
	"cursor":      `{"tool_name":"Task","tool_input":{"task":"ping"}}`,
	"codex":       `{"hook_event_name":"PreToolUse","session_id":"smoke","tool_name":"spawn_agent","tool_input":{"message":"ping","task_name":"ping"}}`,
	"antigravity": `{"toolCall":{"name":"invoke_subagent","args":{"Subagents":[{"Prompt":"ping"}]}}}`,
	"kirocrew":    `{"hook_event_name":"PreToolUse","tool_name":"spawn_run","tool_input":{"model":"claude-sonnet-5-5","task":"ping"}}`,
	"opencode":    `{"v":1,"kind":"route","harness":{"id":"opencode","version":"2.0.32"},"event":"subagent_spawn_intent","correlation":{"child":"0123456789abcdef0123","parent":"fedcba9876543210"},"agent":"smoke","model":{"providerID":"opencode","id":"claude-sonnet-5-5","variant":""},"taskText":"ping","tsUtc":"2026-10-30T00:00:00Z"}`,
}

// smokeResult is one harness's handshake verdict.
type smokeResult struct {
	Elapsed time.Duration
	// Decided is true only when the hook recorded a routing decision for the spawn.
	Decided bool
	// Skipped is true when the harness has no hook or no spawn to send.
	Skipped bool
	// Detail says why the decision is missing, for the operator.
	Detail string
}

func renderTUISmokeCheck(out io.Writer, targets []setupTarget) {
	fmt.Fprintln(out, "Live validation of the registered adapters (<50ms smoke check):")
	cat := catalog.Load()
	for _, t := range targets {
		res := runSmokeCheck(t.Name, cat)
		switch {
		case res.Skipped:
			fmt.Fprintf(out, "  [•] %-14s handshake bypassed (custom/manual adapter)\n", t.Name)
		case res.Decided:
			fmt.Fprintf(out, "  [✔] %-14s handshake OK in %v\n", t.Name, res.Elapsed.Round(time.Microsecond))
		default:
			fmt.Fprintf(out, "  [✖] %-14s handshake not validated: %s\n", t.Name, res.Detail)
		}
	}
	fmt.Fprintln(out)
}

// runSmokeCheck sends one spawn through the harness hook and judges the
// decision the hook recorded. The exit code is not evidence: a hook that fails
// open also exits 0, so only a recorded routing decision counts.
func runSmokeCheck(harness string, cat core.Resolver) smokeResult {
	entry, ok := harnessRegistry[harness]
	payload, hasSpawn := smokeSpawns[harness]
	if !ok || entry.Hook == nil || !hasSpawn {
		return smokeResult{Skipped: true}
	}
	dir, err := os.MkdirTemp("", "downshift-smoke-")
	if err != nil {
		return smokeResult{Detail: "no scratch directory: " + err.Error()}
	}
	defer os.RemoveAll(dir)
	logPath := filepath.Join(dir, "events.jsonl")
	restore := setEnvFor(eventLogEnv, logPath)
	start := time.Now()
	runQuietly(func() { entry.Hook(strings.NewReader(payload), cat) })
	elapsed := time.Since(start)
	restore()
	return judgeSmoke(elapsed, readSmokeEvents(logPath))
}

// judgeSmoke decides from the hook's own events: any failure record means the
// hook gave up, and a hook-sourced event that is not a failure is a decision.
func judgeSmoke(elapsed time.Duration, events []telemetry.Event) smokeResult {
	decided := false
	for _, ev := range events {
		if ev.Source != "hook" {
			continue
		}
		if telemetry.IsFailureOutcome(ev.Outcome) {
			return smokeResult{Elapsed: elapsed, Detail: fmt.Sprintf("hook failed open (%s)", ev.ErrorCode)}
		}
		decided = true
	}
	if !decided {
		return smokeResult{Elapsed: elapsed, Detail: "no routing decision recorded for the spawn"}
	}
	return smokeResult{Elapsed: elapsed, Decided: true}
}

func readSmokeEvents(path string) []telemetry.Event {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var events []telemetry.Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev telemetry.Event
		if json.Unmarshal(sc.Bytes(), &ev) == nil {
			events = append(events, ev)
		}
	}
	return events
}

// setEnvFor sets key to value and returns the function that restores it.
func setEnvFor(key, value string) func() {
	prev, had := os.LookupEnv(key)
	_ = os.Setenv(key, value)
	return func() {
		if had {
			_ = os.Setenv(key, prev)
			return
		}
		_ = os.Unsetenv(key)
	}
}

func runQuietly(fn func()) {
	r, w, err := os.Pipe()
	if err != nil {
		fn()
		return
	}
	oldOut := os.Stdout
	oldErr := os.Stderr
	os.Stdout = w
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr
	_, _ = io.Copy(io.Discard, r)
	_ = r.Close()
}
