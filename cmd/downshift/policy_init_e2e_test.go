// Copyright (c) 2026 Tiago Vilas Boas.
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

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/classify"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
)

// The file `policy init` writes is a real routing policy: through the Claude
// Code hook, an Explore spawn in a project holding that file goes to the small
// tier, written as the harness's own alias, whether or not the call names a
// model (a declared profile bounds the orchestrator's choice). The prompt is
// one the classifier calls COMPLEX, so without the file the same spawn is kept:
// what moves it is the profile, not the task class.
func TestHookE2E_PolicyInitExploreProfile(t *testing.T) {
	decidertest.Use(t, routingDecider{now: time.Now})
	project := isolatePolicy(t)
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	var out, errw bytes.Buffer
	if rc := runPolicyInit([]string{"--yes"}, strings.NewReader(""), &out, &errw); rc != 0 {
		t.Fatalf("policy init rc=%d: %s", rc, errw.String())
	}
	frontier := cmdCat.ModelFor("claude-code", core.TierFrontier)
	small := cmdCat.ModelFor("claude-code", core.TierSmall)
	wantSmall := small.ID
	if small.Native != "" {
		wantSmall = small.Native
	}
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(transcript, []byte(`{"type":"assistant","message":{"model":"`+frontier.ID+`","content":[]}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(cwd, model string) (written string, rewritten bool) {
		toolInput := map[string]any{"subagent_type": "Explore", "prompt": explorePrompt}
		if model != "" {
			toolInput["model"] = model
		}
		payload := map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Agent", "transcript_path": transcript, "tool_input": toolInput}
		if cwd != "" {
			payload["cwd"] = cwd
		}
		stdin, _ := json.Marshal(payload)
		raw := captureStdout(func() {
			runHookAdapter("claude-code", bytes.NewReader(stdin),
				func(b []byte) (claudecode.Event, error) { var e claudecode.Event; return e, json.Unmarshal(b, &e) },
				func(e claudecode.Event) (any, string, core.Decision) { return claudecode.Handle(e, cmdCat) },
				printAllow)
		})
		var o struct {
			HookSpecificOutput struct {
				UpdatedInput map[string]any `json:"updatedInput"`
			} `json:"hookSpecificOutput"`
		}
		_ = json.Unmarshal([]byte(raw), &o)
		m, ok := o.HookSpecificOutput.UpdatedInput["model"].(string)
		return m, ok
	}
	if c := classify.Classify(explorePrompt); c.Complexity != classify.Complex || len(c.Risk) != 0 {
		t.Fatalf("the fixture prompt must classify COMPLEX with no risk category so only the profile can move it, got %+v", c)
	}
	if got, ok := run(project, ""); !ok || got != wantSmall {
		t.Fatalf("Explore in the project: model=%q rewritten=%v, want %q", got, ok, wantSmall)
	}
	if got, ok := run(project, frontier.Native); !ok || got != wantSmall {
		t.Fatalf("Explore with a model named on the call: model=%q rewritten=%v, want the profile's %q", got, ok, wantSmall)
	}
	if got, ok := run(t.TempDir(), ""); ok {
		t.Fatalf("without the policy file a COMPLEX Explore spawn must be kept, got rewrite to %q", got)
	}
}

// explorePrompt is a read-only investigation the classifier calls COMPLEX and
// that names no risk category, so the class step never moves it.
const explorePrompt = "Investigate why the report generation diverges between the EU and US regions, trace the root cause across the scheduler, the cache and the renderer, and weigh the architectural trade-offs of each fix"
