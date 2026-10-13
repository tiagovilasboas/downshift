// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// endEnv isolates state for the shared end runner.
func endEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_QUIET", "")
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
}

// runEndFor runs `downshift end <harness>` on a payload and returns rc, stdout
// and stderr.
func runEndFor(harness string, payload []byte) (rc int, stdout, stderr string) {
	stdout = captureStdout(func() {
		stderr = captureStderr(func() {
			rc = runEnd(bytes.NewReader(payload), portFor(harness), cmdCat)
		})
	})
	return
}

func endMarshal(m map[string]any) []byte { b, _ := json.Marshal(m); return b }

// seedDecision records the PreToolUse decision a spawn would have produced.
func seedDecision(t *testing.T, harness, session, call, from, to, outcome string) telemetry.Event {
	t.Helper()
	ev := telemetry.Event{
		CorrelationID: "decision-" + harness, Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Harness: harness, Outcome: outcome, Verdict: "DOWNSHIFT", Complexity: "TRIVIAL", Tier: "small",
		FromModel: from, ToModel: to,
		SessionID: telemetry.HashSessionID(session), ToolUseHash: telemetry.HashSessionID(call),
	}
	if err := telemetry.AppendTo(os.Getenv("DOWNSHIFT_EVENT_LOG"), ev); err != nil {
		t.Fatal(err)
	}
	return ev
}

func eventsOf(t *testing.T, outcome string) []telemetry.Event {
	t.Helper()
	all, _ := telemetry.ReadEvents()
	var out []telemetry.Event
	for _, ev := range all {
		if ev.Outcome == outcome {
			out = append(out, ev)
		}
	}
	return out
}

