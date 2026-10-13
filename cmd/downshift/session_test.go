// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

const pauseSession = "sess-pause"

// sessionHook runs the real Claude Code PreToolUse hook for a trivial task
// requested on the frontier model and returns what it wrote to stdout.
func sessionHook(t *testing.T, session, frontier string) string {
	t.Helper()
	pre, _ := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse", "tool_name": "Agent", "session_id": session,
		"tool_use_id": "toolu_" + session,
		"tool_input":  map[string]any{"prompt": "rename the variable userId to userIdentifier", "model": frontier},
	})
	return captureStdout(func() {
		runHookAdapter(
			bytes.NewReader(pre),
			func(b []byte) (claudecode.Event, error) { var e claudecode.Event; return e, json.Unmarshal(b, &e) },
			func(e claudecode.Event) (any, string, core.Decision) { return claudecode.Handle(e, cmdCat) },
			printAllow,
		)
	})
}

func sessionCmd(t *testing.T, args ...string) (stdout, stderr string, rc int) {
	t.Helper()
	var o, e bytes.Buffer
	rc = runSession(args, &o, &e, cmdCat)
	return o.String(), e.String(), rc
}

func TestSessionPause_IsAControlGroupThroughTheSharedRunner(t *testing.T) {
	log, _, frontier := childLineEnv(t)
	state := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", state)
	hash := telemetry.HashSessionID(pauseSession)

	if out := sessionHook(t, pauseSession, frontier); !strings.Contains(out, "updatedInput") {
		t.Fatalf("unpaused, the trivial task must be rewritten, got %s", out)
	}
	if out, _, rc := sessionCmd(t, "--pause", "--session="+pauseSession); rc != 0 || !strings.Contains(out, "paused") {
		t.Fatalf("pause: rc=%d %q", rc, out)
	}
	if out := sessionHook(t, pauseSession, frontier); strings.Contains(out, "updatedInput") {
		t.Fatalf("paused: the spawn must go through unchanged, got %s", out)
	}
	// Another session is not paused.
	if out := sessionHook(t, "sess-other", frontier); !strings.Contains(out, "updatedInput") {
		t.Fatalf("pause leaked into another session: %s", out)
	}
	events, _ := telemetry.ReadEventsFrom(log)
	var paused *telemetry.Event
	for i := range events {
		if events[i].Outcome == telemetry.OutcomeBaseline {
			paused = &events[i]
		}
	}
	if paused == nil || paused.SessionID != hash {
		t.Fatalf("a paused spawn is logged as a baseline decision of its session: %+v", events)
	}
	if !contains(paused.ReasonCodes, telemetry.ReasonPaused) || paused.Complexity == "" || paused.RecommendedModel == "" {
		t.Errorf("it is still classified, with reason PAUSED: %+v", paused)
	}
	if paused.ToModel != paused.FromModel {
		t.Errorf("final model must be the model that ran: %s vs %s", paused.ToModel, paused.FromModel)
	}

	// An expired marker no longer pauses; resume removes it.
	marker := filepath.Join(state, "paused", hash)
	old := time.Now().Add(-13 * time.Hour)
	if err := os.Chtimes(marker, old, old); err != nil {
		t.Fatal(err)
	}
	if out := sessionHook(t, pauseSession, frontier); !strings.Contains(out, "updatedInput") {
		t.Fatalf("a 13h-old pause must have expired: %s", out)
	}
	sessionCmd(t, "--pause", "--session="+pauseSession)
	if out, _, rc := sessionCmd(t, "--resume", "--session="+pauseSession); rc != 0 || !strings.Contains(out, "resumed") {
		t.Fatalf("resume: rc=%d %q", rc, out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("resume must remove the marker: %v", err)
	}
	if out := sessionHook(t, pauseSession, frontier); !strings.Contains(out, "updatedInput") {
		t.Fatalf("resumed, routing is back: %s", out)
	}
}

func TestSessionPause_UnreadableStateFailsOpen(t *testing.T) {
	_, _, frontier := childLineEnv(t)
	state := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", state)
	// "paused" is a file, not a directory: every marker stat fails with ENOTDIR.
	if err := os.WriteFile(filepath.Join(state, "paused"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if out := sessionHook(t, pauseSession, frontier); !strings.Contains(out, "updatedInput") {
		t.Fatalf("a failing marker check must not stop routing: %s", out)
	}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func seedTreeLog(t *testing.T) (log, sessA, sessB string) {
	t.Helper()
	log = filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	sessA, sessB = "sess-a", "sess-b"
	ha, hb := telemetry.HashSessionID(sessA), telemetry.HashSessionID(sessB)
	now := time.Now().UTC()
	ts := func(d time.Duration) string { return now.Add(d).Format(time.RFC3339Nano) }
	cost, base := 0.22, 0.60
	for _, ev := range []telemetry.Event{
		{CorrelationID: "a1", Timestamp: ts(-3 * time.Minute), Harness: "claude-code", SessionID: ha, Outcome: telemetry.OutcomeRewriteEmitted, Complexity: "MEDIUM", FromModel: "claude-opus-5-5", ToModel: "claude-sonnet-5-5", Tier: "mid", Verdict: "DOWNSHIFT", ReasonCodes: []string{telemetry.ReasonClassifierConfident}},
		{CorrelationID: "au", Timestamp: ts(-2 * time.Minute), Harness: "claude-code", SessionID: ha, Outcome: telemetry.OutcomeUsage, LinkedDecision: "a1", ToModel: "claude-sonnet-5-5", ActualCostUSD: &cost, BaselineCostUSD: &base},
		{CorrelationID: "b1", Timestamp: ts(-time.Minute), Harness: "claude-code", SessionID: hb, Outcome: telemetry.OutcomeAllow, Complexity: "COMPLEX", FromModel: "claude-opus-5-5", ToModel: "claude-opus-5-5", Tier: "frontier", Verdict: "UPSHIFT"},
	} {
		if err := telemetry.AppendTo(log, ev); err != nil {
			t.Fatal(err)
		}
	}
	return log, sessA, sessB
}

func TestSession_SelectionExplicitThenEnvThenLatest(t *testing.T) {
	_, a, _ := seedTreeLog(t)
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	const header = "session · "

	// Latest decision in the log is session b: one subagent, unpriced.
	out, _, rc := sessionCmd(t)
	if rc != 0 || !strings.HasPrefix(out, header+"1 subagent · cost not observed") || !strings.Contains(out, "opus") {
		t.Fatalf("latest session: rc=%d\n%s", rc, out)
	}
	// The harness's declared env var beats "latest" (read from capabilities, no name in code).
	var envName string
	for _, n := range core.AllHarnessCaps() {
		if n.Caps.SessionEnv != "" {
			envName = n.Caps.SessionEnv
		}
	}
	if envName == "" {
		t.Fatal("no harness declares SessionEnv")
	}
	t.Setenv(envName, a)
	if out, _, _ := sessionCmd(t); !strings.Contains(out, "$0.22 (without downshift: $0.60) · saved $0.38") {
		t.Fatalf("env session:\n%s", out)
	}
	// --session beats the env, and accepts a hash prefix as printed in --json.
	prefix := telemetry.HashSessionID("sess-b")[:8]
	if out, _, _ := sessionCmd(t, "--session="+prefix); !strings.Contains(out, "cost not observed") {
		t.Fatalf("prefix session:\n%s", out)
	}
	if _, _, rc := sessionCmd(t, "--session=bad id with spaces"); rc != 2 {
		t.Fatalf("an invalid id is a usage error, rc=%d", rc)
	}
	if _, _, rc := sessionCmd(t, "--tree", "--why"); rc != 2 {
		t.Fatalf("two modes is a usage error, rc=%d", rc)
	}
}

func TestSession_TreeWhyAndJSONWriteNothing(t *testing.T) {
	_, a, _ := seedTreeLog(t)
	state := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", state)
	tree, _, _ := sessionCmd(t, "--session="+a)
	if !strings.Contains(tree, "└─ subagent 1  sonnet  ✓  $0.22  MEDIUM, confident") {
		t.Fatalf("tree:\n%s", tree)
	}
	why, _, _ := sessionCmd(t, "--why", "--session="+a)
	if !strings.Contains(why, "CLASSIFIER_CONFIDENT") || !strings.Contains(why, "asked for   opus") {
		t.Fatalf("why:\n%s", why)
	}
	js, _, _ := sessionCmd(t, "--json", "--session="+a)
	var view struct {
		Subagents int `json:"subagents"`
		Rows      []struct {
			Label    string `json:"label"`
			Status   string `json:"status"`
			CostKind string `json:"cost_kind"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(js), &view); err != nil || view.Subagents != 1 || view.Rows[0].Label != "subagent 1" || view.Rows[0].CostKind != "real" {
		t.Fatalf("json: %v\n%s", err, js)
	}
	if entries, _ := os.ReadDir(state); len(entries) != 0 {
		t.Fatalf("reading the session must not write to the state dir: %v", entries)
	}
	// Missing log: an empty view, not an error.
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "none.jsonl"))
	if out, _, rc := sessionCmd(t, "--session="+a); rc != 0 || !strings.Contains(out, "no subagents yet") {
		t.Fatalf("missing log: rc=%d %q", rc, out)
	}
}

func TestSession_FastOnALargeLog(t *testing.T) {
	if testing.Short() {
		t.Skip("writes a 50 MB log")
	}
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	f, err := os.Create(log)
	if err != nil {
		t.Fatal(err)
	}
	w := bufio.NewWriter(f)
	hash := telemetry.HashSessionID("sess-big")
	written := 0
	for i := 0; written < 50<<20; i++ {
		line := fmt.Sprintf(`{"correlation_id":"%032x","timestamp":"2026-10-13T08:%02d:%02d.000000Z","source":"hook","agent":"subagent","harness":"claude-code","session_id":"%s","complexity":"MEDIUM","requested_model":"claude-opus-5-5","final_model":"claude-sonnet-5-5","verdict":"DOWNSHIFT","tier":"mid","outcome":"rewrite_emitted","reason_codes":["CLASSIFIER_CONFIDENT"],"policy_version":"core-route-v1","binary_version":"dev","padding":"%s"}`+"\n",
			i, (i/60)%60, i%60, hash, strings.Repeat("x", 150))
		n, _ := w.WriteString(line)
		written += n
	}
	w.Flush()
	f.Close()

	best := time.Hour
	for i := 0; i < 5; i++ {
		start := time.Now()
		out, _, rc := sessionCmd(t, "--session=sess-big")
		if d := time.Since(start); d < best {
			best = d
		}
		if rc != 0 || !strings.HasPrefix(out, "session · ") {
			t.Fatalf("rc=%d out=%q", rc, out[:min(len(out), 80)])
		}
	}
	t.Logf("session tree on a 50 MB log: %v", best)
	// Locally this is ~25ms; decoding the whole 50 MB log takes ~400ms. 200ms
	// leaves shared-runner headroom and still catches a full read.
	if best > 200*time.Millisecond {
		t.Fatalf("session tree took %v on a 50 MB log, want < 200ms (it must read only the tail)", best)
	}
}
