// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// managedSettingsPaths are the enterprise-managed settings files. Tests swap
// it.
var managedSettingsPaths = defaultManagedSettingsPaths

func defaultManagedSettingsPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"/Library/Application Support/ClaudeCode/managed-settings.json"}
	case "windows":
		return []string{`C:\Program Files\ClaudeCode\managed-settings.json`}
	}
	return []string{"/etc/claude-code/managed-settings.json"}
}

type permissionRules struct {
	Permissions struct {
		Allow []string `json:"allow"`
		Deny  []string `json:"deny"`
		Ask   []string `json:"ask"`
	} `json:"permissions"`
}

// settingsFiles lists every settings file that can carry permission rules:
// managed, user (CLAUDE_CONFIG_DIR honoured), and the project and local files
// of the working directory and each parent up to the repository root.
func settingsFiles(cwd string) ([]string, error) {
	files := append([]string{}, managedSettingsPaths()...)
	userDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if userDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		userDir = filepath.Join(home, ".claude")
	}
	files = append(files, filepath.Join(userDir, "settings.json"), filepath.Join(userDir, "settings.local.json"))
	if cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		cwd = wd
	}
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	for {
		files = append(files, filepath.Join(dir, ".claude", "settings.json"), filepath.Join(dir, ".claude", "settings.local.json"))
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return files, nil
}

// loadPermissionRules merges allow, deny and ask rules from every scope. A
// missing file is skipped; a file that exists but cannot be read or parsed is
// an error, so the caller does not guess.
func loadPermissionRules(cwd string) (allow, deny, ask []string, err error) {
	files, err := settingsFiles(cwd)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, f := range files {
		data, err := readSettings(f)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, nil, nil, err
		}
		var s permissionRules
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, nil, nil, err
		}
		allow = append(allow, s.Permissions.Allow...)
		deny = append(deny, s.Permissions.Deny...)
		ask = append(ask, s.Permissions.Ask...)
	}
	return allow, deny, ask, nil
}

// ShellBlocked reports whether a permission rule forbids a rewrite of command:
// some deny or ask rule matches it, across the managed, user, project and
// local scopes. Any settings read or parse error answers true (fail closed).
// Allow rules are not consulted: the rewrite emits no
// permissionDecision, so the harness's own permission flow still decides.
func (ev Event) ShellBlocked(command string) bool {
	_, deny, ask, err := loadPermissionRules(ev.Cwd)
	if err != nil {
		return true
	}
	command = strings.TrimSpace(command)
	for _, r := range append(append([]string{}, deny...), ask...) {
		if bashRuleMatches(r, command) {
			return true
		}
	}
	return false
}

// bashRuleMatches reports whether a permission rule of the Bash tool matches
// command, as the Claude Code permissions docs define it (see
// shellperm_conformance_test.go): a trailing " *" that is the rule's only
// wildcard, or ":*", also matches the bare command.
func bashRuleMatches(rule, command string) bool {
	rule = strings.TrimSpace(rule)
	if rule == shellToolName {
		return true
	}
	if !strings.HasPrefix(rule, shellToolName+"(") || !strings.HasSuffix(rule, ")") {
		return false
	}
	pattern := rule[len(shellToolName)+1 : len(rule)-1]
	switch {
	case pattern == "*":
		return true
	case strings.HasSuffix(pattern, ":*"):
		prefix := strings.TrimSuffix(pattern, ":*")
		return command == prefix || strings.HasPrefix(command, prefix+" ")
	case strings.Contains(pattern, "*"):
		if strings.Count(pattern, "*") == 1 && strings.HasSuffix(pattern, " *") && command == strings.TrimSuffix(pattern, " *") {
			return true
		}
		return globMatch(pattern, command)
	}
	return pattern == command
}

func globMatch(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	re, err := regexp.Compile("^" + strings.Join(parts, ".*") + "$")
	return err == nil && re.MatchString(s)
}

// WithManagedSettings replaces the managed settings paths and returns the
// restore func. Tests use it to stay independent of the machine's policy.
func WithManagedSettings(paths []string) func() {
	old := managedSettingsPaths
	managedSettingsPaths = func() []string { return paths }
	return func() { managedSettingsPaths = old }
}

// maxSettingsBytes caps one settings file read on every Bash call: a cloned
// repository can carry a huge .claude/settings.json, and the hook has a 1.5 s
// deadline. A larger file is an error, which ShellBlocked reads as blocked.
const maxSettingsBytes = 1 << 20

// readSettings reads one settings file up to maxSettingsBytes.
func readSettings(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxSettingsBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSettingsBytes {
		return nil, errSettingsTooLarge
	}
	return data, nil
}

var errSettingsTooLarge = errors.New("settings file larger than 1 MiB")
