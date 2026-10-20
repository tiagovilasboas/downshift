// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccsource

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/replay"
)

// cmp treats "opus" and its full id as one model, and "haiku" as cheaper.
type cmp struct{}

func (cmp) Known(m string) bool { return m == "opus" || m == "claude-opus-5-5" || m == "claude-haiku-5-5" }
func (cmp) Compare(from, to string) replay.Direction {
	rank := func(m string) int {
		if m == "claude-haiku-5-5" {
			return 1
		}
		return 3
	}
	switch {
	case rank(to) < rank(from):
		return replay.Cheaper
	case rank(to) > rank(from):
		return replay.Pricier
	}
	return replay.Same
}

// write lays out one parent transcript and its subagents as Claude Code does.
func write(t *testing.T, root string, parentLines []string, children map[string][]string) string {
	t.Helper()
	parent := filepath.Join(root, "proj", "s1.jsonl")
	if err := os.MkdirAll(filepath.Join(root, "proj", "s1", "subagents"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parent, []byte(strings.Join(parentLines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for id, lines := range children {
		p := filepath.Join(root, "proj", "s1", "subagents", "agent-"+id+".jsonl")
		if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func spawn(toolID, prompt, model string) string {
	input := `"prompt":"` + prompt + `","subagent_type":"general-purpose"`
	if model != "" {
		input += `,"model":"` + model + `"`
	}
	return `{"type":"assistant","message":{"model":"claude-opus-5-5","content":[{"type":"tool_use","id":"` + toolID + `","name":"Agent","input":{` + input + `}}]}}`
}

func result(toolID, agent string) string {
	return `{"type":"user","toolUseResult":{"agentId":"` + agent + `","resolvedModel":"x"},"message":{"content":[{"type":"tool_result","tool_use_id":"` + toolID + `"}]}}`
}

func child(id, model string, usage string) string {
	return `{"type":"assistant","message":{"id":"` + id + `","model":"` + model + `","usage":` + usage + `}}`
}

func TestCasesUseCertainBaselinesAndDedupeStreamedMessages(t *testing.T) {
	root := write(t, t.TempDir(), []string{
		spawn("t1", "rename a variable", ""),
		result("t1", "a1"),
		spawn("t2", "plan a migration", "opus"),
		result("t2", "a2"),
		spawn("t3", "fix a typo", ""),
		result("t3", "a3"),
	}, map[string][]string{
		// Two copies of one streamed message: only the last usage counts.
		"a1": {
			child("m1", "claude-opus-5-5", `{"input_tokens":10,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}`),
			child("m1", "claude-opus-5-5", `{"input_tokens":20,"output_tokens":2,"cache_read_input_tokens":3,"cache_creation_input_tokens":4}`),
			child("m2", "claude-opus-5-5", `{"input_tokens":5,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}`),
		},
		"a2": {child("m3", "claude-opus-5-5", `{"input_tokens":7,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}`)},
		// The hook (or a pin) ran this child on haiku while its parent is opus.
		"a3": {child("m4", "claude-haiku-5-5", `{"input_tokens":9,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}`)},
	})
	corpus, err := New(root, cmp{}).Cases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 2 {
		t.Fatalf("cases = %d, want 2 (got skips %v)", len(corpus.Cases), corpus.Skipped)
	}
	if corpus.Skipped[replay.SkipModelDiffers] != 1 {
		t.Errorf("skips = %v, want one model_differs_from_baseline", corpus.Skipped)
	}
	byPrompt := map[string]replay.Case{}
	for _, c := range corpus.Cases {
		byPrompt[c.Prompt] = c
	}
	c := byPrompt["rename a variable"]
	want := replay.Tokens{Input: 25, Output: 2, CacheRead: 3, CacheWrite: 4}
	if c.Tokens != want {
		t.Errorf("tokens = %+v, want %+v (the first copy of m1 must not count)", c.Tokens, want)
	}
	if c.Baseline != "claude-opus-5-5" || c.Ran != "claude-opus-5-5" {
		t.Errorf("baseline %q ran %q, want opus on both", c.Baseline, c.Ran)
	}
	if byPrompt["plan a migration"].Baseline != "opus" {
		t.Errorf("explicit model alias should be the baseline, got %q", byPrompt["plan a migration"].Baseline)
	}
	if strings.Contains(c.ID, "rename") || len(c.ID) != 16 {
		t.Errorf("case id must be an opaque 16-char hash, got %q", c.ID)
	}
}
