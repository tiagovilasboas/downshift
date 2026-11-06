// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/setup"
)

// The installed-plugin hook check reads ~/.claude/plugins/installed_plugins.json
// and the installed hooks/hooks.json. Every test builds both under a temp HOME;
// the real HOME is never read.

const testPluginSha = "f76cb0b331cc1ebd9fbf9bb6da313e233e5bc448"

// shippedPluginHooks is the plugin's own hooks/hooks.json from this repository,
// the file a current install carries.
func shippedPluginHooks(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func pluginFixtureHooks(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "plugin_hooks", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// registerPluginInstall writes the registry with one install of the Downshift
// plugin whose hooks/hooks.json is hooks (nil writes no hooks file).
func registerPluginInstall(t *testing.T, home string, in map[string]any, hooks []byte) {
	t.Helper()
	const pluginID = "downshift@downshift"
	if _, ok := in["installPath"]; !ok {
		in["installPath"] = filepath.Join(home, ".claude", "plugins", "cache", "downshift", "downshift", "f76cb0b331cc")
	}
	if hooks != nil {
		writeFile(t, filepath.Join(in["installPath"].(string), "hooks", "hooks.json"), string(hooks))
	}
	reg, err := json.Marshal(map[string]any{"version": 2, "plugins": map[string]any{pluginID: []any{in}}})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), string(reg))
}

func pluginHookRows(home, harness string, enabled bool) []mcpRow {
	var rows []mcpRow
	checkInstalledPluginHooks(func(id, status, detail, fix string) {
		if id != "plugin hooks" {
			panic("unexpected row " + id)
		}
		rows = append(rows, mcpRow{status, detail, fix})
	}, home, harness, enabled)
	return rows
}

func wantOneRow(t *testing.T, rows []mcpRow) mcpRow {
	t.Helper()
	if len(rows) != 1 {
		t.Fatalf("want one plugin hooks row, got %+v", rows)
	}
	return rows[0]
}

func wantAll(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(text, p) {
			t.Errorf("%q does not contain %q", text, p)
		}
	}
}

func TestPluginHooksMatchTheShippedFile(t *testing.T) {
	home := t.TempDir()
	registerPluginInstall(t, home, map[string]any{"scope": "user", "gitCommitSha": testPluginSha}, shippedPluginHooks(t))
	row := wantOneRow(t, pluginHookRows(home, "claude-code", true))
	if row.status != doctorOK || row.fix != "" {
		t.Fatalf("row = %+v, want ok with no fix", row)
	}
	wantAll(t, row.detail, "commit f76cb0b331cc", "carries the 8 hooks this binary ships")
	if strings.Contains(row.detail, testPluginSha) {
		t.Errorf("detail should name the short commit, got %q", row.detail)
	}
}

func TestPluginHooksWarnOnRetiredAndMissing(t *testing.T) {
	home := t.TempDir()
	registerPluginInstall(t, home, map[string]any{"scope": "user", "gitCommitSha": testPluginSha}, pluginFixtureHooks(t, "retired_dci.json"))
	row := wantOneRow(t, pluginHookRows(home, "claude-code", true))
	if row.status != doctorWarn {
		t.Fatalf("status = %q, want warn: %+v", row.status, row)
	}
	wantAll(t, row.detail,
		"commit f76cb0b331cc",
		"missing: SubagentStop downshift claude-code-subagent-stop",
		"retired: UserPromptSubmit downshift dci hook claude-code")
	if strings.Contains(row.detail, "extra:") {
		t.Errorf("a retired hook must not also be reported as extra: %q", row.detail)
	}
	wantAll(t, row.fix, "claude plugin update downshift@downshift", "new session")
}

func TestPluginHooksWarnOnExtra(t *testing.T) {
	home := t.TempDir()
	registerPluginInstall(t, home, map[string]any{"scope": "user", "gitCommitSha": testPluginSha}, pluginFixtureHooks(t, "extra_command.json"))
	row := wantOneRow(t, pluginHookRows(home, "claude-code", true))
	if row.status != doctorWarn {
		t.Fatalf("status = %q, want warn: %+v", row.status, row)
	}
	// A shipped subcommand under a matcher this binary does not wire is extra
	// too, and so is a command that is not Downshift at all.
	wantAll(t, row.detail, "extra: PostToolUse(Bash) other-tool audit, PreToolUse(Edit) downshift claude-code")
	if strings.Contains(row.detail, "missing:") || strings.Contains(row.detail, "retired:") {
		t.Errorf("only extra commands expected: %q", row.detail)
	}
}

func TestPluginHooksNameVersionWhenNoCommitIsRecorded(t *testing.T) {
	home := t.TempDir()
	registerPluginInstall(t, home, map[string]any{"version": "1.2.3"}, shippedPluginHooks(t))
	row := wantOneRow(t, pluginHookRows(home, "claude-code", true))
	wantAll(t, row.detail, "version 1.2.3 (no commit recorded)")
	if row.status != doctorOK {
		t.Errorf("an install with no scope is a user install: %+v", row)
	}
}

