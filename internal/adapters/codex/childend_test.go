// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codex_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Fixtures follow the documented shapes only: PostToolUse common fields
// (tool_use_id, session_id, transcript_path) and rollout event_msg/token_count
// events. tool_response.status/model are assumed names (unverified).
func rollout(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const tokenCount = `{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":%s,"cached_input_tokens":%s,"output_tokens":%s}}}}`

func tc(in, cached, out string) string {
	return fmt.Sprintf(tokenCount, in, cached, out)
}

func TestParseChildEnd(t *testing.T) {
	p := rollout(t, tc("100", "0", "5"), `{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"limit_id":"codex"}}}`, tc("2000", "500", "300"))
	cases := []struct {
		name      string
		payload   string
		ok        bool
		status    telemetry.ChildStatus
		wantUsage *hookport.TokenUsage
		wantModel string
		wantAgent string
	}{
		{"completed with usage", `{"hook_event_name":"PostToolUse","session_id":"s","tool_name":"spawn_agent","tool_use_id":"call_1","transcript_path":"` + p + `","tool_response":{"status":"completed","model":"gpt-5.6-terra"}}`,
			true, telemetry.ChildCompleted, &hookport.TokenUsage{InputTokens: 1500, CachedTokens: 500, OutputTokens: 300}, "gpt-5.6-terra", "call_1"},
		{"namespaced tool", `{"tool_name":"multi_agent_v2.spawn_agent","tool_use_id":"call_2","transcript_path":"` + p + `","tool_response":{"status":"error"}}`,
			true, telemetry.ChildError, &hookport.TokenUsage{InputTokens: 1500, CachedTokens: 500, OutputTokens: 300}, "", "call_2"},
		{"no status is a launch and reads no usage", `{"hook_event_name":"PostToolUse","tool_name":"spawn_agent","tool_use_id":"call_3","transcript_path":"` + p + `"}`,
			true, telemetry.ChildLaunched, nil, "", "call_3"},
		{"interrupted", `{"tool_name":"spawn_agent","tool_use_id":"call_4","tool_response":{"status":"aborted"}}`,
			true, telemetry.ChildInterrupted, nil, "", "call_4"},
		{"string tool_response is empty", `{"tool_name":"spawn_agent","tool_use_id":"call_5","tool_response":"done"}`,
			true, telemetry.ChildLaunched, nil, "", "call_5"},
		{"other tool", `{"tool_name":"shell","tool_use_id":"call_6"}`, false, "", nil, "", ""},
		{"other event", `{"hook_event_name":"PreToolUse","tool_name":"spawn_agent"}`, false, "", nil, "", ""},
		{"not json", `nope`, false, "", nil, "", ""},
	}
	for _, c := range cases {
		ce, ok := codex.ParseChildEnd([]byte(c.payload))
		if ok != c.ok {
			t.Errorf("%s: ok = %v, want %v", c.name, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		if ce.Harness != "codex" || ce.Source != hookport.SourcePayload || !ce.IsChild || ce.Status != c.status {
			t.Errorf("%s: %+v", c.name, ce)
		}
		if ce.AgentKey != c.wantAgent || ce.ToolUseID != c.wantAgent || ce.Model != c.wantModel || ce.ResolvedModel != c.wantModel {
			t.Errorf("%s: ids/model %+v", c.name, ce)
		}
		switch {
		case c.wantUsage == nil && ce.Usage != nil:
			t.Errorf("%s: usage = %+v, want none", c.name, *ce.Usage)
		case c.wantUsage != nil && (ce.Usage == nil || *ce.Usage != *c.wantUsage):
			t.Errorf("%s: usage = %+v, want %+v", c.name, ce.Usage, *c.wantUsage)
		}
	}
}

func TestParseChildEndUsageNeedsAReadableJSONLRollout(t *testing.T) {
	good := rollout(t, tc("10", "0", "2"))
	dir := t.TempDir()
	txt := filepath.Join(dir, "r.txt")
	_ = os.WriteFile(txt, []byte(tc("10", "0", "2")+"\n"), 0o600)
	for name, path := range map[string]string{"missing": filepath.Join(dir, "none.jsonl"), "wrong extension": txt, "empty": ""} {
		ce, ok := codex.ParseChildEnd([]byte(`{"tool_name":"spawn_agent","tool_use_id":"c","transcript_path":"` + path + `","tool_response":{"status":"completed"}}`))
		if !ok || ce.Usage != nil {
			t.Errorf("%s: ok=%v usage=%v, want a parsed end with no usage", name, ok, ce.Usage)
		}
	}
	ce, _ := codex.ParseChildEnd([]byte(`{"tool_name":"spawn_agent","tool_use_id":"c","transcript_path":"` + good + `","tool_response":{"status":"completed"}}`))
	if ce.Usage == nil || ce.Usage.InputTokens != 10 || ce.Usage.OutputTokens != 2 {
		t.Errorf("usage = %+v", ce.Usage)
	}
	// Cached above input is clamped, never negative.
	over := rollout(t, tc("10", "40", "2"))
	ce, _ = codex.ParseChildEnd([]byte(`{"tool_name":"spawn_agent","tool_use_id":"c","transcript_path":"` + over + `","tool_response":{"status":"completed"}}`))
	if ce.Usage == nil || ce.Usage.InputTokens != 0 || ce.Usage.CachedTokens != 10 {
		t.Errorf("clamped usage = %+v", ce.Usage)
	}
}
