// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func promptPayload(t *testing.T, prompt string) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "prompt": prompt})
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b)
}

// Every harness that had a DCI path before the runner moved keeps the same
// output: the nested hookSpecificOutput everywhere, plus the top-level
// additionalContext for every harness but Claude Code.
func TestPromptContextOutputPerHarness(t *testing.T) {
	source := func(prompt, _ string) string { return "ctx:" + prompt }
	for _, harness := range []string{"claude-code", "cursor", "codex", "opencode", "generic"} {
		var out bytes.Buffer
		if code := runPromptContext(harness, promptPayload(t, "hi"), &out, source); code != 0 {
			t.Fatalf("%s: exit %d, want 0", harness, code)
		}
		want := `{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"ctx:hi"}`
		if harness != "claude-code" {
			want += `,"additionalContext":"ctx:hi"`
		}
		want += "}\n"
		if out.String() != want {
			t.Errorf("%s: output %q, want %q", harness, out.String(), want)
		}
	}
}

func TestPromptContextFailsOpenSilently(t *testing.T) {
	source := func(string, string) string { return "ctx" }
	abstain := func(string, string) string { return "" }
	panics := func(string, string) string { panic("boom") }
	cases := []struct {
		name, harness string
		in            string
		source        promptContextSource
	}{
		{"harness without the capability", "kirocrew", `{"prompt":"hi"}`, source},
		{"unregistered harness", "nope", `{"prompt":"hi"}`, source},
		{"empty stdin", "claude-code", ``, source},
		{"invalid json", "claude-code", `{bad`, source},
		{"empty prompt", "claude-code", `{"cwd":"/tmp"}`, source},
		{"abstention", "claude-code", `{"prompt":"hi"}`, abstain},
		{"source panic", "claude-code", `{"prompt":"hi"}`, panics},
	}
	for _, c := range cases {
		var out bytes.Buffer
		if code := runPromptContext(c.harness, strings.NewReader(c.in), &out, c.source); code != 0 || out.Len() != 0 {
			t.Errorf("%s: code=%d out=%q, want 0 and silence", c.name, code, out.String())
		}
	}
	if code := runPromptContext("claude-code", nil, &bytes.Buffer{}, source); code != 0 {
		t.Errorf("nil reader: code=%d, want 0", code)
	}
}

// The runner's harness gate reads the capability table, not a private list.
func TestPromptContextGateFollowsCapabilities(t *testing.T) {
	for _, n := range core.AllHarnessCaps() {
		if got := promptContextHarness(n.Harness); got != n.Caps.CanInjectPromptContext {
			t.Errorf("%s: gate=%v, caps=%v", n.Harness, got, n.Caps.CanInjectPromptContext)
		}
	}
	if !promptContextHarness("generic") {
		t.Error("generic must stay accepted")
	}
}