// codexRollout writes a Codex rollout shaped like the documented
// event_msg/token_count events: cumulative info.total_token_usage, input
// including the cached part. A rate-limit-only event must not count as usage.
func codexRollout(t *testing.T, in, cached, out int) string {
	t.Helper()
	lines := []map[string]any{
		{"type": "turn_context", "payload": map[string]any{"model": "gpt-5.6-sol"}},
		{"type": "event_msg", "payload": map[string]any{"type": "token_count", "info": map[string]any{"total_token_usage": map[string]any{
			"input_tokens": 100, "cached_input_tokens": 0, "output_tokens": 10}}}},
		{"type": "event_msg", "payload": map[string]any{"type": "token_count", "info": map[string]any{"total_token_usage": map[string]any{
			"input_tokens": in, "cached_input_tokens": cached, "output_tokens": out}}}},
		{"type": "event_msg", "payload": map[string]any{"type": "token_count", "rate_limits": map[string]any{"limit_id": "codex"}}},
	}
	var b bytes.Buffer
	for _, l := range lines {
		row, _ := json.Marshal(l)
		b.Write(row)
		b.WriteByte('\n')
	}
	p := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(p, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func codexEnd(session, call, status, model, transcript string) []byte {
	resp := map[string]any{}
	if status != "" {
		resp["status"] = status
	}
	if model != "" {
		resp["model"] = model
	}
	return endMarshal(map[string]any{
		"hook_event_name": "PostToolUse", "session_id": session, "tool_name": "spawn_agent",
		"tool_use_id": call, "transcript_path": transcript, "tool_response": resp,
	})
}

// Codex records the end and the status, but no cost: its usage source stays
// UsageNone until a real payload proves transcript_path is the child's rollout.
func TestEndCodexRecordsResolvedAndEndButNoUsageUntilVerified(t *testing.T) {
	endEnv(t)
	frontier := cmdCat.ModelFor("codex", core.TierFrontier).ID
	small := cmdCat.ModelFor("codex", core.TierSmall).ID
	dec := seedDecision(t, "codex", "sess-cx", "call_cx1", frontier, small, telemetry.OutcomeRewriteEmitted)

	rc, stdout, _ := runEndFor("codex", codexEnd("sess-cx", "call_cx1", "completed", small, codexRollout(t, 5000, 1000, 800)))
	if rc != 0 || stdout != "" {
		t.Fatalf("rc=%d stdout=%q: codex cannot show a child line, so nothing goes to stdout", rc, stdout)
	}
	key := telemetry.DecisionKey(dec)
	resolved := eventsOf(t, telemetry.OutcomeResolved)
	ended := eventsOf(t, telemetry.OutcomeChildEnded)
	usage := eventsOf(t, telemetry.OutcomeUsage)
	if len(resolved) != 1 || len(ended) != 1 || len(usage) != 0 {
		t.Fatalf("resolved=%d child_ended=%d usage=%d, want 1/1/0", len(resolved), len(ended), len(usage))
	}
	for _, ev := range []telemetry.Event{resolved[0], ended[0]} {
		if ev.LinkedDecision != key || ev.Harness != "codex" {
			t.Errorf("%s linked to %q harness %q, want %q codex", ev.Outcome, ev.LinkedDecision, ev.Harness, key)
		}
	}
	if resolved[0].RewriteHonored == nil || !*resolved[0].RewriteHonored {
		t.Errorf("resolved model equals the written model, want rewrite honored: %+v", resolved[0].RewriteHonored)
	}
	if ended[0].ChildStatus != telemetry.ChildCompleted {
		t.Errorf("status = %q", ended[0].ChildStatus)
	}
}

func TestEndCodexLaunchWithoutStatusIsNotAnEndAndReadsNoUsage(t *testing.T) {
	endEnv(t)
	frontier := cmdCat.ModelFor("codex", core.TierFrontier).ID
	seedDecision(t, "codex", "sess-cx", "call_cx2", frontier, frontier, telemetry.OutcomeAllow)

	runEndFor("codex", codexEnd("sess-cx", "call_cx2", "", "", codexRollout(t, 9000, 0, 900)))
	if n := len(eventsOf(t, telemetry.OutcomeUsage)); n != 0 {
		t.Fatalf("a launch read usage from the parent rollout: %d usage records", n)
	}
	ended := eventsOf(t, telemetry.OutcomeChildEnded)
	if len(ended) != 1 || ended[0].ChildStatus != telemetry.ChildLaunched {
		t.Fatalf("child_ended = %+v, want one launched", ended)
	}
	all, _ := telemetry.ReadEvents()
	for _, c := range telemetry.CompletionByHarness(all) {
		if c.Harness == "codex" && c.Measurable {
			t.Fatal("a launch must not make completion measurable")
		}
	}
}

func TestEndCodexErrorStatusRecordsAFailedEnd(t *testing.T) {
	endEnv(t)
	frontier := cmdCat.ModelFor("codex", core.TierFrontier).ID
	seedDecision(t, "codex", "sess-cx", "call_cx3", frontier, frontier, telemetry.OutcomeAllow)
	runEndFor("codex", codexEnd("sess-cx", "call_cx3", "failed", "", filepath.Join(t.TempDir(), "gone.jsonl")))
	ended := eventsOf(t, telemetry.OutcomeChildEnded)
	if len(ended) != 1 || ended[0].ChildStatus != telemetry.ChildError {
		t.Fatalf("child_ended = %+v, want one error", ended)
	}
	if n := len(eventsOf(t, telemetry.OutcomeUsage)); n != 0 {
		t.Fatalf("a missing rollout produced %d usage records", n)
	}
}

func TestEndCodexIgnoresOtherToolsOtherEventsAndGarbage(t *testing.T) {
	endEnv(t)
	frontier := cmdCat.ModelFor("codex", core.TierFrontier).ID
	seedDecision(t, "codex", "sess-cx", "call_cx4", frontier, frontier, telemetry.OutcomeAllow)
	for name, raw := range map[string][]byte{
		"other tool":  endMarshal(map[string]any{"hook_event_name": "PostToolUse", "tool_name": "shell", "session_id": "sess-cx", "tool_use_id": "call_cx4"}),
		"pre event":   endMarshal(map[string]any{"hook_event_name": "PreToolUse", "tool_name": "spawn_agent", "session_id": "sess-cx", "tool_use_id": "call_cx4"}),
		"garbage":     []byte("not json"),
		"empty":       nil,
		"bad usage":   codexEnd("sess-cx", "call_cx4", "completed", "", filepath.Join(t.TempDir(), "x.txt")),
		"array reply": endMarshal(map[string]any{"hook_event_name": "PostToolUse", "tool_name": "spawn_agent", "tool_response": []any{1}}),
	} {
		before, _ := telemetry.ReadEvents()
		rc, stdout, _ := runEndFor("codex", raw)
		after, _ := telemetry.ReadEvents()
		if rc != 0 || stdout != "" {
			t.Errorf("%s: rc=%d stdout=%q", name, rc, stdout)
		}
		if name == "bad usage" {
			continue // a completed spawn with an unreadable rollout still records the end
		}
		if len(after) != len(before) {
			t.Errorf("%s: recorded %d events, want none", name, len(after)-len(before))
		}
	}
}

func TestEndCursorSubagentStopLinksByToolCallAndStaysUnmeasuredWithoutIds(t *testing.T) {
	endEnv(t)
	frontier := cmdCat.ModelFor("cursor", core.TierFrontier).ID
	dec := seedDecision(t, "cursor", "", "tc_1", frontier, frontier, telemetry.OutcomeAllow)

	rc, stdout, _ := runEndFor("cursor", endMarshal(map[string]any{"hook_event_name": "subagentStop", "status": "completed", "tool_call_id": "tc_1"}))
	if rc != 0 || stdout != "" {
		t.Fatalf("rc=%d stdout=%q", rc, stdout)
	}
	ended := eventsOf(t, telemetry.OutcomeChildEnded)
	if len(ended) != 1 || ended[0].ChildStatus != telemetry.ChildCompleted || ended[0].LinkedDecision != telemetry.DecisionKey(dec) {
		t.Fatalf("child_ended = %+v", ended)
	}
	if n := len(eventsOf(t, telemetry.OutcomeUsage)); n != 0 {
		t.Fatalf("cursor reports no usage this adapter reads, got %d usage records", n)
	}

	// No session and no call id: nothing can be linked, so nothing is recorded.
	before, _ := telemetry.ReadEvents()
	runEndFor("cursor", endMarshal(map[string]any{"hook_event_name": "subagentStop", "status": "completed"}))
	after, _ := telemetry.ReadEvents()
	if len(after) != len(before) {
		t.Fatalf("an unlinkable stop recorded %d events", len(after)-len(before))
	}
}

func TestEndWithoutAnEndEventRecordsNothingAndIsNotAnError(t *testing.T) {
	endEnv(t)
	payload := endMarshal(map[string]any{"hook_event_name": "PostToolUse", "tool_name": "invoke_subagent", "session_id": "s1", "status": "completed"})
	for _, h := range []string{"antigravity", "kirocrew", "grok", "no-such-harness"} {
		rc, stdout, stderr := runEndFor(h, payload)
		if rc != 0 || stdout != "" {
			t.Errorf("%s: rc=%d stdout=%q", h, rc, stdout)
		}
		if !strings.Contains(stderr, "no end event known for "+h) {
			t.Errorf("%s: stderr = %q", h, stderr)
		}
	}
	if all, _ := telemetry.ReadEvents(); len(all) != 0 {
		t.Fatalf("recorded %d events", len(all))
	}
}

// The end parser exists exactly where a child end event is declared as data.
func TestEndParserMatchesDeclaredCapability(t *testing.T) {
	for _, n := range core.AllHarnessCaps() {
		has := portFor(n.Harness).ParseChildEnd != nil
		if declared := n.Caps.ChildEndEvent != ""; declared != has {
			t.Errorf("%s: ChildEndEvent %q but ParseChildEnd set = %v", n.Harness, n.Caps.ChildEndEvent, has)
		}
		if n.Caps.ChildEndEvent == "" && n.Caps.UsageSource != "" && n.Caps.UsageSource != core.UsageNone {
			t.Errorf("%s: UsageSource %q without an end event", n.Harness, n.Caps.UsageSource)
		}
	}
}

// Setup wires an end hook for every harness that declares an end event.
func TestSetupTargetsWireEveryChildEndEvent(t *testing.T) {
	for _, n := range core.AllHarnessCaps() {
		if n.Caps.ChildEndEvent == "" {
			continue
		}
		wired := false
		for _, tg := range setupTargets {
			if tg.Name != n.Harness {
				continue
			}
			for _, h := range tg.Hooks {
				if h.Event == n.Caps.ChildEndEvent {
					wired = true
				}
			}
		}
		if !wired {
			t.Errorf("%s declares ChildEndEvent %q but setup wires no hook for it", n.Harness, n.Caps.ChildEndEvent)
		}
	}
}

// `end claude-code` is the same path as the historical aliases: a stop is
// priced once and the line comes back in the harness's wire form.
func TestEndClaudeCodeMatchesTheAliases(t *testing.T) {
	_, small, frontier := childLineEnv(t)
	spawn(t, small, frontier)
	rc, stdout, _ := runEndFor("claude-code", stopRaw(childTranscript(t, small, 300000, 20000)))
	if rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	if msg := systemMessage(t, stdout); !strings.HasPrefix(msg, "↳ downshift: finished on haiku") {
		t.Fatalf("line = %q", msg)
	}
	// The alias sees the agent as priced already.
	if again, _ := runStop(stopRaw(childTranscript(t, small, 1000, 100))); again != "" {
		t.Errorf("alias priced again: %q", again)
	}
	if n := len(eventsOf(t, telemetry.OutcomeUsage)); n != 1 {
		t.Fatalf("usage records = %d, want 1", n)
	}
	t.Setenv("DOWNSHIFT_QUIET", "1")
	if _, q, _ := runEndFor("claude-code", stopRaw(childTranscript(t, small, 1000, 100))); q != "" {
		t.Errorf("quiet printed %q", q)
	}
}

func TestReportAndDoctorSayWhetherCompletionIsMeasurable(t *testing.T) {
	endEnv(t)
	frontier := cmdCat.ModelFor("codex", core.TierFrontier).ID
	seedDecision(t, "codex", "sess-cx", "call_rp", frontier, frontier, telemetry.OutcomeAllow)

	text := func() string {
		var out, errOut bytes.Buffer
		if rc := runReport(nil, &out, &errOut); rc != 0 {
			t.Fatalf("report rc=%d %s", rc, errOut.String())
		}
		return out.String()
	}
	before := text()
	if !strings.Contains(before, "codex") || !strings.Contains(before, "not yet measurable: no end record seen from PostToolUse") ||
		!strings.Contains(before, "no end event known; completion is not measurable") {
		t.Fatalf("report before any end:\n%s", before)
	}
	runEndFor("codex", codexEnd("sess-cx", "call_rp", "completed", "", ""))
	after := text()
	if !strings.Contains(after, "measurable: 1 end record from PostToolUse") {
		t.Fatalf("report after an end:\n%s", after)
	}
	var out, errOut bytes.Buffer
	runReport([]string{"--json"}, &out, &errOut)
	var rep struct {
		Completion []telemetry.HarnessCompletion `json:"completion"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil || len(rep.Completion) != len(core.AllHarnessCaps()) {
		t.Fatalf("json completion = %+v (%v)", rep.Completion, err)
	}
	for _, c := range rep.Completion {
		if want := c.Harness == "codex"; c.Measurable != want {
			t.Errorf("%s measurable = %v, want %v", c.Harness, c.Measurable, want)
		}
	}
}

// Setup writes the end hook for each harness that declares one, once.
func TestSetupWiresTheEndHookIdempotently(t *testing.T) {
	e := newSetupEnv(t, ".codex", ".cursor", ".claude")
	for i := 0; i < 2; i++ {
		if code, out, _ := e.run("--yes"); code != 0 {
			t.Fatalf("run %d: exit %d\n%s", i, code, out)
		}
	}
	codex := parseDoc(t, e.read(".codex/hooks.json"))
	if got := hookCommands(t, codex, "PostToolUse"); len(got) != 1 || got[0] != testBin+" end codex" {
		t.Errorf("codex PostToolUse = %v", got)
	}
	post := codex["hooks"].(map[string]any)["PostToolUse"].([]any)[0].(map[string]any)
	if post["matcher"] != "(^Agent$|.*spawn_agent$)" {
		t.Errorf("codex end matcher = %v", post["matcher"])
	}
	cur := parseDoc(t, e.read(".cursor/hooks.json"))["hooks"].(map[string]any)["subagentStop"].([]any)
	if len(cur) != 1 || cur[0].(map[string]any)["command"] != testBin+" end cursor" {
		t.Errorf("cursor subagentStop = %v", cur)
	}
	claude := parseDoc(t, e.read(".claude/settings.json"))
	if got := hookCommands(t, claude, "SubagentStop"); len(got) != 1 || got[0] != testBin+" claude-code-subagent-stop" {
		t.Errorf("claude SubagentStop = %v", got)
	}
}
