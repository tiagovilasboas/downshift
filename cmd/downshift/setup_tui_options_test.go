// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/console/setupwizard"
	"github.com/tiagovilasboas/downshift/internal/setup"
)

// roundTripView is the part of the options the wizard controls.
type roundTripView struct {
	Names                                                        []string
	Mcp, NoPlugin, NoMods, NoSessionPolicy, NoSkills, NoSteering bool
	YouShouldKnow                                                bool
	NoPolicy, KeepBinaries                                       bool
}

func viewOf(o setup.Options, f setupArgFlags) roundTripView {
	return roundTripView{o.Names, o.Mcp, o.NoPlugin, o.NoMods, o.NoSessionPolicy, o.NoSkills, o.NoSteering,
		o.YouShouldKnow, f.noPolicy, f.keepBinaries}
}

func allComponentIDs() []string {
	var ids []string
	for _, c := range setupComponents {
		ids = append(ids, c.id)
	}
	return ids
}

func TestSelectionRoundTripsThroughTheFlagParser(t *testing.T) {
	base := defaultSetupOptions(nil, nil, nil)
	cases := map[string]setupwizard.Selection{
		"all components":   {Harnesses: []string{"claude-code", "cursor"}, Components: allComponentIDs()},
		"no components":    {Harnesses: []string{"codex"}},
		"hooks only":       {Harnesses: []string{"claude-desktop"}, Components: []string{"hooks"}},
		"defaults":         {Harnesses: []string{"claude-code"}, Components: []string{"mcp", "plugin", "mods", "session-policy", "skills", "steering", "policy"}},
		"only prune":       {Harnesses: []string{"opencode"}, Components: []string{"prune"}},
		"you-should-know":  {Harnesses: []string{"claude-code"}, Components: []string{"you-should-know", "mcp"}},
		"all but steering": {Harnesses: []string{"kirocrew"}, Components: slices.DeleteFunc(allComponentIDs(), func(s string) bool { return s == "steering" })},
	}
	for name, sel := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotFlags := selectionOptions(base, sel)

			parsed := defaultSetupOptions(nil, nil, nil)
			flags, help, bad := parseSetupArgs(selectionArgs(sel), &parsed)
			if help || bad != "" {
				t.Fatalf("args %v not accepted by the parser (bad=%q)", selectionArgs(sel), bad)
			}
			if a, b := viewOf(got, gotFlags), viewOf(parsed, flags); !equalViews(a, b) {
				t.Errorf("selection -> options %+v\nargs -> options     %+v\nargs: %v", a, b, selectionArgs(sel))
			}
		})
	}
}

func equalViews(a, b roundTripView) bool {
	return slices.Equal(a.Names, b.Names) && a.Mcp == b.Mcp && a.NoPlugin == b.NoPlugin && a.NoMods == b.NoMods &&
		a.NoSessionPolicy == b.NoSessionPolicy && a.NoSkills == b.NoSkills && a.NoSteering == b.NoSteering &&
		a.YouShouldKnow == b.YouShouldKnow && a.NoPolicy == b.NoPolicy && a.KeepBinaries == b.KeepBinaries
}

func TestSelectionArgs(t *testing.T) {
	sel := setupwizard.Selection{Harnesses: []string{"claude-code"}, Components: []string{"hooks", "mcp", "policy"}}
	got := strings.Join(selectionArgs(sel), " ")
	want := "claude-code --no-plugin --no-mods --no-session-policy --no-skills --no-steering --keep-binaries"
	if got != want {
		t.Errorf("args = %q, want %q", got, want)
	}
	if cmd := setupwizard.EquivalentCommand(sel, selectionArgs); cmd != "downshift setup "+want {
		t.Errorf("equivalent command = %q", cmd)
	}
}

