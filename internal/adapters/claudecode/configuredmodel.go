// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// configuredModelEnv is the variable that picks the session model at launch.
const configuredModelEnv = "ANTHROPIC_MODEL"

// familyAliases are the model aliases Claude Code accepts that name a family.
// Each is looked up in the catalog (native name), so no model id lives here.
var familyAliases = map[string]bool{"sonnet": true, "opus": true, "haiku": true, "fable": true}

// configuredModel returns the model the session is configured to use, as a
// catalog id, or "" when it cannot be told. It is the fallback for a baseline
// the transcript does not have yet (parallel Agent calls in a session's first
// assistant message). Claude Code's order (model-config docs): ANTHROPIC_MODEL,
// then the `model` setting from <cwd>/.claude/settings.local.json,
// <cwd>/.claude/settings.json and the user settings.json; the first that sets
// it wins, and "default" or an unrecognised value means unknown. The `--model`
// flag and a mid-session /model are not visible to a hook, so the result is an
// inference and the caller flags it.
func configuredModel(cwd, permissionMode string, res core.Resolver) string {
	if res == nil {
		return ""
	}
	raw := strings.TrimSpace(os.Getenv(configuredModelEnv))
	if raw == "" {
		raw = modelSetting(cwd)
	}
	return resolveConfiguredModel(raw, permissionMode, res)
}

// modelSetting reads `model` from the settings files, nearest first. A file
// that is missing, unreadable, invalid JSON or silent about `model` is skipped.
func modelSetting(cwd string) string {
	var files []string
	if cwd != "" {
		files = append(files, filepath.Join(cwd, ".claude", "settings.local.json"), filepath.Join(cwd, ".claude", "settings.json"))
	}
	userDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if userDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			userDir = filepath.Join(home, ".claude")
		}
	}
	if userDir != "" {
		files = append(files, filepath.Join(userDir, "settings.json"))
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var s struct {
			Model string `json:"model"`
		}
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		if m := strings.TrimSpace(s.Model); m != "" {
			return m
		}
	}
	return ""
}

// resolveConfiguredModel turns a configured value into a catalog id: a model
// the catalog knows passes through, "opusplan" is Opus in plan mode and Sonnet
// otherwise, the family aliases map to the catalog's model of that family, and
// anything else (including "default") is unknown.
func resolveConfiguredModel(raw, permissionMode string, res core.Resolver) string {
	alias := strings.ToLower(raw)
	if alias == "opusplan" {
		alias = "sonnet"
		if permissionMode == "plan" {
			alias = "opus"
		}
	}
	if raw == "" || alias == "default" {
		return ""
	}
	lookup := raw
	if familyAliases[alias] {
		lookup = alias
	}
	if m, ok := res.LookupByID(harnessID, lookup); ok {
		if familyAliases[alias] {
			return m.ID
		}
		return raw
	}
	return ""
}
