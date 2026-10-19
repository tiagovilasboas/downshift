// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package simbench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// Fixture agent types for Claude Code scenarios. Each has a known truth the
// simulator prices the baseline with.
const (
	// AgentInherit has a definition without a model line: it inherits the parent.
	AgentInherit = "sim-inherit"
	// AgentPinned has a definition naming its own model (Env.PinnedModel).
	AgentPinned = "sim-pinned"
	// AgentUndefined has no definition: Downshift cannot know its baseline.
	AgentUndefined = "sim-undefined"
	// PluginName is the plugin the repository's own agent definitions are
	// installed under, so "downshift:mechanical-worker" resolves as in a real
	// plugin install.
	PluginName = "downshift"
)

// DefaultControlShare matches the production default (about 1 in 10).
const DefaultControlShare = 0.10

// Env is one sandbox: every path the hook path reads or writes lives under
// Root, so a run never touches the operator's real state.
type Env struct {
	Root     string
	Home     string
	StateDir string
	EventLog string
	// Fixtures holds what the simulated harness wrote (transcripts, rollouts);
	// it is the input side and is not scanned by the prompt-free check.
	Fixtures      string
	Project       string
	SessionModels string
	// PinnedModel is the model AgentPinned's definition names.
	PinnedModel  string
	ControlShare float64
}

// NewEnv builds a sandbox under root: an empty project, a session allowlist
// with every tier for every harness, the fixture agent definitions and, when
// pluginAgents is a directory, a plugin install holding its *.md definitions.
func NewEnv(root string, cat core.Resolver, pluginAgents string) (Env, error) {
	e := Env{
		Root:         root,
		Home:         filepath.Join(root, "home"),
		StateDir:     filepath.Join(root, "state"),
		Fixtures:     filepath.Join(root, "fixtures"),
		Project:      filepath.Join(root, "project"),
		ControlShare: DefaultControlShare,
	}
	e.EventLog = filepath.Join(e.StateDir, "events.jsonl")
	e.SessionModels = filepath.Join(root, "session-models.json")
	e.PinnedModel = cat.ModelFor("claude-code", core.TierMid).ID
	claude := filepath.Join(e.Home, ".claude")
	for _, dir := range []string{e.StateDir, e.Fixtures, e.Project, filepath.Join(claude, "agents")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return Env{}, err
		}
	}
	allow := map[string][]string{}
	for _, h := range Harnesses() {
		for _, t := range []core.Tier{core.TierSmall, core.TierMid, core.TierFrontier} {
			allow[h] = append(allow[h], cat.ModelFor(h, t).ID)
		}
	}
	if err := writeJSON(e.SessionModels, allow); err != nil {
		return Env{}, err
	}
	defs := map[string]string{
		AgentInherit: "---\nname: " + AgentInherit + "\ndescription: simulated custom agent\n---\n",
		AgentPinned:  "---\nname: " + AgentPinned + "\nmodel: " + e.PinnedModel + "\n---\n",
	}
	for name, body := range defs {
		if err := os.WriteFile(filepath.Join(claude, "agents", name+".md"), []byte(body), 0o600); err != nil {
			return Env{}, err
		}
	}
	if pluginAgents != "" {
		if err := installPlugin(claude, filepath.Join(root, "plugins", PluginName), pluginAgents); err != nil {
			return Env{}, err
		}
	}
	return e, nil
}

// installPlugin copies the definitions into a plugin install path and
// registers it the way Claude Code's installed_plugins.json does.
func installPlugin(claude, install, agents string) error {
	files, err := filepath.Glob(filepath.Join(agents, "*.md"))
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no agent definitions in %s", agents)
	}
	if err := os.MkdirAll(filepath.Join(install, "agents"), 0o700); err != nil {
		return err
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(install, "agents", filepath.Base(f)), data, 0o600); err != nil {
			return err
		}
	}
	doc := map[string]any{"plugins": map[string]any{PluginName + "@sim": []any{map[string]any{"installPath": install}}}}
	if err := os.MkdirAll(filepath.Join(claude, "plugins"), 0o700); err != nil {
		return err
	}
	return writeJSON(filepath.Join(claude, "plugins", "installed_plugins.json"), doc)
}

// ResetPrefixes names the environment families a caller must blank before
// applying Vars: an operator's own DOWNSHIFT_* or Claude Code subagent
// override would otherwise change what the simulation routes.
var ResetPrefixes = []string{"DOWNSHIFT_", "CLAUDE_CODE_", "ANTHROPIC_MODEL", "CODEX_", "CURSOR_", "OPENCODE_"}

// Vars is the environment a run needs. Everything points inside Root; the
// capability-score refresh (the hook path's only network call) is off, and log
// rotation cannot rename the log mid-run. The caller must also make Project
// the working directory: a payload without a cwd (OpenCode) falls back to it,
// and the policy-file and agent-definition lookups walk up from there.
func (e Env) Vars() map[string]string {
	return map[string]string{
		"HOME":                     e.Home,
		"CLAUDE_CONFIG_DIR":        filepath.Join(e.Home, ".claude"),
		"CODEX_HOME":               filepath.Join(e.Home, ".codex"),
		"XDG_CONFIG_HOME":          filepath.Join(e.Home, ".config"),
		"XDG_DATA_HOME":            filepath.Join(e.Home, ".local", "share"),
		"XDG_STATE_HOME":           filepath.Join(e.Home, ".local", "state"),
		"DOWNSHIFT_STATE_DIR":      e.StateDir,
		"DOWNSHIFT_EVENT_LOG":      e.EventLog,
		"DOWNSHIFT_SESSION_MODELS": e.SessionModels,
		"DOWNSHIFT_SCORES":         "off",
		"DOWNSHIFT_DISCOVERY":      "off",
		"DOWNSHIFT_LOG_MAX_MB":     "0",
		"DOWNSHIFT_QUIET":          "1",
		"DOWNSHIFT_CONTROL_SHARE":  strconv.FormatFloat(e.ControlShare, 'f', -1, 64),
	}
}

// Blank reports whether an inherited variable must be blanked before a run.
func Blank(name string) bool {
	for _, p := range ResetPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func writeJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
