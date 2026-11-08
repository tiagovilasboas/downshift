// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// The installed-plugin hook check for `doctor`. A plugin is pinned to the
// commit it was installed from, so the hooks it carries can be older than this
// binary: a hook this binary expects may be missing, and a hook that was
// removed (the retired `dci` hook) may still run. The "hooks wired" row counts
// the hooks this binary ships, so only the installed hooks/hooks.json can show
// the difference.
//
// The registry path comes from the setup target (ConfigDir, Plugin, Hooks,
// Retired): nothing here names a harness. The check only reads files.

// pluginHooksRel is where a plugin keeps its hooks, relative to its install path.
const pluginHooksRel = "hooks/hooks.json"

// installedPlugin is one entry of a plugin id in installed_plugins.json.
type installedPlugin struct {
	Scope        string `json:"scope"`
	InstallPath  string `json:"installPath"`
	Version      string `json:"version"`
	GitCommitSha string `json:"gitCommitSha"`
}

// pluginHookCmd is one command found in an installed hooks.json.
type pluginHookCmd struct {
	Event, Matcher, Command string
}

// pluginHooksDiff is how the installed hooks differ from the ones this binary ships.
type pluginHooksDiff struct {
	Missing, Retired, Extra []string
}

func (d pluginHooksDiff) empty() bool {
	return len(d.Missing)+len(d.Retired)+len(d.Extra) == 0
}

// checkInstalledPluginHooks compares the hooks of the installed plugin with the
// hooks this binary ships and reports one "plugin hooks" row per user install.
// enabled says the settings file enables the plugin: then a plugin that is not
// installed is reported. Without the plugin nothing is reported, the hooks live
// in the settings file and the "hooks wired" row covers them.
func checkInstalledPluginHooks(add func(id, status, detail, fix string), home, harness string, enabled bool) {
	t, ok := pluginTarget(harness)
	if !ok {
		return
	}
	reg := filepath.Join(home, filepath.FromSlash(t.ConfigDir), filepath.FromSlash(setup.PluginRegistryRel))
	installs, err := readPluginInstalls(reg, t.Plugin)
	installFix := "/plugin install " + strings.SplitN(t.Plugin, "@", 2)[0] + " --marketplace " + setup.PluginMarketplaceSource
	switch {
	case !enabled && (err != nil || len(installs) == 0):
		return
	case err != nil:
		add("plugin hooks", doctorWarn, t.Plugin+" is enabled but "+err.Error(), installFix)
		return
	case len(installs) == 0:
		add("plugin hooks", doctorWarn, fmt.Sprintf("%s is enabled but %s lists no user install of it", t.Plugin, reg), installFix)
		return
	}
	updateFix := "claude plugin update " + t.Plugin + ", then open a new session"
	for _, in := range installs {
		commit := in.commit()
		diff, err := diffInstalledPluginHooks(in.InstallPath, t)
		switch {
		case err != nil:
			add("plugin hooks", doctorWarn, fmt.Sprintf("installed plugin at %s: %v", commit, err), updateFix)
		case diff.empty():
			add("plugin hooks", doctorOK,
				fmt.Sprintf("installed plugin at %s carries the %d hooks this binary ships", commit, len(t.Hooks)), "")
		default:
			add("plugin hooks", doctorWarn, fmt.Sprintf("installed plugin at %s differs from this binary: %s", commit, diff), updateFix)
		}
	}
}

// pluginTarget returns the setup target of harness when it can load a plugin.
func pluginTarget(harness string) (setup.Target, bool) {
	for _, t := range setup.Targets {
		if t.Name == harness && t.Plugin != "" && t.ConfigDir != "" {
			return t, true
		}
	}
	return setup.Target{}, false
}

