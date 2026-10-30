// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// pluginRegistryRel is where Claude Code records installed plugins, relative to
// the target's config directory. Each key is a plugin id ("name@marketplace")
// whose installs carry an installPath. A plugin is pinned to the commit it was
// installed from, so what it carries can be older than this binary.
const pluginRegistryRel = "plugins/installed_plugins.json"

// pluginInstallDirs returns the user-scope install directories of t's plugin.
// Setup writes user-level skills, so a project or local install does not count.
// An install with no scope is read as user. A relative installPath is refused:
// it would resolve against the working directory, not the plugin cache.
func pluginInstallDirs(o setupOptions, t setupTarget) ([]string, error) {
	if t.ConfigDir == "" || t.Plugin == "" {
		return nil, errors.New("the harness has no plugin registry")
	}
	reg := filepath.Join(o.Home, filepath.FromSlash(t.ConfigDir), filepath.FromSlash(pluginRegistryRel))
	raw, err := os.ReadFile(reg)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s", setupDisplay(o.Home, reg))
	}
	var doc struct {
		Plugins map[string][]struct {
			Scope       string `json:"scope"`
			InstallPath string `json:"installPath"`
		} `json:"plugins"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return nil, fmt.Errorf("%s is not valid JSON", setupDisplay(o.Home, reg))
	}
	var dirs []string
	for _, in := range doc.Plugins[t.Plugin] {
		if (in.Scope == "" || in.Scope == "user") && filepath.IsAbs(in.InstallPath) {
			dirs = append(dirs, in.InstallPath)
		}
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("%s lists no user install of %s", setupDisplay(o.Home, reg), t.Plugin)
	}
	return dirs, nil
}

// pluginCarriedSkills reports which of installedSkills the active plugin of t
// carries, by looking for <installPath>/skills/<name>/SKILL.md. With several
// user installs a skill counts only when every one of them has it.
//
// unknown is non-empty, and carried nil, when the plugin's files cannot be read:
// then setup cannot tell what the plugin carries, and the caller must install
// nothing rather than risk a second copy of what the plugin already provides.
func pluginCarriedSkills(o setupOptions, t setupTarget) (carried map[string]bool, unknown string) {
	dirs, err := pluginInstallDirs(o, t)
	if err == nil {
		for _, d := range dirs {
			if st, serr := os.Stat(d); serr != nil || !st.IsDir() {
				err = fmt.Errorf("%s is not a directory", setupDisplay(o.Home, d))
				break
			}
		}
	}
	if err != nil {
		return nil, "the " + t.Plugin + " plugin is active but setup cannot read its install directory (" + err.Error() +
			"), so it cannot tell which Downshift skills the plugin carries and installs none; " +
			"run `claude plugin update " + t.Plugin + "` or setup again once the plugin is installed"
	}
	carried = map[string]bool{}
	for _, name := range installedSkills {
		all := true
		for _, d := range dirs {
			if st, serr := os.Stat(filepath.Join(d, "skills", name, "SKILL.md")); serr != nil || !st.Mode().IsRegular() {
				all = false
				break
			}
		}
		carried[name] = all
	}
	return carried, ""
}
