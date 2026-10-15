// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
)

func writeTranscript(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSessionModelFromTranscript(t *testing.T) {
	last := `{"type":"assistant","message":{"model":"m-old"}}
{"type":"user","message":{"content":"the word \"assistant\" in a prompt"}}
{"type":"assistant","message":{"model":"m-new"}}
{"type":"assistant","message":{"model":"<synthetic>"}}
`
	cases := []struct {
		name, path, want string
	}{
		{"last real assistant model wins", writeTranscript(t, last), "m-new"},
		{"missing file fails open", filepath.Join(t.TempDir(), "absent.jsonl"), ""},
		{"empty path", "", ""},
		{"not a jsonl", writeTranscript(t, last) + ".txt", ""},
		{"no assistant rows", writeTranscript(t, `{"type":"user"}`+"\n"), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sessionModelFromTranscript(c.path); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// Only the tail is read; the partial first line after the seek is skipped.
func TestSessionModelFromTranscript_ReadsTailOfLargeFile(t *testing.T) {
	filler := `{"type":"assistant","message":{"model":"m-head","content":"` + strings.Repeat("x", 4096) + `"}}` + "\n"
	body := strings.Repeat(filler, (hookutil.TranscriptTailBytes/len(filler))+64) + `{"type":"assistant","message":{"model":"m-tail"}}` + "\n"
	if got := sessionModelFromTranscript(writeTranscript(t, body)); got != "m-tail" {
		t.Errorf("got %q, want m-tail", got)
	}
}

func TestInheritsSessionModel(t *testing.T) {
	cases := []struct {
		ti   map[string]any
		want bool
	}{
		{map[string]any{}, true},
		{map[string]any{"subagent_type": "general-purpose"}, true},
		{map[string]any{"subagent_type": "Explore"}, true},
		{map[string]any{"subagent_type": "Plan"}, true},
		{map[string]any{"subagent_type": "claude"}, true},
		{map[string]any{"subagent_type": "my-custom-reviewer"}, false},
		{map[string]any{"subagent_type": "statusline-setup"}, false},
		{map[string]any{"subagent_type": "claude-code-guide"}, false},
		{map[string]any{"model": "haiku"}, false},
	}
	for _, c := range cases {
		if got := inheritsSessionModel(c.ti); got != c.want {
			t.Errorf("inheritsSessionModel(%v) = %v, want %v", c.ti, got, c.want)
		}
	}
}

func TestCurrentModel_SubagentModelEnv(t *testing.T) {
	env := core.ClaudeCodeCaps.ModelOverrideEnv
	tr := writeTranscript(t, `{"type":"assistant","message":{"model":"m-parent"}}`+"\n")
	cases := []struct {
		name, envVal string
		set          bool
		ti           map[string]any
		want         string
	}{
		{"env set beats transcript", "m-env", true, map[string]any{}, "m-env"},
		{"env set, general-purpose", "m-env", true, map[string]any{"subagent_type": "general-purpose"}, "m-env"},
		{"env inherit falls to transcript", "inherit", true, map[string]any{}, "m-parent"},
		{"env INHERIT is case-insensitive", "Inherit", true, map[string]any{}, "m-parent"},
		{"env blank falls to transcript", "  ", true, map[string]any{}, "m-parent"},
		{"env unset falls to transcript", "", false, map[string]any{}, "m-parent"},
		{"explicit model wins", "m-env", true, map[string]any{"model": "haiku"}, "haiku"},
		{"custom type untouched", "m-env", true, map[string]any{"subagent_type": "my-custom-reviewer"}, ""},
		{"built-in with its own model untouched", "m-env", true, map[string]any{"subagent_type": "claude-code-guide"}, ""},
		// Explore and Plan run on the main model; the variable is only a default
		// for general-purpose, and reaches them only when forced onto every subagent.
		{"Explore runs on the main model", "", false, map[string]any{"subagent_type": "Explore"}, "m-parent"},
		{"Plan runs on the main model", "", false, map[string]any{"subagent_type": "Plan"}, "m-parent"},
		{"Explore ignores an unforced env default", "m-env", true, map[string]any{"subagent_type": "Explore"}, "m-parent"},
		{"Plan ignores an unforced env default", "m-env", true, map[string]any{"subagent_type": "Plan"}, "m-parent"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.set {
				t.Setenv(env, c.envVal)
			} else {
				t.Setenv(env, "")
				os.Unsetenv(env)
			}
			ev := Event{TranscriptPath: tr}
			if got := ev.currentModel(c.ti); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// CLAUDE_CODE_SUBAGENT_MODEL_FORCE puts the variable on every subagent, the
// built-in Explore and Plan included.
func TestCurrentModel_ForcedEnvReachesExploreAndPlan(t *testing.T) {
	tr := writeTranscript(t, `{"type":"assistant","message":{"model":"m-parent"}}`+"\n")
	t.Setenv(core.ClaudeCodeCaps.ModelOverrideEnv, "m-env")
	t.Setenv("CLAUDE_CODE_SUBAGENT_MODEL_FORCE", "1")
	for _, typ := range []string{"Explore", "Plan", "general-purpose"} {
		ev := Event{TranscriptPath: tr}
		if got := ev.currentModel(map[string]any{"subagent_type": typ}); got != "m-env" {
			t.Errorf("%s with the force flag: got %q, want m-env", typ, got)
		}
	}
	t.Setenv("CLAUDE_CODE_SUBAGENT_MODEL_FORCE", "0")
	ev := Event{TranscriptPath: tr}
	if got := ev.currentModel(map[string]any{"subagent_type": "Explore"}); got != "m-parent" {
		t.Errorf("force flag 0: got %q, want m-parent", got)
	}
}

// With a Fable main model Claude Code may run Explore on Opus (subscription,
// Console, gateway) or keep it on the main model (cloud providers), so the
// baseline is not knowable and stays unknown, which holds the rewrite.
func TestCurrentModel_ExploreUnderFableStaysUnknown(t *testing.T) {
	t.Setenv(core.ClaudeCodeCaps.ModelOverrideEnv, "")
	os.Unsetenv(core.ClaudeCodeCaps.ModelOverrideEnv)
	tr := writeTranscript(t, `{"type":"assistant","message":{"model":"claude-fable-5-1"}}`+"\n")
	ev := Event{TranscriptPath: tr}
	if got := ev.currentModel(map[string]any{"subagent_type": "Explore"}); got != "" {
		t.Errorf("Explore under Fable: got %q, want unknown", got)
	}
	for _, typ := range []string{"Plan", "general-purpose"} {
		if got := ev.currentModel(map[string]any{"subagent_type": typ}); got != "claude-fable-5-1" {
			t.Errorf("%s under Fable: got %q, want the main model", typ, got)
		}
	}
}