// readPluginInstalls returns the user-scope installs of plugin from the
// registry file. A missing or invalid registry is an error; a registry without
// the plugin is an empty list. A relative installPath is skipped: it would
// resolve against the working directory, not the plugin cache.
func readPluginInstalls(reg, plugin string) ([]installedPlugin, error) {
	raw, err := os.ReadFile(reg)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s", reg)
	}
	var doc struct {
		Plugins map[string][]installedPlugin `json:"plugins"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return nil, fmt.Errorf("%s is not valid JSON", reg)
	}
	var out []installedPlugin
	for _, in := range doc.Plugins[plugin] {
		if (in.Scope == "" || in.Scope == "user") && filepath.IsAbs(in.InstallPath) {
			out = append(out, in)
		}
	}
	return out, nil
}

// commit names the installed commit for a message.
func (in installedPlugin) commit() string {
	switch {
	case len(in.GitCommitSha) > 12:
		return "commit " + in.GitCommitSha[:12]
	case in.GitCommitSha != "":
		return "commit " + in.GitCommitSha
	case in.Version != "":
		return "version " + in.Version + " (no commit recorded)"
	}
	return "an unknown commit"
}

// diffInstalledPluginHooks reads <installPath>/hooks/hooks.json and compares it
// with the hooks of t.
func diffInstalledPluginHooks(installPath string, t setup.Target) (pluginHooksDiff, error) {
	file := filepath.Join(installPath, filepath.FromSlash(pluginHooksRel))
	raw, err := os.ReadFile(file)
	if err != nil {
		return pluginHooksDiff{}, fmt.Errorf("cannot read %s", file)
	}
	got, err := parsePluginHooks(raw)
	if err != nil {
		return pluginHooksDiff{}, fmt.Errorf("%s is not a hooks file: %v", file, err)
	}
	return comparePluginHooks(got, t), nil
}

// parsePluginHooks flattens the nested hooks.json shape (event, matcher group,
// commands) into a sorted list, so a report does not depend on map order.
func parsePluginHooks(raw []byte) ([]pluginHookCmd, error) {
	var doc struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var out []pluginHookCmd
	for event, groups := range doc.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				out = append(out, pluginHookCmd{Event: event, Matcher: g.Matcher, Command: h.Command})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Event != b.Event {
			return a.Event < b.Event
		}
		if a.Matcher != b.Matcher {
			return a.Matcher < b.Matcher
		}
		return a.Command < b.Command
	})
	return out, nil
}

// comparePluginHooks sorts the installed commands against the hooks this binary
// ships: a shipped hook with no installed command is missing; an installed
// command that matches a retired hook is retired; any other unmatched command
// is extra.
func comparePluginHooks(got []pluginHookCmd, t setup.Target) pluginHooksDiff {
	var d pluginHooksDiff
	for _, want := range t.Hooks {
		found := false
		for _, g := range got {
			if isInstalledHook(g, []setup.Hook{want}, true) {
				found = true
				break
			}
		}
		if !found {
			d.Missing = append(d.Missing, hookLabel(want.Event, want.Matcher, "downshift "+want.Args))
		}
	}
	for _, g := range got {
		if isInstalledHook(g, t.Hooks, true) {
			continue
		}
		label := hookLabel(g.Event, g.Matcher, g.Command)
		if isInstalledHook(g, t.Retired, false) {
			d.Retired = append(d.Retired, label)
		} else {
			d.Extra = append(d.Extra, label)
		}
	}
	return d
}

// isInstalledHook reports whether installed command g is one of hooks: same
// event and the same Downshift subcommand, and the same matcher when
// withMatcher is set. A retired hook is matched without the matcher: the one an
// old plugin gave it is not part of what makes it retired.
func isInstalledHook(g pluginHookCmd, hooks []setup.Hook, withMatcher bool) bool {
	for _, h := range hooks {
		if g.Event == h.Event && (!withMatcher || g.Matcher == h.Matcher) && setup.IsDownshiftCommand(g.Command, h.Args) {
			return true
		}
	}
	return false
}

func hookLabel(event, matcher, command string) string {
	if matcher == "" {
		return event + " " + command
	}
	return event + "(" + matcher + ") " + command
}

func (d pluginHooksDiff) String() string {
	var parts []string
	for _, p := range []struct {
		name  string
		items []string
	}{{"missing", d.Missing}, {"retired", d.Retired}, {"extra", d.Extra}} {
		if len(p.items) > 0 {
			parts = append(parts, p.name+": "+strings.Join(p.items, ", "))
		}
	}
	return strings.Join(parts, "; ")
}
