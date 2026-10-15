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
)

func probeRun(t *testing.T, event, stdin string) (code int, out string, lines []map[string]any, raw string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	var o, e bytes.Buffer
	code = runProbe([]string{event}, strings.NewReader(stdin), &o, &e)
	data, _ := os.ReadFile(filepath.Join(dir, "probe.jsonl"))
	raw = string(data)
	for _, l := range strings.Split(strings.TrimSpace(raw), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("probe line is not JSON: %q", l)
		}
		lines = append(lines, m)
	}
	return code, strings.TrimSpace(o.String()), lines, raw
}

// The probe answers the harness so it never blocks anything: preToolUse needs
// an explicit allow, every other event takes an empty object.
func TestProbeAlwaysAllowsAndExitsZero(t *testing.T) {
	code, out, _, _ := probeRun(t, "preToolUse", `{"tool_name":"Task"}`)
	if code != 0 || out != `{"permission":"allow"}` {
		t.Errorf("preToolUse: code %d, out %q", code, out)
	}
	code, out, _, _ = probeRun(t, "subagentStart", `{"subagent_model":"claude-haiku-5-5"}`)
	if code != 0 || out != `{}` {
		t.Errorf("subagentStart: code %d, out %q", code, out)
	}
	// Garbage input fails open too.
	code, out, lines, _ := probeRun(t, "sessionStart", `not json`)
	if code != 0 || out != `{}` {
		t.Errorf("bad input: code %d, out %q", code, out)
	}
	if len(lines) != 1 || lines[0]["parse_error"] != true {
		t.Errorf("a bad payload should be recorded as a parse error: %v", lines)
	}
}

func TestProbeRecordsFieldNamesAndModelFieldsOnly(t *testing.T) {
	payload := `{
	  "hook_event_name":"preToolUse","cursor_version":"2.9.1","model":"","model_id":"claude-opus-4-7",
	  "tool_name":"Task","conversation_id":"conv-secret-123","generation_id":"gen-secret-456",
	  "transcript_path":"/Users/someone/.cursor/projects/x/transcript.jsonl","workspace_roots":["/Users/someone/work"],
	  "user_email":"someone@example.com",
	  "tool_input":{"task":"SECRET TASK TEXT: rotate the production key","model":"inherit","subagent_type":"explore"}
	}`
	_, _, lines, raw := probeRun(t, "preToolUse", payload)
	if len(lines) != 1 {
		t.Fatalf("one line expected, got %d", len(lines))
	}
	for _, leak := range []string{"SECRET TASK TEXT", "conv-secret-123", "gen-secret-456", "/Users/someone", "someone@example.com", "rotate the production key"} {
		if strings.Contains(raw, leak) {
			t.Errorf("the probe recorded %q:\n%s", leak, raw)
		}
	}
	l := lines[0]
	if l["event"] != "preToolUse" {
		t.Errorf("event = %v", l["event"])
	}
	keys, _ := l["top_keys"].([]any)
	for _, want := range []string{"conversation_id", "model_id", "tool_input", "transcript_path"} {
		found := false
		for _, k := range keys {
			found = found || k == want
		}
		if !found {
			t.Errorf("top_keys is missing %q: %v", want, keys)
		}
	}
	inputKeys, _ := l["tool_input_keys"].([]any)
	if len(inputKeys) != 3 {
		t.Errorf("tool_input_keys = %v, want task, model, subagent_type", inputKeys)
	}
	fields, _ := l["fields"].(map[string]any)
	// Model ids and the tool name are not sensitive: their values are kept, and an
	// empty model is recorded as empty, which is what the probe is there to learn.
	if fields["model_id"] != "claude-opus-4-7" || fields["model"] != "" || fields["tool_name"] != "Task" || fields["tool_input.model"] != "inherit" {
		t.Errorf("fields = %v", fields)
	}
	if fields["transcript_path"] != "present" {
		t.Errorf("transcript_path should be recorded as present, not its value: %v", fields["transcript_path"])
	}
	// Ids are kept as short hashes so events can be joined without exposing them.
	conv, _ := fields["conversation_id"].(string)
	if len(conv) != 8 || conv == "conv-secret" {
		t.Errorf("conversation_id should be an 8-char hash, got %q", conv)
	}
}

func TestProbeHashesTheSameIdTheSameWay(t *testing.T) {
	a := probeIDHash("conv-1")
	if a != probeIDHash("conv-1") || a == probeIDHash("conv-2") {
		t.Errorf("hash must be stable and distinguish ids")
	}
	if probeIDHash("") != "" {
		t.Errorf("an empty id stays empty")
	}
}

func TestProbeRejectsAnOddEventNameButStillAnswers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	var o, e bytes.Buffer
	code := runProbe([]string{"../../etc/passwd"}, strings.NewReader(`{}`), &o, &e)
	if code != 0 || strings.TrimSpace(o.String()) != `{}` {
		t.Errorf("code %d out %q", code, o.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "probe.jsonl")); err == nil {
		t.Errorf("nothing should be written for an invalid event name")
	}
}

func TestProbeStopsAtTheSizeCap(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "probe.jsonl"), bytes.Repeat([]byte("x"), probeMaxBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	var o, e bytes.Buffer
	if code := runProbe([]string{"stop"}, strings.NewReader(`{}`), &o, &e); code != 0 {
		t.Fatalf("code %d", code)
	}
	info, _ := os.Stat(filepath.Join(dir, "probe.jsonl"))
	if info.Size() != int64(probeMaxBytes) {
		t.Errorf("file grew past the cap: %d", info.Size())
	}
}
