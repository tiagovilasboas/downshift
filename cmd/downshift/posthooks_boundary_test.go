// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

type phRunner struct {
	name string
	run  func(io.Reader) int
}

func phRunners() []phRunner {
	return []phRunner{
		{"post-tool-use", func(in io.Reader) int { return runHonor(in, portFor("claude-code"), cmdCat) }},
		{"subagent-stop", func(in io.Reader) int { return runUsage(in, portFor("claude-code"), cmdCat) }},
		{"end", func(in io.Reader) int { return runEnd(in, portFor("claude-code"), cmdCat) }},
	}
}

func phIsolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv(paths.EnvStateDir, filepath.Join(dir, "state"))
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(dir, "events.jsonl"))
	return dir
}

func assertNeutral(t *testing.T, label, out string) {
	t.Helper()
	trim := strings.TrimSpace(out)
	if trim != "" && trim != "{}" && !strings.HasPrefix(trim, `{"systemMessage":`) {
		t.Fatalf("%s: non-neutral stdout %q", label, out)
	}
	for _, bad := range []string{"decision", "block", "hookSpecificOutput", "additionalContext", "updated", "continue", "reason"} {
		if strings.Contains(trim, `"`+bad) {
			t.Fatalf("%s: stdout carries %q: %q", label, bad, out)
		}
	}
}

func TestPostHooks_PostHooksFailOpen(t *testing.T) {
	prev := hookReadTimeout
	hookReadTimeout = 50 * time.Millisecond
	t.Cleanup(func() { hookReadTimeout = prev })
	huge := `{"hook_event_name":"PostToolUse","tool_name":"Task","x":"` + strings.Repeat("a", 1<<20+10) + `"}`
	inputs := map[string]func() io.Reader{
		"malformed": func() io.Reader { return strings.NewReader(`{"tool_name":`) },
		"empty":     func() io.Reader { return strings.NewReader("") },
		"huge":      func() io.Reader { return strings.NewReader(huge) },
		"unknown": func() io.Reader {
			return strings.NewReader(`{"zz":1,"tool_name":"Task","tool_response":[1,2],"hook_event_name":7}`)
		},
		"badtypes": func() io.Reader {
			return strings.NewReader(`{"hook_event_name":"SubagentStop","agent_id":5,"agent_transcript_path":["x"]}`)
		},
		"timeout": func() io.Reader { return delayedReader{release: make(chan struct{})} },
	}
	for _, r := range phRunners() {
		for name, mk := range inputs {
			phIsolate(t)
			var rc int
			start := time.Now()
			out := captureStdout(func() { captureStderr(func() { rc = r.run(mk()) }) })
			label := r.name + "/" + name
			if rc != 0 {
				t.Fatalf("%s: rc=%d", label, rc)
			}
			assertNeutral(t, label, out)
			if d := time.Since(start); d > time.Second {
				t.Fatalf("%s: took %v", label, d)
			}
		}
	}
}

// The PostToolUse runner takes a blocking flock on context-sensor.json.lock
// (internal/sensor/flock_unix.go:14, LOCK_EX without LOCK_NB, no timeout).
// Any other holder of that lock stalls the hook indefinitely.
func TestPostHooks_PostToolUseBoundedWhenSensorLockHeld(t *testing.T) {
	dir := phIsolate(t)
	state := filepath.Join(dir, "state")
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	lf, err := os.OpenFile(filepath.Join(state, "context-sensor.json.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	payload := `{"hook_event_name":"PostToolUse","session_id":"s1","tool_name":"Task","tool_use_id":"tu1","tool_response":{"status":"completed","content":"ok"}}`
	done := make(chan int, 1)
	r, w, _ := os.Pipe()
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w
	go func() { done <- runHonor(strings.NewReader(payload), portFor("claude-code"), cmdCat) }()
	var rc int
	var hung bool
	select {
	case rc = <-done:
	case <-time.After(3 * time.Second):
		hung = true
	}
	os.Stdout, os.Stderr = oldOut, oldErr
	syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) // release so the goroutine can finish
	lf.Close()
	if hung {
		<-done
	}
	w.Close()
	io.Copy(io.Discard, r)
	if hung {
		t.Fatalf("PostToolUse hook did not return within 3s while context-sensor.json.lock was held (unbounded flock)")
	}
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
}

// Prompt text, tool output, paths and secrets in PostToolUse / SubagentStop
// payloads must not reach any state file.
func TestPostHooks_PostHooksTelemetryPrivacy(t *testing.T) {
	dir := phIsolate(t)
	const secret = "sk-ant-SECRETVALUE123"
	const prompt = "PROMPT_TEXT_MARKER refactor auth"
	tp := filepath.Join(dir, "PATHMARKER", "agent.jsonl")
	os.MkdirAll(filepath.Dir(tp), 0o700)
	os.WriteFile(tp, []byte(`{"type":"assistant","message":{"id":"m1","model":"claude-haiku-4-5","usage":{"input_tokens":10,"output_tokens":5},"content":[{"type":"text","text":"`+secret+`"}]}}`+"\n"), 0o600)
	posts := []string{
		`{"hook_event_name":"PostToolUse","session_id":"s1","tool_name":"Task","tool_use_id":"tu1","tool_input":{"prompt":"` + prompt + `"},"tool_response":{"status":"completed","agentId":"a1","resolvedModel":"claude-haiku-4-5","content":"` + secret + ` PATHMARKER","usage":{"input_tokens":3,"output_tokens":4}},"cwd":"/x/PATHMARKER"}`,
		`{"hook_event_name":"PostToolUseFailure","session_id":"s1","tool_name":"Task","tool_use_id":"tu2","tool_input":{"prompt":"` + prompt + `"},"error":"` + secret + `","is_interrupt":false}`,
	}
	for _, p := range posts {
		captureStdout(func() { captureStderr(func() { runHonor(strings.NewReader(p), portFor("claude-code"), cmdCat) }) })
	}
	stop := fmt.Sprintf(`{"hook_event_name":"SubagentStop","session_id":"s1","agent_id":"a2","agent_transcript_path":%q,"last_assistant_message":%q}`, tp, secret)
	captureStdout(func() { captureStderr(func() { runUsage(strings.NewReader(stop), portFor("claude-code"), cmdCat) }) })
	found := false
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || p == tp {
			return nil
		}
		b, _ := os.ReadFile(p)
		found = true
		t.Logf("%s: %s", filepath.Base(p), strings.TrimSpace(string(b)))
		for _, m := range []string{secret, "PROMPT_TEXT_MARKER", "PATHMARKER", "\"s1\"", "\"a1\"", "\"tu1\""} {
			if strings.Contains(string(b), m) {
				t.Errorf("%s contains %q", p, m)
			}
		}
		return nil
	})
	if !found {
		t.Fatal("no state files written")
	}
}

