// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/skills"
)

const pluginID = "downshift@downshift"

// pluginInstall is one entry of installed_plugins.json.
type pluginInstall struct {
	Scope       string `json:"scope"`
	InstallPath string `json:"installPath"`
}

// enablePlugin turns the Downshift plugin on in the user settings, the way
// Claude Code records it.
func (e *setupEnv) enablePlugin() {
	e.t.Helper()
	e.write(".claude/settings.json", `{"enabledPlugins":{"`+pluginID+`":true}}`)
}

// registerPlugins writes the plugin registry setup reads.
func (e *setupEnv) registerPlugins(plugins map[string][]pluginInstall) {
	e.t.Helper()
	raw, err := json.Marshal(map[string]any{"version": 2, "plugins": plugins})
	if err != nil {
		e.t.Fatal(err)
	}
	e.write(".claude/plugins/installed_plugins.json", string(raw))
}

// pluginDir creates an install directory that carries the named skills.
func (e *setupEnv) pluginDir(rel string, carried ...string) string {
	e.t.Helper()
	dir := e.path(rel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.t.Fatal(err)
	}
	for _, name := range carried {
		e.write(filepath.Join(rel, "skills", name, "SKILL.md"), "---\nname: "+name+"\n---\n")
	}
	return dir
}

// installPlugin creates an install directory carrying the named skills and
// registers it under scope as the only install of the Downshift plugin.
func (e *setupEnv) installPlugin(scope, rel string, carried ...string) {
	e.t.Helper()
	dir := e.pluginDir(rel, carried...)
	e.registerPlugins(map[string][]pluginInstall{pluginID: {{scope, dir}}})
}

// writtenSkills returns the installedSkills with at least one file under the
// user skills directory, and fails when a skill is only half written.
func (e *setupEnv) writtenSkills() []string {
	e.t.Helper()
	var got []string
	for _, name := range installedSkills {
		files, err := skillFiles(name)
		if err != nil {
			e.t.Fatal(err)
		}
		present := 0
		for _, f := range files {
			if _, err := os.Stat(e.path(".claude/skills/" + f)); err == nil {
				present++
			}
		}
		if present != 0 && present != len(files) {
			e.t.Fatalf("%s is half written: %d of %d files", name, present, len(files))
		}
		if present > 0 {
			got = append(got, name)
		}
	}
	return got
}

func sameNames(a, b []string) bool {
	a, b = append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	return strings.Join(a, ",") == strings.Join(b, ",")
}

func claudeCodeTarget(t *testing.T) setupTarget {
	t.Helper()
	for _, tg := range setupTargets {
		if tg.Name == "claude-code" {
			return tg
		}
	}
	t.Fatal("no claude-code target")
	return setupTarget{}
}

func TestSetupInstallsOnlyTheSkillsTheActivePluginLacks(t *testing.T) {
	all := append([]string(nil), installedSkills...)
	cases := []struct {
		name  string
		setup func(e *setupEnv)
		want  []string // skills setup writes
		say   string   // text the plan must show
	}{
		{
			name:  "plugin carries every skill",
			setup: func(e *setupEnv) { e.installPlugin("user", "p/new", all...) },
			say:   "already carries the Downshift skills",
		},
		{
			name:  "plugin lacks downshift-effort only",
			setup: func(e *setupEnv) { e.installPlugin("user", "p/old", skills.Audit, skills.Orchestrator) },
			want:  []string{skills.Effort},
			say:   "already carries " + skills.Audit,
		},
		{
			name:  "plugin predates the skills directory",
			setup: func(e *setupEnv) { e.installPlugin("user", "p/ancient") },
			want:  all,
		},
		{
			name:  "install without a scope counts as user",
			setup: func(e *setupEnv) { e.installPlugin("", "p/noscope", skills.Audit, skills.Orchestrator) },
			want:  []string{skills.Effort},
		},
		{
			name:  "registry missing",
			setup: func(e *setupEnv) {},
			say:   "install directory",
		},
		{
			name:  "registry is not JSON",
			setup: func(e *setupEnv) { e.write(".claude/plugins/installed_plugins.json", "{not json") },
			say:   "install directory",
		},
		{
			name: "registry has no entry for the plugin",
			setup: func(e *setupEnv) {
				dir := e.pluginDir("p/other")
				e.registerPlugins(map[string][]pluginInstall{"drawio@drawio": {{"user", dir}}})
			},
			say: "install directory",
		},
		{
			name: "install directory is gone",
			setup: func(e *setupEnv) {
				e.registerPlugins(map[string][]pluginInstall{pluginID: {{"user", e.path("p/gone")}}})
			},
			say: "install directory",
		},
		{
			name: "install path is not absolute",
			setup: func(e *setupEnv) {
				e.pluginDir("p/rel")
				e.registerPlugins(map[string][]pluginInstall{pluginID: {{"user", "p/rel"}}})
			},
			say: "install directory",
		},
		{
			name:  "only a project install exists",
			setup: func(e *setupEnv) { e.installPlugin("project", "p/proj", all...) },
			say:   "install directory",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newSetupEnv(t, ".claude")
			e.enablePlugin()
			c.setup(e)
			code, out, errOut := e.run("--yes", "claude-code")
			if code != 0 {
				t.Fatalf("setup exit %d\n%s\n%s", code, out, errOut)
			}
			if got := e.writtenSkills(); !sameNames(got, c.want) {
				t.Errorf("setup wrote skills %v, want %v\n%s", got, c.want, out)
			}
			if c.say != "" && !strings.Contains(out, c.say) {
				t.Errorf("output lacks %q:\n%s", c.say, out)
			}
		})
	}
}

