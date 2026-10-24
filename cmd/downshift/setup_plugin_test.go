// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const staleSettings = `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Agent|Task", "hooks": [{"type": "command", "command": "/old/place/downshift claude-code"}]},
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "/usr/local/bin/my-guard"}]}
    ],
    "SubagentStop": [
      {"hooks": [{"type": "command", "command": "/old/place/downshift claude-code-subagent-stop"}]}
    ]
  }
}`

// fakeClaude records the commands setup runs and, like Claude Code, writes the
// plugin into settings.json when `claude plugin install` succeeds.
type fakeClaude struct {
	e           *setupEnv
	calls       []string
	hasClaude   bool
	onLoginPath bool
	loginBin    string // what a login shell resolves `downshift` to; testBin when empty
	failInstall bool
}

func (f *fakeClaude) exec() setupExec {
	return setupExec{
		LookPath: func(name string) (string, error) {
			if name == "claude" && f.hasClaude {
				return "/usr/local/bin/claude", nil
			}
			return "", errors.New("not found")
		},
		Run: func(name string, args ...string) ([]byte, error) {
			f.calls = append(f.calls, name+" "+strings.Join(args, " "))
			joined := strings.Join(args, " ")
			switch {
			case strings.Contains(joined, "command -v downshift"):
				if f.onLoginPath {
					bin := f.loginBin
					if bin == "" {
						bin = testBin
					}
					return []byte(bin + "\n"), nil
				}
				return nil, errors.New("exit 1")
			case strings.HasPrefix(joined, "plugin install"):
				if f.failInstall {
					return []byte("plugin install is not available\n"), errors.New("exit 1")
				}
				p := f.e.path(".claude/settings.json")
				raw, _ := os.ReadFile(p)
				doc, _ := parseJSONDoc(raw)
				plugins := newJobj()
				if v, ok := doc.get("enabledPlugins"); ok {
					if o, isObj := asObj(v); isObj {
						plugins = o
					}
				}
				plugins.set(args[len(args)-1], true)
				doc.set("enabledPlugins", plugins)
				out, _ := renderJSONDoc(doc)
				if err := os.WriteFile(p, out, 0o644); err != nil {
					return nil, err
				}
			}
			return nil, nil
		},
	}
}

func (f *fakeClaude) run(opts func(*setupOptions)) (code int, out, errOut string) {
	var o, er bytes.Buffer
	so := setupOptions{Home: f.e.home, Binary: testBin, In: strings.NewReader(""), Out: &o, Err: &er, Yes: true, Exec: f.exec()}
	so.Now = func() time.Time { f.e.tick++; return time.Date(2026, 10, 19, 12, 0, 0, f.e.tick*1000, time.UTC) }
	so.Names = []string{"claude-code"}
	if opts != nil {
		opts(&so)
	}
	return setupRun(so), o.String(), er.String()
}

func newClaudeEnv(t *testing.T, settings string) (*setupEnv, *fakeClaude) {
	e := newSetupEnv(t, ".claude")
	if settings != "" {
		e.write(".claude/settings.json", settings)
	}
	return e, &fakeClaude{e: e, hasClaude: true, onLoginPath: true}
}

func TestSetupInstallsPluginAndDropsSettingsHooks(t *testing.T) {
	e, f := newClaudeEnv(t, staleSettings)
	code, out, errOut := f.run(nil)
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	want := []string{"claude plugin marketplace add tiagovilasboas/downshift", "claude plugin install downshift@downshift"}
	got := strings.Join(f.calls, "\n")
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Fatalf("missing %q in calls:\n%s", w, got)
		}
	}
	doc := parseDoc(t, e.read(".claude/settings.json"))
	if _, ok := doc["enabledPlugins"].(map[string]any)["downshift@downshift"]; !ok {
		t.Fatalf("plugin not enabled: %v", doc["enabledPlugins"])
	}
	for _, c := range hookCommands(t, doc, "PreToolUse") {
		if strings.Contains(c, "downshift") {
			t.Fatalf("settings still routes spawns, the plugin would route them twice: %q", c)
		}
	}
	if cmds := hookCommands(t, doc, "PreToolUse"); len(cmds) != 1 || cmds[0] != "/usr/local/bin/my-guard" {
		t.Fatalf("a hook that is not Downshift's must stay: %v", cmds)
	}
	if hooks, _ := doc["hooks"].(map[string]any); hooks != nil {
		if _, ok := hooks["SubagentStop"]; ok {
			t.Fatalf("an event left empty must be removed: %v", hooks)
		}
	}
	if doc["model"] != "opusplan" {
		t.Fatalf("the session policy still applies: %v", doc["model"])
	}
	if !strings.Contains(out, "removed 2 Downshift hook(s)") {
		t.Fatalf("the output must say what was removed:\n%s", out)
	}
}

func TestSetupPluginFallsBackToSettingsHooks(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*fakeClaude)
		note string
	}{
		{"no claude binary", func(f *fakeClaude) { f.hasClaude = false }, "`claude` is not on PATH"},
		{"downshift not on the login PATH", func(f *fakeClaude) { f.onLoginPath = false }, "not on PATH in a login shell"},
		{"login PATH finds another downshift build", func(f *fakeClaude) { f.loginBin = "/opt/homebrew/bin/downshift" }, "would run a different build"},
		{"install fails", func(f *fakeClaude) { f.failInstall = true }, "plugin install"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, f := newClaudeEnv(t, "")
			tc.mut(f)
			code, out, errOut := f.run(nil)
			if code != 0 {
				t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
			}
			doc := parseDoc(t, e.read(".claude/settings.json"))
			// Agent|Task and Bash share the handler: two groups, one command each.
			if cmds := hookCommands(t, doc, "PreToolUse"); len(cmds) != 2 || !strings.HasSuffix(cmds[0], "downshift claude-code") || !strings.HasSuffix(cmds[1], "downshift claude-code") {
				t.Fatalf("fallback must write the settings hooks, got %v", cmds)
			}
			if !strings.Contains(out, tc.note) || !strings.Contains(out, "wiring the hooks in settings.json instead") {
				t.Fatalf("the fallback reason must be shown (%q):\n%s", tc.note, out)
			}
		})
	}
}

func TestSetupPluginDryRunInstallsNothing(t *testing.T) {
	e, f := newClaudeEnv(t, staleSettings)
	before := e.read(".claude/settings.json")
	code, out, _ := f.run(func(o *setupOptions) { o.DryRun = true })
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for _, c := range f.calls {
		if strings.Contains(c, "plugin install") || strings.Contains(c, "marketplace add") {
			t.Fatalf("a dry run must not install: %v", f.calls)
		}
	}
	if e.read(".claude/settings.json") != before {
		t.Fatal("a dry run must not write")
	}
	if !strings.Contains(out, "would run `claude plugin marketplace add") {
		t.Fatalf("the dry run must say what it would do:\n%s", out)
	}
}