// NO_ROUTE / pause / control share: post hooks must stay neutral.
func TestPostHooks_PostHooksNeutralInNoRouteAndControl(t *testing.T) {
	payload := `{"hook_event_name":"PostToolUse","session_id":"s1","tool_name":"Task","tool_use_id":"tu1","tool_response":{"status":"completed","resolvedModel":"claude-haiku-4-5","content":"FAIL: TestX","usage":{"input_tokens":3,"output_tokens":4}}}`
	for _, env := range [][2]string{{"DOWNSHIFT_NO_ROUTE", "1"}, {"DOWNSHIFT_CONTROL_SHARE", "1"}} {
		phIsolate(t)
		t.Setenv(env[0], env[1])
		var rc int
		out := captureStdout(func() {
			captureStderr(func() { rc = runHonor(strings.NewReader(payload), portFor("claude-code"), cmdCat) })
		})
		if rc != 0 {
			t.Fatalf("%s: rc=%d", env[0], rc)
		}
		assertNeutral(t, env[0], out)
		t.Logf("%s stdout: %s", env[0], strings.TrimSpace(out))
	}
}

func TestPostHooks_LinkedRecordsPrivacy(t *testing.T) {
	dir := phIsolate(t)
	ev := telemetry.Event{CorrelationID: "c0", Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Source: "hook", Agent: "subagent",
		Harness: "claude-code", Outcome: telemetry.OutcomeRewriteEmitted, FromModel: "claude-sonnet-4-5", ToModel: "claude-haiku-4-5",
		Verdict: "DOWNSHIFT", SessionID: telemetry.HashSessionID("s1"), ToolUseHash: telemetry.HashSessionID("tu1")}
	telemetry.Record(ev)
	secret := "sk-ant-SECRETVALUE123"
	for _, p := range []string{
		`{"hook_event_name":"PostToolUse","session_id":"s1","tool_name":"Task","tool_use_id":"tu1","tool_input":{"prompt":"PROMPT_TEXT_MARKER"},"tool_response":{"status":"completed","agentId":"a1","resolvedModel":"claude-haiku-4-5","content":"` + secret + `"},"usage":{"input_tokens":3,"output_tokens":4},"model":"claude-haiku-4-5"}`,
	} {
		out := captureStdout(func() { captureStderr(func() { runHonor(strings.NewReader(p), portFor("claude-code"), cmdCat) }) })
		t.Logf("stdout: %s", strings.TrimSpace(out))
	}
	b, _ := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		t.Logf("%s", l)
	}
	for _, m := range []string{secret, "PROMPT_TEXT_MARKER", `"s1"`, `"a1"`, `"tu1"`} {
		if strings.Contains(string(b), m) {
			t.Errorf("events contain %q", m)
		}
	}
}

// readSubagentUsage checks IsRegular only after os.Open, and open(2) on a FIFO
// blocks until a writer appears (claudecode/subagentstop.go:39-43).
func TestPostHooks_SubagentStopFIFOTranscriptBounded(t *testing.T) {
	dir := phIsolate(t)
	fifo := filepath.Join(dir, "agent.jsonl")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip(err)
	}
	payload := fmt.Sprintf(`{"hook_event_name":"SubagentStop","session_id":"s1","agent_id":"a1","agent_transcript_path":%q}`, fifo)
	done := make(chan int, 1)
	go func() { done <- runUsage(strings.NewReader(payload), portFor("claude-code"), cmdCat) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		w, _ := os.OpenFile(fifo, os.O_WRONLY, 0) // unblock the reader
		w.Close()
		<-done
		t.Fatalf("SubagentStop hook blocked >3s opening a FIFO transcript path")
	}
}