// The PluginActive seam (the plugin installed by this very run, or assumed in a
// dry run) goes through the same lookup as the enabledPlugins setting.
func TestPlanSkillsReadsThePluginWhenPluginActiveIsSet(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.installPlugin("user", "p/old", skills.Audit, skills.Orchestrator)
	tg := claudeCodeTarget(t)
	o := setupOptions{Home: e.home, PluginActive: map[string]bool{tg.Name: true}}
	var writes []string
	for _, p := range planSkills(o, tg) {
		if p.skip == "" && p.err == nil {
			writes = append(writes, filepath.ToSlash(setupDisplay(e.home, p.path)))
		}
	}
	for _, w := range writes {
		if !strings.Contains(w, skills.Effort+"/") {
			t.Errorf("planned %s, which the plugin carries", w)
		}
	}
	if len(writes) == 0 {
		t.Fatal("no file planned for the skill the plugin lacks")
	}
}

func TestSkillPathsForListsOnlyWhatSetupWrote(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.enablePlugin()
	e.installPlugin("user", "p/old", skills.Audit, skills.Orchestrator)
	tg := claudeCodeTarget(t)
	o := setupOptions{Home: e.home}

	if got := skillPathsFor(o, tg); len(got) != 0 {
		t.Fatalf("before setup the undo list is %v, want empty", got)
	}
	if code, out, _ := e.run("--yes", "claude-code"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	files, err := skillFiles(skills.Effort)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, f := range files {
		want = append(want, e.path(".claude/skills/"+filepath.ToSlash(f)))
	}
	got := skillPathsFor(o, tg)
	if !sameNames(got, want) {
		t.Fatalf("undo list = %v, want exactly the written files %v", got, want)
	}
}

// Undo removes what setup wrote and says nothing about a skill the plugin
// carries, even when a person keeps their own folder of that name.
func TestSetupUndoLeavesWhatThePluginProvides(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.enablePlugin()
	e.installPlugin("user", "p/old", skills.Audit, skills.Orchestrator)
	mine := ".claude/skills/" + skills.Audit + "/SKILL.md"
	e.write(mine, "mine\n")

	if code, out, _ := e.run("--yes", "claude-code"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	code, out, _ := e.run("--undo", "--yes")
	if code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if got := e.read(mine); got != "mine\n" {
		t.Fatalf("undo touched a skill setup never wrote: %q", got)
	}
	if strings.Contains(out, skills.Audit) {
		t.Errorf("undo reports on a skill the plugin provides:\n%s", out)
	}
	if _, err := os.Stat(e.path(".claude/skills/" + skills.Effort + "/SKILL.md")); !os.IsNotExist(err) {
		t.Errorf("undo left the skill setup wrote: %v", err)
	}
}

// A copy setup wrote while the plugin lacked the skill is still undone once the
// plugin catches up: the undo list follows what was written, not the plugin.
func TestSetupUndoStillRemovesACopyThePluginNowCarries(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.enablePlugin()
	e.installPlugin("user", "p/old", skills.Audit, skills.Orchestrator)
	if code, out, _ := e.run("--yes", "claude-code"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	e.pluginDir("p/old", skills.Effort) // the plugin updates

	if code, out, _ := e.run("--undo", "--yes"); code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if got := e.writtenSkills(); len(got) != 0 {
		t.Fatalf("undo left %v behind", got)
	}
}
