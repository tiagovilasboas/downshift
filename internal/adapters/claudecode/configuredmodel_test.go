// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

var cfgCat = catalog.Load()

func familyID(t *testing.T, native string) string {
	t.Helper()
	m, ok := cfgCat.LookupByID(harnessID, native)
	if !ok {
		t.Fatalf("catalog has no %q", native)
	}
	return m.ID
}

// settingsFile writes content to dir/.claude/<name>.
func settingsFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude", name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestConfiguredModel(t *testing.T) {
	sonnet, opus, haiku, fable := familyID(t, "sonnet"), familyID(t, "opus"), familyID(t, "haiku"), familyID(t, "fable")
	type files struct{ local, project, user string }
	cases := []struct {
		name  string
		env   string
		f     files
		mode  string
		want  string
		noRes bool
	}{
		{name: "env beats settings", env: "opus", f: files{project: `{"model":"haiku"}`}, want: opus},
		{name: "local beats project beats user", f: files{local: `{"model":"haiku"}`, project: `{"model":"opus"}`, user: `{"model":"sonnet"}`}, want: haiku},
		{name: "project beats user", f: files{project: `{"model":"opus"}`, user: `{"model":"sonnet"}`}, want: opus},
		{name: "user only", f: files{user: `{"model":"sonnet"}`}, want: sonnet},
		{name: "local without model falls to project", f: files{local: `{"permissions":{}}`, project: `{"model":"opus"}`}, want: opus},
		{name: "alias sonnet", f: files{project: `{"model":"sonnet"}`}, want: sonnet},
		{name: "alias opus", f: files{project: `{"model":"opus"}`}, want: opus},
		{name: "alias haiku", f: files{project: `{"model":"haiku"}`}, want: haiku},
		{name: "alias fable", f: files{project: `{"model":"fable"}`}, want: fable},
		{name: "alias is case-insensitive", f: files{project: `{"model":"Sonnet"}`}, want: sonnet},
		{name: "opusplan under plan mode is opus", f: files{project: `{"model":"opusplan"}`}, mode: "plan", want: opus},
		{name: "opusplan under default mode is sonnet", f: files{project: `{"model":"opusplan"}`}, mode: "default", want: sonnet},
		{name: "opusplan without a mode is sonnet", f: files{project: `{"model":"opusplan"}`}, want: sonnet},
		{name: "full id passes through", f: files{project: `{"model":"claude-opus-9-9"}`}, want: "claude-opus-9-9"},
		{name: "default is unknown", f: files{project: `{"model":"default"}`}, want: ""},
		{name: "unknown alias is unknown", f: files{project: `{"model":"banana"}`}, want: ""},
		{name: "unknown env does not fall through", env: "banana", f: files{project: `{"model":"opus"}`}, want: ""},
		{name: "invalid JSON is skipped", f: files{local: `{not json`, project: `{"model":"opus"}`}, want: opus},
		{name: "invalid JSON everywhere is unknown", f: files{local: `{`, project: `nope`, user: `[]`}, want: ""},
		{name: "no settings at all is unknown", want: ""},
		{name: "no resolver, no fallback", f: files{project: `{"model":"sonnet"}`}, want: "", noRes: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cwd, userDir := t.TempDir(), t.TempDir()
			t.Setenv("CLAUDE_CONFIG_DIR", userDir)
			t.Setenv(configuredModelEnv, c.env)
			if c.f.local != "" {
				settingsFile(t, cwd, "settings.local.json", c.f.local)
			}
			if c.f.project != "" {
				settingsFile(t, cwd, "settings.json", c.f.project)
			}
			if c.f.user != "" {
				if err := os.WriteFile(filepath.Join(userDir, "settings.json"), []byte(c.f.user), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var res core.Resolver = cfgCat
			if c.noRes {
				res = nil
			}
			if got := configuredModel(cwd, c.mode, res); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestConfiguredModel_UserDirFollowsHomeWithoutConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv(configuredModelEnv, "")
	settingsFile(t, home, "settings.json", `{"model":"haiku"}`)
	if got := configuredModel(t.TempDir(), "", cfgCat); got != familyID(t, "haiku") {
		t.Errorf("got %q, want the haiku id", got)
	}
}

func TestResolveModel_Fallback(t *testing.T) {
	sonnet := familyID(t, "sonnet")
	fableID := familyID(t, "fable")
	withModel := writeTranscript(t, `{"type":"assistant","message":{"model":"m-transcript"}}`+"\n")
	empty := writeTranscript(t, "")
	cases := []struct {
		name     string
		setting  string
		ti       map[string]any
		ev       Event
		wantID   string
		wantFlag bool
	}{
		{"empty transcript uses the setting", "sonnet", map[string]any{"subagent_type": "general-purpose"}, Event{TranscriptPath: empty}, sonnet, true},
		{"no transcript path uses the setting", "sonnet", map[string]any{}, Event{}, sonnet, true},
		{"transcript model wins", "sonnet", map[string]any{}, Event{TranscriptPath: withModel}, "m-transcript", false},
		{"Explore under a Fable setting stays unknown", "fable", map[string]any{"subagent_type": "Explore"}, Event{TranscriptPath: empty}, "", false},
		{"Plan under a Fable setting inherits", "fable", map[string]any{"subagent_type": "Plan"}, Event{TranscriptPath: empty}, fableID, true},
		{"custom type has no fallback", "sonnet", map[string]any{"subagent_type": "my-reviewer"}, Event{TranscriptPath: empty}, "", false},
		{"built-in with its own model has no fallback", "sonnet", map[string]any{"subagent_type": "claude-code-guide"}, Event{TranscriptPath: empty}, "", false},
		{"explicit Task model wins", "sonnet", map[string]any{"model": "haiku"}, Event{TranscriptPath: empty}, "haiku", false},
		{"payload model wins", "sonnet", map[string]any{}, Event{Model: "m-payload", TranscriptPath: empty}, "m-payload", false},
		{"unusable setting stays unknown", "default", map[string]any{}, Event{TranscriptPath: empty}, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cwd := t.TempDir()
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			t.Setenv(configuredModelEnv, "")
			t.Setenv(core.ClaudeCodeCaps.ModelOverrideEnv, "")
			settingsFile(t, cwd, "settings.json", `{"model":"`+c.setting+`"}`)
			c.ev.Cwd = cwd
			got, flag := c.ev.resolveModel(c.ti, cfgCat)
			if got != c.wantID || flag != c.wantFlag {
				t.Errorf("got (%q, %v), want (%q, %v)", got, flag, c.wantID, c.wantFlag)
			}
		})
	}
}

// With the model already in the transcript the settings are never read: a
// settings file that would change the answer, and one that cannot be read at
// all, make no difference.
func TestResolveModel_FallbackNeverRunsWhenTheTranscriptHasTheModel(t *testing.T) {
	cwd := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv(configuredModelEnv, "opus")
	t.Setenv(core.ClaudeCodeCaps.ModelOverrideEnv, "")
	settingsFile(t, cwd, "settings.json", `{"model":"haiku"}`)
	tr := writeTranscript(t, `{"type":"assistant","message":{"model":"m-transcript"}}`+"\n")
	ev := Event{Cwd: cwd, TranscriptPath: tr}
	got, flag := ev.resolveModel(map[string]any{}, cfgCat)
	if got != "m-transcript" || flag {
		t.Errorf("got (%q, %v), want (m-transcript, false)", got, flag)
	}
	if got := ev.currentModel(map[string]any{}); got != "m-transcript" {
		t.Errorf("currentModel = %q", got)
	}
}
