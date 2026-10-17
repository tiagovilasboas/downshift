// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/semantic/e5shadow"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestModelOverrideNotes(t *testing.T) {
	caps := core.AllHarnessCaps()
	if got := modelOverrideNotes(caps, envOf(nil)); len(got) != 0 {
		t.Fatalf("unset must be silent, got %v", got)
	}
	if got := modelOverrideNotes(caps, envOf(map[string]string{"CLAUDE_CODE_SUBAGENT_MODEL": "  "})); len(got) != 0 {
		t.Fatalf("blank must be silent, got %v", got)
	}
	got := modelOverrideNotes(caps, envOf(map[string]string{"CLAUDE_CODE_SUBAGENT_MODEL": "haiku"}))
	if len(got) != 1 || !strings.Contains(got[0], "CLAUDE_CODE_SUBAGENT_MODEL=haiku") || !strings.Contains(got[0], "claude-code") || !strings.Contains(got[0], "not verified") {
		t.Fatalf("notes = %v", got)
	}
	long := strings.Repeat("x", 200)
	got = modelOverrideNotes(caps, envOf(map[string]string{"CLAUDE_CODE_SUBAGENT_MODEL": long}))
	if len(got) != 1 || strings.Contains(got[0], long) || !strings.Contains(got[0], "...") {
		t.Fatalf("long value must be truncated, got %v", got)
	}
}

// A harness that declares a variable is reported without touching doctor.
func TestModelOverrideNotesAreDrivenByCapabilities(t *testing.T) {
	caps := []core.NamedCaps{
		{Harness: "future-harness", Caps: core.HarnessCapabilities{ModelOverrideEnv: "FUTURE_MODEL"}},
		{Harness: "quiet", Caps: core.HarnessCapabilities{}},
	}
	got := modelOverrideNotes(caps, envOf(map[string]string{"FUTURE_MODEL": "m1"}))
	if len(got) != 1 || !strings.Contains(got[0], "future-harness") {
		t.Fatalf("notes = %v", got)
	}
}

func TestCheckE5ShadowWarnsWhenMisconfigured(t *testing.T) {
	rep := &doctorReport{}
	checkE5Shadow(rep, doctorEnv{Getenv: envOf(map[string]string{
		e5shadow.EnableEnv: "1",
	})})
	if len(rep.Checks) != 1 || rep.Checks[0].Status != doctorWarn {
		t.Fatalf("checks = %+v", rep.Checks)
	}
}

func TestCheckE5FuseWarnsWhenMisconfigured(t *testing.T) {
	rep := &doctorReport{}
	checkE5Shadow(rep, doctorEnv{Getenv: envOf(map[string]string{
		e5shadow.IntegrateEnv: "1",
	})})
	if len(rep.Checks) != 1 || rep.Checks[0].Status != doctorWarn || rep.Checks[0].ID != "e5 fuse" {
		t.Fatalf("checks = %+v", rep.Checks)
	}
}

func TestAllHarnessCapsCoversEveryAdapter(t *testing.T) {
	want := map[string]bool{"claude-code": true, "cursor": true, "codex": true, "kirocrew": true, "antigravity": true}
	for _, c := range core.AllHarnessCaps() {
		delete(want, c.Harness)
	}
	if len(want) != 0 {
		t.Fatalf("harnesses missing from AllHarnessCaps: %v", want)
	}
}

// The companion row: absent and not recorded is silent (verbose prints ok);
// detected but not recorded warns with the enable command; recorded but
// missing warns with the disable command; both present is ok.
func TestDoctorCompanionRow(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	find := func(rep *doctorReport) *doctorCheck {
		for i := range rep.Checks {
			if rep.Checks[i].ID == "companion" {
				return &rep.Checks[i]
			}
		}
		return nil
	}
	present := func(name string) (string, error) {
		if name == "headroom" {
			return "/opt/bin/headroom", nil
		}
		return "", os.ErrNotExist
	}
	absent := func(string) (string, error) { return "", os.ErrNotExist }
	ver := func(context.Context, string, ...string) ([]byte, error) { return []byte("headroom 2.0.0"), nil }

	rep := &doctorReport{}
	checkCompanion(rep, doctorEnv{LookPath: absent}, contextopt.DefaultConfig())
	if find(rep) != nil {
		t.Fatal("absent and not recorded must be silent")
	}
	rep = &doctorReport{}
	checkCompanion(rep, doctorEnv{LookPath: absent, Verbose: true}, contextopt.DefaultConfig())
	if c := find(rep); c == nil || c.Status != doctorOK {
		t.Fatalf("verbose absent = %+v", c)
	}

	rep = &doctorReport{}
	checkCompanion(rep, doctorEnv{LookPath: present, Run: ver}, contextopt.DefaultConfig())
	if c := find(rep); c == nil || c.Status != doctorWarn || c.Fix != "downshift context enable headroom" || !strings.Contains(c.Detail, "v2.0.0") {
		t.Fatalf("detected not recorded = %+v", c)
	}

	cfg := contextopt.DefaultConfig()
	cfg.Companion = "headroom"
	rep = &doctorReport{}
	checkCompanion(rep, doctorEnv{LookPath: present, Run: ver}, cfg)
	if c := find(rep); c == nil || c.Status != doctorOK || !strings.Contains(c.Detail, "headroom wrap") {
		t.Fatalf("recorded and present = %+v", c)
	}

	rep = &doctorReport{}
	checkCompanion(rep, doctorEnv{LookPath: absent}, cfg)
	if c := find(rep); c == nil || c.Status != doctorWarn || c.Fix != "downshift context disable headroom" {
		t.Fatalf("recorded but missing = %+v", c)
	}
}

// Plugin wiring is data on the setup target. With the plugin enabled and no
// settings hooks, "hooks wired" is ok via the plugin; with both, doctor warns
// that every spawn is routed twice and names the fix.
func TestDoctorPluginWiring(t *testing.T) {
	f := newDoctorFixture(t)
	f.writeEvents(t, filepath.Join(f.home, ".downshift"), time.Hour, true)

	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), `{"enabledPlugins":{"downshift@downshift":true}}`)
	rep := buildDoctorReport(f.env)
	if rep.Failed != 0 {
		var b bytes.Buffer
		rep.write(&b, false)
		t.Fatalf("plugin alone must not fail:\n%s", b.String())
	}
	wantStatus(t, f.check(t, rep, "claude-code", "hooks wired"), doctorOK)
	if c := f.check(t, rep, "claude-code", "plugin"); c.Status != doctorOK || !strings.Contains(c.Detail, "downshift@downshift enabled") {
		t.Fatalf("plugin row = %+v", c)
	}

	both := strings.TrimSuffix(claudeSettings(f.bin), "}") + `,"enabledPlugins":{"downshift@downshift":true}}`
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), both)
	rep = buildDoctorReport(f.env)
	c := f.check(t, rep, "claude-code", "plugin")
	if c.Status != doctorWarn || !strings.Contains(c.Detail, "routed twice") || !strings.Contains(c.Fix, "downshift setup --undo") {
		t.Fatalf("double wiring = %+v", c)
	}
	wantStatus(t, f.check(t, rep, "claude-code", "hooks wired"), doctorOK)

	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), `{"enabledPlugins":{"downshift@downshift":false}}`)
	rep = buildDoctorReport(f.env)
	wantStatus(t, f.check(t, rep, "claude-code", "hooks wired"), doctorFail)
	for _, c := range rep.Checks {
		if c.Harness == "claude-code" && c.ID == "plugin" {
			t.Fatal("a disabled plugin must not add a row")
		}
	}
}