func TestWizardHarnessesListEveryTarget(t *testing.T) {
	o := tuiOpts(t, nil, &bytes.Buffer{}, &bytes.Buffer{})
	hs := wizardHarnesses(o)
	if len(hs) != len(setup.Targets) {
		t.Fatalf("%d harnesses, want %d", len(hs), len(setup.Targets))
	}
	byID := map[string]setupwizard.Harness{}
	for _, h := range hs {
		byID[h.ID] = h
	}
	if _, ok := byID["claude-desktop"]; !ok {
		t.Error("claude-desktop missing from the wizard")
	}
	if !byID["claude-code"].Detected || byID["cursor"].Detected {
		t.Errorf("detection wrong: %+v", byID)
	}
	o.Names = []string{"cursor"}
	for _, h := range wizardHarnesses(o) {
		if h.ID == "claude-code" && !h.Unchecked {
			t.Error("a detected harness not named on the command line must start unchecked")
		}
	}
}

func TestWizardComponentsDefaults(t *testing.T) {
	o := defaultSetupOptions(nil, nil, nil)
	o.ProjectDir = "/work/proj"
	byID := map[string]setupwizard.Component{}
	for _, c := range wizardComponents(o) {
		byID[c.ID] = c
	}
	if h := byID["hooks"]; !h.Disabled || h.DisabledReason == "" {
		t.Errorf("hooks must be a fixed item with a reason: %+v", h)
	}
	for _, id := range []string{"mcp", "plugin", "mods", "session-policy", "skills", "steering", "policy"} {
		if !byID[id].Default {
			t.Errorf("%s should default on", id)
		}
	}
	for _, id := range []string{"you-should-know", "prune"} {
		if byID[id].Default {
			t.Errorf("%s should default off", id)
		}
	}
	if !byID["prune"].Destructive {
		t.Error("prune must be marked destructive")
	}
	o.NoMods, o.ProjectDir = true, ""
	byID = map[string]setupwizard.Component{}
	for _, c := range wizardComponents(o) {
		byID[c.ID] = c
	}
	if byID["mods"].Default || byID["policy"].Default {
		t.Error("flags already on the command line must start the component off")
	}
}

func TestSetupPlanFilters(t *testing.T) {
	cases := []struct {
		name      string
		mut       func(*setup.Options)
		wantSkil  bool
		wantSteer bool
	}{
		{"default plan", func(*setup.Options) {}, true, true},
		{"no skills", func(o *setup.Options) { o.NoSkills = true }, false, true},
		{"no steering", func(o *setup.Options) { o.NoSteering = true }, true, false},
		{"neither", func(o *setup.Options) { o.NoSkills, o.NoSteering = true, true }, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errw bytes.Buffer
			o := tuiOpts(t, nil, &out, &errw)
			o.DryRun, o.TUI, o.NoMods = true, false, true // mods also live under skills/
			c.mut(&o)
			if rc := setupRun(o); rc != 0 {
				t.Fatalf("rc = %d: %s", rc, errw.String())
			}
			s := out.String()
			if got := strings.Contains(s, ".claude/skills/"); got != c.wantSkil {
				t.Errorf("skills planned = %v, want %v\n%s", got, c.wantSkil, s)
			}
			if got := strings.Contains(s, ".claude/CLAUDE.md would change"); got != c.wantSteer {
				t.Errorf("steering planned = %v, want %v\n%s", got, c.wantSteer, s)
			}
		})
	}
}

func TestParseSetupPreview(t *testing.T) {
	text := "claude-code: ~/.claude/settings.json would change\n--- a/settings.json\n+++ b/settings.json\n@@ -1 +1,2 @@\n {\n+  \"x\": 1\n  note: a note\n\n" +
		"codex: ~/.codex/hooks.json already up to date\n\nbinaries: ~/bin/downshift (built none) would be removed\n\nDry run: nothing was written.\n"
	files := parseSetupPreview(text)
	if len(files) != 2 {
		t.Fatalf("files = %+v", files)
	}
	if files[0].Path != "~/.claude/settings.json" || files[0].Lines[0] != "@@ -1 +1,2 @@" || len(files[0].Lines) != 4 {
		t.Errorf("diff file = %+v", files[0])
	}
	if files[1].Path != "notes" || len(files[1].Lines) != 2 {
		t.Errorf("notes file = %+v", files[1])
	}
	if got := parseSetupPreview("Dry run: nothing was written.\n"); len(got) != 0 {
		t.Errorf("empty preview = %+v", got)
	}
}
