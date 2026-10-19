// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Agent definitions. A subagent type that is not one of the built-ins that
// inherit (inheritingAgentTypes) is defined by a Markdown file with a
// frontmatter. Its `model:` line decides which model the child runs on when the
// Task call names none: absent or `inherit` means the parent's model, anything
// else is the author's explicit choice. The hook reads only that one line (and
// `name:` to match a file to its type); the body of the file is never read or
// stored.

// builtinPinnedAgentTypes are Claude Code's built-in subagent types that run on
// their own model, not the parent's (sub-agents docs, "Built-in subagents").
// The value is a family alias resolved through the catalog, so no model id
// lives here.
var builtinPinnedAgentTypes = map[string]string{
	"claude-code-guide": "haiku",
	"statusline-setup":  "sonnet",
}

const (
	// maxAgentDefinitionBytes bounds one definition read: only the frontmatter
	// at the top of the file matters.
	maxAgentDefinitionBytes = 64 << 10
	// maxAgentDirEntries bounds the scan of one agents directory for a file
	// whose `name:` matches when no file carries the type's name.
	maxAgentDirEntries = 200
)

// agentTypeRE accepts a plain agent name or a plugin-scoped one
// (`plugin:agent`). It has no path separator and no leading dot, so a type can
// never name a file outside an agents directory.
var agentTypeRE = regexp.MustCompile(`^(?:[A-Za-z0-9][A-Za-z0-9._-]{0,63}:)?[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// agentDefinition is what a definition says about the child's model.
type agentDefinition struct {
	// Found is true when a definition file (or a built-in entry) was located.
	Found bool
	// Model is the frontmatter `model:` value; empty or "inherit" means none.
	Model string
}

// Inherits reports a found definition that leaves the model to the parent.
func (d agentDefinition) Inherits() bool {
	return d.Found && (d.Model == "" || strings.EqualFold(d.Model, "inherit"))
}

// Pins reports a found definition that names its own model.
func (d agentDefinition) Pins() bool { return d.Found && !d.Inherits() }

// lookupAgentDefinition finds the definition of a subagent type: the built-in
// table, then the project agents directories (cwd and each parent), then the
// user's, then the installed plugins for a scoped type. A type with no file
// found is not Found: its baseline stays unknown, as before.
func lookupAgentDefinition(cwd, subagentType string) agentDefinition {
	if alias, ok := builtinPinnedAgentTypes[subagentType]; ok {
		return agentDefinition{Found: true, Model: alias}
	}
	if !agentTypeRE.MatchString(subagentType) {
		return agentDefinition{}
	}
	plugin, name, scoped := strings.Cut(subagentType, ":")
	if scoped {
		for _, dir := range pluginAgentDirs(plugin) {
			if def := definitionInDir(dir, name); def.Found {
				return def
			}
		}
		return agentDefinition{}
	}
	for _, dir := range agentDirs(cwd) {
		if def := definitionInDir(dir, subagentType); def.Found {
			return def
		}
	}
	return agentDefinition{}
}

// agentDirs lists the agents directories to search, nearest first: the project
// ones from cwd up to the filesystem root, then the user's.
func agentDirs(cwd string) []string {
	var dirs []string
	for dir := cwd; dir != "" && filepath.IsAbs(dir); {
		dirs = append(dirs, filepath.Join(dir, ".claude", "agents"))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if user := claudeUserDir(); user != "" {
		dirs = append(dirs, filepath.Join(user, "agents"))
	}
	return dirs
}

// claudeUserDir is the user-level Claude Code directory (CLAUDE_CONFIG_DIR
// honoured, as for the settings files).
func claudeUserDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".claude")
	}
	return ""
}

// pluginAgentDirs lists the agents directories of the installed plugins whose
// name is plugin, from the user's installed_plugins.json (`name@marketplace`
// keys with an installPath). An unreadable or odd file yields nothing.
func pluginAgentDirs(plugin string) []string {
	user := claudeUserDir()
	if user == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(user, "plugins", "installed_plugins.json"))
	if err != nil {
		return nil
	}
	var doc struct {
		Plugins map[string][]struct {
			InstallPath string `json:"installPath"`
		} `json:"plugins"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return nil
	}
	var dirs []string
	for key, installs := range doc.Plugins {
		if name, _, _ := strings.Cut(key, "@"); name != plugin {
			continue
		}
		for _, in := range installs {
			if filepath.IsAbs(in.InstallPath) {
				dirs = append(dirs, filepath.Join(in.InstallPath, "agents"))
			}
		}
	}
	return dirs
}

// definitionInDir reads the definition of agent name from one agents
// directory: the file named after it, otherwise a file whose frontmatter
// `name:` is it.
func definitionInDir(dir, name string) agentDefinition {
	if fm, ok := readFrontmatter(filepath.Join(dir, name+".md")); ok {
		return agentDefinition{Found: true, Model: fm["model"]}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return agentDefinition{}
	}
	for i, e := range entries {
		if i >= maxAgentDirEntries {
			break
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if fm, ok := readFrontmatter(filepath.Join(dir, e.Name())); ok && fm["name"] == name {
			return agentDefinition{Found: true, Model: fm["model"]}
		}
	}
	return agentDefinition{}
}

// readFrontmatter returns the top-level `name` and `model` values of a file's
// YAML frontmatter. ok is false when the file is missing, unreadable or has no
// frontmatter. Only those two keys are kept.
func readFrontmatter(path string) (map[string]string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), maxAgentDefinitionBytes)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return nil, false
	}
	out := map[string]string{}
	read := 0
	for sc.Scan() {
		line := sc.Text()
		if read += len(line) + 1; read > maxAgentDefinitionBytes {
			return nil, false
		}
		if strings.TrimSpace(line) == "---" {
			return out, true
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || key != "name" && key != "model" {
			continue
		}
		out[key] = unquote(strings.TrimSpace(value))
	}
	return nil, false // no closing delimiter
}

// unquote drops one pair of surrounding quotes and a trailing comment.
func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') {
		if end := strings.IndexByte(v[1:], v[0]); end >= 0 {
			return v[1 : 1+end]
		}
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}