func TestSetupNoPluginKeepsSettingsHooks(t *testing.T) {
	e, f := newClaudeEnv(t, "")
	code, out, errOut := f.run(func(o *setupOptions) { o.NoPlugin = true })
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if len(f.calls) != 0 {
		t.Fatalf("--no-plugin must not run claude: %v", f.calls)
	}
	doc := parseDoc(t, e.read(".claude/settings.json"))
	if cmds := hookCommands(t, doc, "PreToolUse"); len(cmds) != 2 {
		t.Fatalf("hooks expected in settings: %v", cmds)
	}
}

func TestSetupAlreadyEnabledPluginMigratesOldSettingsHooks(t *testing.T) {
	settings := strings.Replace(staleSettings, `"hooks": {`, `"enabledPlugins": {"downshift@downshift": true}, "hooks": {`, 1)
	e, f := newClaudeEnv(t, settings)
	code, out, errOut := f.run(nil)
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	for _, c := range f.calls {
		if strings.Contains(c, "plugin install downshift@downshift") {
			t.Fatalf("an enabled plugin is not installed again: %v", f.calls)
		}
	}
	doc := parseDoc(t, e.read(".claude/settings.json"))
	for _, c := range hookCommands(t, doc, "PreToolUse") {
		if strings.Contains(c, "downshift") {
			t.Fatalf("duplicate routing left behind: %q", c)
		}
	}
}

func TestSetupInstallsSiblingMods(t *testing.T) {
	e, f := newClaudeEnv(t, "")
	code, out, errOut := f.run(nil)
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	doc := parseDoc(t, e.read(".claude/settings.json"))
	plugins, _ := doc["enabledPlugins"].(map[string]any)
	for _, id := range []string{"savvy-progress@downshift"} {
		if plugins[id] != true {
			t.Fatalf("mod %s not enabled: %v\ncalls: %v", id, plugins, f.calls)
		}
	}
	if !strings.Contains(out, "installed the mods savvy-progress@downshift") {
		t.Fatalf("the output must name the mods:\n%s", out)
	}
}

func TestSetupModsSurviveRoutingFallback(t *testing.T) {
	e, f := newClaudeEnv(t, "")
	f.onLoginPath = false // the plugin falls back to settings hooks; mods do not need downshift
	if code, out, errOut := f.run(nil); code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	doc := parseDoc(t, e.read(".claude/settings.json"))
	plugins, _ := doc["enabledPlugins"].(map[string]any)
	if plugins["savvy-progress@downshift"] != true || plugins["downshift@downshift"] == true {
		t.Fatalf("want mods only, got %v", plugins)
	}
}

func TestSetupSkipsEnabledMods(t *testing.T) {
	_, f := newClaudeEnv(t, `{"enabledPlugins": {"downshift@downshift": true, "savvy-progress@downshift": true, "filetree@downshift": true, "downshift-dock@downshift": true}}`)
	if code, out, errOut := f.run(nil); code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	for _, c := range f.calls {
		if strings.Contains(c, "plugin install") || strings.Contains(c, "marketplace add") {
			t.Fatalf("enabled mods are not installed again: %v", f.calls)
		}
	}
}

// Every mod setup installs must be a plugin of the Downshift marketplace.
func TestSetupModsAreMarketplacePlugins(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", ".claude-plugin", "marketplace.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name string `json:"name"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, p := range m.Plugins {
		listed[p.Name+"@"+m.Name] = true
	}
	for _, st := range setupTargets {
		for _, id := range st.Mods {
			if !listed[id] {
				t.Errorf("%s: mod %s is not in marketplace.json", st.Name, id)
			}
		}
	}
}
