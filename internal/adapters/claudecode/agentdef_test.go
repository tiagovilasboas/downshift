// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"github.com/tiagovilasboas/downshift/internal/telemetry/fromcore"
)

const mechanicalBrief = "rename the userId variable to userIdentifier in user.go"

// writeFile creates dir/name with content, making directories as needed.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The model a custom or plugin agent runs on comes from its definition, not
// from a closed list of type names. No `model:` or `inherit` is the parent's
// model (a measurable baseline, so a downshift counts); a named model is the
// author's explicit choice (known baseline, never rewritten); no definition
// found keeps today's unknown baseline.
func TestHandle_AgentDefinitionDecidesBaseline(t *testing.T) {
	frontier := catID(core.TierFrontier)
	cases := []struct {
		name        string
		subagent    string
		definition  string // file under <cwd>/.claude/agents, "" for none
		plugin      bool   // definition lives in an installed plugin instead
		brief       string
		wantBase    string // baseline model id, "" for unknown
		wantPinned  bool
		wantRewrite bool // the small-tier pick is written (a confident trivial brief needs no baseline)
	}{
		{name: "no model line inherits the parent", subagent: "worker", definition: "---\nname: worker\ndescription: x\n---\nbody", brief: mechanicalBrief, wantBase: frontier, wantRewrite: true},
		{name: "inherit inherits the parent", subagent: "worker", definition: "---\nname: worker\nmodel: inherit\n---\n", brief: mechanicalBrief, wantBase: frontier, wantRewrite: true},
		{name: "quoted inherit inherits the parent", subagent: "worker", definition: "---\nmodel: \"Inherit\"\n---\n", brief: mechanicalBrief, wantBase: frontier, wantRewrite: true},
		{name: "a named alias is pinned and kept", subagent: "worker", definition: "---\nmodel: opus\n---\n", brief: mechanicalBrief, wantBase: frontier, wantPinned: true},
		{name: "a model the catalog does not know is pinned and kept", subagent: "worker", definition: "---\nmodel: some-unlisted-model\n---\n", brief: mechanicalBrief, wantPinned: true},
		{name: "file name differs from the agent name", subagent: "renamer", definition: "---\nname: renamer\n---\n", brief: mechanicalBrief, wantBase: frontier, wantRewrite: true},
		{name: "no definition found keeps the baseline unknown", subagent: "ghost", brief: mechanicalBrief, wantRewrite: true},
		{name: "a path-like type is never looked up", subagent: "../worker", definition: "---\nmodel: inherit\n---\n", brief: mechanicalBrief, wantRewrite: true},
		{name: "a file with no frontmatter is not a definition", subagent: "worker", definition: "model: inherit\nno delimiters", brief: mechanicalBrief, wantRewrite: true},
		{name: "plugin agent with no model inherits", subagent: "mechanical-pack:mechanical-worker", plugin: true, definition: "---\nname: mechanical-worker\n---\n", brief: mechanicalBrief, wantBase: frontier, wantRewrite: true},
		// Risk floors and the classifier still apply to a declared mechanical agent.
		{name: "a risky brief is not downshifted for a mechanical agent", subagent: "mechanical-pack:mechanical-worker", plugin: true, definition: "---\nname: mechanical-worker\n---\n", brief: "drop the production users table and delete all payment records", wantBase: frontier},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home, cwd, user := t.TempDir(), t.TempDir(), t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("CLAUDE_CONFIG_DIR", user)
			t.Setenv("ANTHROPIC_MODEL", "")
			t.Setenv("CLAUDE_CODE_SUBAGENT_MODEL", "")
			t.Setenv("CLAUDE_CODE_SUBAGENT_MODEL_FORCE", "")
			if tc.definition != "" {
				if tc.plugin {
					install := filepath.Join(user, "plugins", "cache", "mechanical-pack")
					writeFile(t, install, "agents/mechanical-worker.md", tc.definition)
					writeFile(t, user, "plugins/installed_plugins.json",
						`{"version":2,"plugins":{"mechanical-pack@market":[{"scope":"user","installPath":"`+install+`"}]}}`)
				} else {
					writeFile(t, cwd, ".claude/agents/worker.md", tc.definition)
					writeFile(t, cwd, ".claude/agents/renamer-file.md", tc.definition)
				}
			}
			transcript := filepath.Join(t.TempDir(), "session.jsonl")
			writeFile(t, filepath.Dir(transcript), filepath.Base(transcript), `{"type":"assistant","message":{"model":"`+frontier+`"}}`+"\n")
			input, _ := json.Marshal(map[string]string{"subagent_type": tc.subagent, "prompt": tc.brief})
			ev := withCatalogSession(claudecode.Event{ToolName: "Agent", TranscriptPath: transcript, Cwd: cwd, ToolInput: input})

			out, note, d := claudecode.Handle(ev, cat)

			if d.Baseline.RequestedID != tc.wantBase {
				t.Errorf("baseline = %q, want %q", d.Baseline.RequestedID, tc.wantBase)
			}
			if d.Baseline.Pinned != tc.wantPinned {
				t.Errorf("pinned = %v, want %v", d.Baseline.Pinned, tc.wantPinned)
			}
			if d.Baseline.Guessed || d.Baseline.FromSettings {
				t.Errorf("a definition is a fact, not a settings guess: %+v", d.Baseline)
			}
			if got := decodeUpdated(t, out) != nil; got != tc.wantRewrite {
				t.Errorf("rewrite written = %v, want %v (verdict %v, tier %v, note %q)", got, tc.wantRewrite, d.Verdict, d.Tier, note)
			}
			if got := slices.Contains(fromcore.ReasonCodes(d), telemetry.ReasonAgentPinsModel); got != tc.wantPinned {
				t.Errorf("AGENT_PINS_MODEL reason = %v, want %v", got, tc.wantPinned)
			}
		})
	}
}

// A built-in type that runs on its own model is known, not unknown, and is left
// alone: it is an explicit choice of Claude Code, not the parent's model.
func TestHandle_BuiltinPinnedTypesHaveKnownBaseline(t *testing.T) {
	for _, typ := range []string{"claude-code-guide", "statusline-setup"} {
		t.Run(typ, func(t *testing.T) {
			input, _ := json.Marshal(map[string]string{"subagent_type": typ, "prompt": mechanicalBrief})
			out, _, d := claudecode.Handle(withCatalogSession(claudecode.Event{ToolName: "Agent", Cwd: t.TempDir(), ToolInput: input}), cat)
			if d.Baseline.RequestedID == "" || !d.Baseline.Pinned {
				t.Fatalf("baseline = %q pinned=%v, want a known pinned model", d.Baseline.RequestedID, d.Baseline.Pinned)
			}
			if decodeUpdated(t, out) != nil {
				t.Fatal("a pinned built-in must not be rewritten")
			}
		})
	}
}