func TestPluginHooksWarnWhenTheInstalledHooksFileIsUnusable(t *testing.T) {
	for name, hooks := range map[string][]byte{"missing": nil, "invalid": []byte("{not json")} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			registerPluginInstall(t, home, map[string]any{"scope": "user", "gitCommitSha": testPluginSha}, hooks)
			row := wantOneRow(t, pluginHookRows(home, "claude-code", true))
			if row.status != doctorWarn || !strings.Contains(row.detail, "commit f76cb0b331cc") || row.fix == "" {
				t.Errorf("row = %+v, want a warn naming the commit with a fix", row)
			}
			wantAll(t, row.detail, filepath.Join("hooks", "hooks.json"))
		})
	}
}

func TestPluginHooksEnabledButNotInstalled(t *testing.T) {
	fix := "/plugin install downshift --marketplace " + setup.PluginMarketplaceSource
	t.Run("no registry", func(t *testing.T) {
		row := wantOneRow(t, pluginHookRows(t.TempDir(), "claude-code", true))
		if row.status != doctorWarn || row.fix != fix {
			t.Errorf("row = %+v", row)
		}
		wantAll(t, row.detail, "downshift@downshift is enabled but cannot read")
	})
	t.Run("registry without the plugin", func(t *testing.T) {
		home := t.TempDir()
		writeFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), `{"version":2,"plugins":{}}`)
		row := wantOneRow(t, pluginHookRows(home, "claude-code", true))
		if row.status != doctorWarn || row.fix != fix {
			t.Errorf("row = %+v", row)
		}
		wantAll(t, row.detail, "lists no user install")
	})
	t.Run("invalid registry", func(t *testing.T) {
		home := t.TempDir()
		writeFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), `{nope`)
		row := wantOneRow(t, pluginHookRows(home, "claude-code", true))
		wantAll(t, row.detail, "is not valid JSON")
	})
}

func TestPluginHooksSilentWithoutThePlugin(t *testing.T) {
	// The settings file wires the hooks and the plugin is not enabled: there is
	// nothing installed to compare, and "hooks wired" covers the settings hooks.
	if rows := pluginHookRows(t.TempDir(), "claude-code", false); len(rows) != 0 {
		t.Errorf("rows = %+v, want none", rows)
	}
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), `{"version":2,"plugins":{}}`)
	if rows := pluginHookRows(home, "claude-code", false); len(rows) != 0 {
		t.Errorf("rows = %+v, want none", rows)
	}
}

func TestPluginHooksSkipOtherHarnessesAndInstalls(t *testing.T) {
	home := t.TempDir()
	registerPluginInstall(t, home, map[string]any{"scope": "user", "gitCommitSha": testPluginSha}, pluginFixtureHooks(t, "retired_dci.json"))
	// A harness with no plugin system is never checked.
	for _, h := range []string{"cursor", "codex", "kirocrew", ""} {
		if rows := pluginHookRows(home, h, true); len(rows) != 0 {
			t.Errorf("harness %q reported %+v", h, rows)
		}
	}
	// A project-scope install and a relative installPath are not user installs.
	project := t.TempDir()
	registerPluginInstall(t, home, map[string]any{"scope": "project", "installPath": project, "gitCommitSha": testPluginSha}, shippedPluginHooks(t))
	row := wantOneRow(t, pluginHookRows(home, "claude-code", true))
	wantAll(t, row.detail, "lists no user install")
	registerPluginInstall(t, home, map[string]any{"scope": "user", "installPath": "cache/downshift"}, nil)
	row = wantOneRow(t, pluginHookRows(home, "claude-code", true))
	wantAll(t, row.detail, "lists no user install")
}

// The doctor report carries the row, under the claude-code harness, next to
// the "hooks wired" row that only counts this binary's hooks.
func TestDoctorReportIncludesPluginHooks(t *testing.T) {
	f := newDoctorFixture(t)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), `{"enabledPlugins":{"downshift@downshift":true}}`)
	registerPluginInstall(t, f.home, map[string]any{"scope": "user", "gitCommitSha": testPluginSha}, pluginFixtureHooks(t, "retired_dci.json"))

	rep := buildDoctorReport(f.env)

	wired := f.check(t, rep, "claude-code", "hooks wired")
	if wired.Status != doctorOK {
		t.Errorf("hooks wired = %+v: it counts this binary's hooks and stays ok", wired)
	}
	row := f.check(t, rep, "claude-code", "plugin hooks")
	if row.Status != doctorWarn {
		t.Fatalf("plugin hooks = %+v, want warn", row)
	}
	wantAll(t, row.Detail, "commit f76cb0b331cc", "retired: UserPromptSubmit downshift dci hook claude-code")
}

func TestDoctorReportPluginHooksOkForTheShippedFile(t *testing.T) {
	f := newDoctorFixture(t)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), `{"enabledPlugins":{"downshift@downshift":true}}`)
	registerPluginInstall(t, f.home, map[string]any{"scope": "user", "gitCommitSha": testPluginSha}, shippedPluginHooks(t))

	rep := buildDoctorReport(f.env)

	if row := f.check(t, rep, "claude-code", "plugin hooks"); row.Status != doctorOK {
		t.Errorf("plugin hooks = %+v, want ok", row)
	}
}
