package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/antigravity"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// Child 0 is rewritten; child 1 (the last) is held by R5. The single event the
// runner records must not claim the held child's downshift was emitted.
func TestAntigravity_HeldLastChildNotLoggedAsRewrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	small := cmdCat.ModelFor("antigravity", core.TierSmall).ID
	mid := cmdCat.ModelFor("antigravity", core.TierMid).ID
	frontier := cmdCat.ModelFor("antigravity", core.TierFrontier).ID
	stdin, _ := json.Marshal(map[string]any{
		"session_models": []string{small, mid, frontier},
		"toolCall": map[string]any{"name": "invoke_subagent", "args": map[string]any{"Subagents": []any{
			map[string]any{"Prompt": "fix typo in README", "Model": frontier},
			map[string]any{"Prompt": "fix typo in the auth token secret handler", "Model": frontier},
		}}},
	})
	stdout := captureStdout(func() {
		runHookAdapter(
			"antigravity", bytes.NewReader(stdin),
			func(b []byte) (antigravity.Event, error) { var e antigravity.Event; return e, json.Unmarshal(b, &e) },
			func(e antigravity.Event) (any, string, core.Decision) { return antigravity.Handle(e, cmdCat) },
			printAllow)
	})
	raw, err := os.ReadFile(eventLog)
	if err != nil {
		t.Fatal(err)
	}
	var ev map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &ev); err != nil {
		t.Fatalf("%q: %v", raw, err)
	}
	t.Logf("stdout=%s", stdout)
	if ev["outcome"] != "allow" && ev["corrections"] != nil {
		t.Fatalf("held decision recorded as outcome=%v verdict=%v safe_verdict=%v corrections=%v to_model=%v",
			ev["outcome"], ev["verdict"], ev["safe_verdict"], ev["corrections"], ev["to_model"])
	}
}
