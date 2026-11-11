// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

func TestReadFeatureFlag(t *testing.T) {
	flag := setup.FeatureFlag{File: "h/config.toml", Table: "features", Key: "hooks", Aliases: []string{"legacy_hooks"}}
	for _, tc := range []struct {
		name, body, want string
		wantErr          bool
	}{
		{name: "no file", body: "", want: featureUnset},
		{name: "no table", body: "model = \"x\"\n", want: featureUnset},
		{name: "on", body: "[features]\nhooks = true\n", want: featureOn},
		{name: "off", body: "[features]\nhooks = false\n", want: featureOff},
		{name: "dotted key", body: "features.hooks = true\n", want: featureOn},
		{name: "legacy alias", body: "[features]\nlegacy_hooks = true\n", want: featureOn},
		{name: "current key before alias", body: "[features]\nhooks = false\nlegacy_hooks = true\n", want: featureOff},
		{name: "table enabled", body: "[features.hooks]\nenabled = true\n", want: featureOn},
		{name: "table disabled", body: "[features.hooks]\nenabled = false\n", want: featureOff},
		{name: "table without enabled", body: "[features.hooks]\nmode = \"x\"\n", want: featureUnset},
		{name: "other key only", body: "[features]\nother = true\n", want: featureUnset},
		{name: "not a boolean", body: "[features]\nhooks = \"yes\"\n", wantErr: true},
		{name: "does not parse", body: "[features\nhooks = true\n", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			if tc.body != "" {
				writeFile(t, filepath.Join(home, filepath.FromSlash(flag.File)), tc.body)
			}
			got, err := readFeatureFlag(home, flag)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Fatalf("state = %q, want %q", got, tc.want)
			}
		})
	}
}

func (f *doctorFixture) writeCodexHooks(t *testing.T) {
	t.Helper()
	bin := strings.ReplaceAll(f.bin, `\`, `\\`)
	writeFile(t, filepath.Join(f.home, ".codex", "hooks.json"),
		fmt.Sprintf(`{"hooks":{"PreToolUse":[{"matcher":"(^Agent$|.*spawn_agent$)","hooks":[{"type":"command","command":"%s codex"}]}]}}`, bin))
}

// J1: Codex hooks are wired but Codex never ran one. The maintainer's case:
// the flag is off (or the hooks are untrusted), and doctor must name the
// exact fix: the flag line, /hooks, and a new session.
func TestDoctor_CodexHooksNeverRanNamesTheFix(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		flagStatus   string
		flagInFix    bool
	}{
		{name: "flag off", config: "[features]\nhooks = false\n", flagStatus: doctorWarn, flagInFix: true},
		{name: "legacy flag off", config: "[features]\ncodex_hooks = false\n", flagStatus: doctorWarn, flagInFix: true},
		{name: "flag unset", config: "", flagStatus: doctorOK, flagInFix: true},
		{name: "flag on, hooks untrusted", config: "[features]\nhooks = true\nmulti_agent_v2 = true\n", flagStatus: doctorOK, flagInFix: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDoctorFixture(t)
			f.writeCodexHooks(t)
			if tc.config != "" {
				writeFile(t, filepath.Join(f.home, ".codex", "config.toml"), tc.config)
			}
			rep := buildDoctorReport(f.env)

			wantStatus(t, f.check(t, rep, "codex", "feature hooks"), tc.flagStatus)

			last := f.check(t, rep, "codex", "last hook event")
			wantStatus(t, last, doctorWarn)
			if !strings.Contains(last.Detail, "has never run one") {
				t.Errorf("detail %q does not say the hooks never ran", last.Detail)
			}
			for _, want := range []string{"/hooks", "new Codex session"} {
				if !strings.Contains(last.Fix, want) {
					t.Errorf("fix %q misses %q", last.Fix, want)
				}
			}
			if got := strings.Contains(last.Fix, "[features] hooks = true"); got != tc.flagInFix {
				t.Errorf("flag line in fix = %v, want %v: %q", got, tc.flagInFix, last.Fix)
			}
		})
	}
}

func TestDoctor_CodexMultiAgentFlag(t *testing.T) {
	for config, want := range map[string]string{
		"":                                    "warn",
		"[features]\nmulti_agent_v2 = true\n": "ok",
		"[features.multi_agent_v2]\nenabled = true\n":  "ok",
		"[features.multi_agent_v2]\nenabled = false\n": "warn",
	} {
		f := newDoctorFixture(t)
		f.writeCodexHooks(t)
		if config != "" {
			writeFile(t, filepath.Join(f.home, ".codex", "config.toml"), config)
		}
		c := f.check(t, buildDoctorReport(f.env), "codex", "feature multi_agent_v2")
		wantStatus(t, c, want)
		if want == doctorWarn && !strings.Contains(c.Fix, "[features] multi_agent_v2 = true") {
			t.Errorf("config %q: fix %q", config, c.Fix)
		}
	}
}

// A recorded Codex event keeps the generic, passing last-event check.
func TestDoctor_CodexWithEventKeepsLastEventOK(t *testing.T) {
	f := newDoctorFixture(t)
	f.writeCodexHooks(t)
	ts := f.started.Add(-time.Minute).Format(time.RFC3339Nano)
	writeFile(t, filepath.Join(f.home, ".downshift", "events.jsonl"),
		fmt.Sprintf(`{"correlation_id":"c1","timestamp":%q,"harness":"codex","verdict":"OK","outcome":"allow"}`, ts)+"\n")
	wantStatus(t, f.check(t, buildDoctorReport(f.env), "codex", "last hook event"), doctorOK)
}

// A harness that declares no flags and no activation step keeps the generic fix.
func TestDoctor_NoActivationKeepsGenericFix(t *testing.T) {
	f := newDoctorFixture(t)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))
	c := f.check(t, buildDoctorReport(f.env), "claude-code", "last hook event")
	if !strings.Contains(c.Fix, "restart claude-code") || strings.Contains(c.Detail, "never run") {
		t.Errorf("claude-code last event: %q / %q", c.Detail, c.Fix)
	}
}
