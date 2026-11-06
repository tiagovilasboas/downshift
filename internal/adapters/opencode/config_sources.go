// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Environment variables OpenCode reads configuration from.
const (
	envConfigFile    = "OPENCODE_CONFIG"
	envConfigDir     = "OPENCODE_CONFIG_DIR"
	envConfigContent = "OPENCODE_CONFIG_CONTENT"
	envPermission    = "OPENCODE_PERMISSION"
	envXDGConfigHome = "XDG_CONFIG_HOME"
)

// permissionRulesPresent reports whether any place OpenCode reads a config
// from holds a permission that is not allow-only, or holds something this
// check cannot read. Any doubt keeps the command unchanged: an empty
// directory, an unreadable or oversized file, a link, a managed preference.
// The shell can run in a directory other than the session's, so each given
// directory is checked, with every ancestor up to the filesystem root.
func permissionRulesPresent(directories ...string) bool {
	if len(directories) == 0 {
		return true
	}
	for _, d := range directories {
		if d == "" {
			return true
		}
	}
	if inlineRestrictive(envConfigContent, false) || inlineRestrictive(envPermission, true) {
		return true
	}
	if path := os.Getenv(envConfigFile); path != "" && configFileRestrictive(path) {
		return true
	}
	if managedPreferencePresent() {
		return true
	}
	seen := map[string]bool{}
	for _, dir := range globalConfigDirs() {
		if !seen[dir] && configDirRestrictive(dir) {
			return true
		}
		seen[dir] = true
	}
	for _, directory := range directories {
		for _, dir := range projectDirs(directory) {
			if seen[dir] {
				continue
			}
			seen[dir] = true
			for _, name := range configNames {
				if configFileRestrictive(filepath.Join(dir, name)) {
					return true
				}
			}
			if configDirRestrictive(filepath.Join(dir, ".opencode")) {
				return true
			}
		}
	}
	return false
}

var configNames = []string{"opencode.jsonc", "opencode.json"}

// globalConfigDirs lists the directories laid out like an OpenCode config dir
// that do not depend on the project: the global ones, the managed one and
// OPENCODE_CONFIG_DIR.
func globalConfigDirs() []string {
	var dirs []string
	if xdg := os.Getenv(envXDGConfigHome); xdg != "" {
		dirs = append(dirs, filepath.Join(xdg, "opencode"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".config", "opencode"))
	}
	if managed := managedConfigDir(); managed != "" {
		dirs = append(dirs, managed)
	}
	if dir := os.Getenv(envConfigDir); dir != "" {
		dirs = append(dirs, dir)
	}
	return dirs
}

// projectDirs lists the directory and every ancestor up to the filesystem
// root: OpenCode reads project config at each level, not only to a git root.
func projectDirs(directory string) []string {
	dir := filepath.Clean(directory)
	var chain []string
	for {
		chain = append(chain, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			return chain
		}
		dir = parent
	}
}

func configDirRestrictive(dir string) bool {
	for _, name := range configNames {
		if configFileRestrictive(filepath.Join(dir, name)) {
			return true
		}
	}
	return agentDirsRestrictive(dir)
}

// configFileRestrictive: a missing file is skipped; an unreadable, oversized,
// non-regular or unparsable one counts as restrictive.
func configFileRestrictive(path string) bool {
	raw, err := readBounded(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err != nil {
		return true
	}
	return configTextRestrictive(raw, false)
}

// inlineRestrictive checks an environment variable holding JSON. An unset or
// empty variable is skipped. permissionOnly marks a value that is the
// permission object itself.
func inlineRestrictive(name string, permissionOnly bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return false
	}
	return configTextRestrictive([]byte(value), permissionOnly)
}

func configTextRestrictive(raw []byte, permissionOnly bool) bool {
	var cfg any
	if err := json.Unmarshal(stripJSONC(raw), &cfg); err != nil {
		return true
	}
	if permissionOnly {
		return !allowOnly(cfg)
	}
	return hasRestrictivePermission(cfg)
}
