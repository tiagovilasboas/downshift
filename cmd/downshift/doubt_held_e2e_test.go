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

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/testenv"
)

// doubtAdapter runs one payload through the shared hook runner for a harness
// and returns what the hook wrote on stdout.
type doubtAdapter struct {
	harness string
	run     func(stdin []byte) string
	// payload builds a spawn of task in cwd; model "" names none.
	payload func(cwd, model, task string) map[string]any
}

func doubtAdapters() []doubtAdapter {
	return []doubtAdapter{
		{
			harness: "claude-code",
			run: func(stdin []byte) string {
				return captureStdout(func() {
					runHookAdapter("claude-code", bytes.NewReader(stdin),
						func(b []byte) (claudecode.Event, error) { var e claudecode.Event; return e, json.Unmarshal(b, &e) },
						func(e claudecode.Event) (any, string, core.Decision) { return claudecode.Handle(e, cmdCat) },
						printAllow)
				})
			},
			payload: func(cwd, model, task string) map[string]any {
				in := map[string]any{"prompt": task}
				if model != "" {
					in["model"] = model
				}
				return map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Agent", "cwd": cwd, "tool_input": in}
			},
		},
		{
			harness: "cursor",
			run: func(stdin []byte) string {
				return captureStdout(func() {
					runHookAdapter("cursor", bytes.NewReader(stdin),
						func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
						func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, cmdCat) },
						printCursorAllow)
				})
			},
			payload: func(cwd, model, task string) map[string]any {
				if model == "" {
					model = "inherit"
				}
				return map[string]any{
					"hook_event_name": "preToolUse", "tool_name": "Task", "cwd": cwd,
					"tool_input": map[string]any{"task": task, "model": model},
				}
			},
		},
	}
}

// doubtTask is a decisive TRIVIAL task with no risk category.
const doubtTask = "rename the userId variable to userIdentifier"

// writeRoutingRule writes a project policy whose one rule sends every spawn to
// tier, so the production decider (routing/decide) proposes that tier.
func writeRoutingRule(t *testing.T, dir, tier string) {
	t.Helper()
	p := filepath.Join(dir, ".downshift", "policy.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"version":1,"routing":{"rules":[{"id":"all","tier":"` + tier + `"}]}}`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Doubt stays allow (R7, R8), enforced by routing/guard on the production
// path: a proposed move above the requested tier (an upshift) and a frontier
// proposal for a spawn whose requested model is unknown are both held, so the
// hook never writes the frontier model. The control proves the rule is read:
// the same rule shape choosing small does move a frontier request down.
func TestHookE2E_DoubtHoldsUpshiftAndUnknownFrontier(t *testing.T) {
	for _, a := range doubtAdapters() {
		t.Run(a.harness, func(t *testing.T) {
			testenv.SetHome(t, t.TempDir())
			t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
			small := cmdCat.ModelFor(a.harness, core.TierSmall)
			mid := cmdCat.ModelFor(a.harness, core.TierMid)
			frontier := cmdCat.ModelFor(a.harness, core.TierFrontier)
			setSessionAllowlist(t, a.harness, []string{small.ID, mid.ID, frontier.ID})
			hook := func(cwd, model string) string {
				stdin, _ := json.Marshal(a.payload(cwd, model, doubtTask))
				return a.run(stdin)
			}
			wrote := func(out string, m core.Model) bool {
				return strings.Contains(out, `"`+m.ID+`"`) || strings.Contains(out, `"`+m.WriteName()+`"`)
			}

			up := t.TempDir()
			writeRoutingRule(t, up, "frontier")
			if out := hook(up, small.ID); wrote(out, frontier) {
				t.Errorf("R8: an upshift from %s must be held, hook wrote %s", small.ID, out)
			}
			if out := hook(up, ""); wrote(out, frontier) {
				t.Errorf("R7: frontier for an unknown requested model must be held, hook wrote %s", out)
			}

			down := t.TempDir()
			writeRoutingRule(t, down, "small")
			if out := hook(down, frontier.ID); !wrote(out, small) {
				t.Errorf("control: a small rule must move a frontier request to %s, hook wrote %s", small.ID, out)
			}
		})
	}
}
