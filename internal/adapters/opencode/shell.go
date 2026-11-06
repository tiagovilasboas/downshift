// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode

import (
	"bytes"
	"path/filepath"
	"strings"
)

// ShellOutput is the answer to a shell call: the command to run in place of
// the one the call carried. The plugin keeps every other field of the call.
type ShellOutput struct {
	V       int    `json:"v"`
	Action  string `json:"action"`
	Command string `json:"command"`
}

// ShellCommand returns the command of a shell call (kind "shell"). A malformed
// shell call still reports that it is a shell call, with no command, so the
// shared runner allows it unchanged instead of routing it.
func (r Request) ShellCommand() (string, bool) {
	if r.Kind != kindShell {
		return "", false
	}
	if r.V != 1 || r.Harness.ID != harnessID || r.Event != eventShell ||
		!childIDPattern.MatchString(r.Correlation.Child) ||
		r.Command == "" || len(r.Command) > maxShellText ||
		!validDirectory(r.Directory) || !validDirectory(r.SessionDirectory) {
		return "", true
	}
	return r.Command, true
}

// validDirectory bounds a directory field: within the cap and free of NUL. An
// empty one is valid on the wire; ShellBlocked treats it as unknown.
func validDirectory(dir string) bool {
	return len(dir) <= maxDirectoryLen && !strings.ContainsRune(dir, 0)
}

// directoriesKnown reports whether every directory is absolute. A relative
// one would be resolved against the hook's own working folder, which says
// nothing about the session, so it counts as doubt like an empty one.
func directoriesKnown(dirs ...string) bool {
	for _, d := range dirs {
		if !filepath.IsAbs(d) {
			return false
		}
	}
	return true
}

// ShellRewrite returns the answer that replaces the command.
func (r Request) ShellRewrite(command string) (any, error) {
	return ShellOutput{V: 1, Action: "rewrite", Command: command}, nil
}

// ShellBlocked reports whether OpenCode's own permission rules could cover
// the command, read from the directory the shell runs in and from the
// session's. Such a command is never rewritten: the rewrite would run a
// command the rule was not checked against. A directory that is empty or not
// absolute counts as doubt.
func (r Request) ShellBlocked(string) bool {
	if !directoriesKnown(r.Directory, r.SessionDirectory) {
		return true
	}
	return permissionRulesPresent(r.Directory, r.SessionDirectory)
}

// hasRestrictivePermission walks the whole config: a permission key at any
// depth (a global one, or one under an agent) that is not allow-only counts.
func hasRestrictivePermission(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		for key, val := range t {
			if (key == "permission" || key == "permissions") && !allowOnly(val) {
				return true
			}
			if hasRestrictivePermission(val) {
				return true
			}
		}
	case []any:
		for _, val := range t {
			if hasRestrictivePermission(val) {
				return true
			}
		}
	}
	return false
}

// allowOnly reports whether a permission value grants everything: an empty
// value, a list of allow rules, or an object whose values are all "allow".
// Anything else is restrictive.
func allowOnly(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case []any:
		for _, rule := range t {
			m, ok := rule.(map[string]any)
			if !ok {
				return false
			}
			if effect, _ := m["effect"].(string); effect != "allow" {
				return false
			}
		}
		return true
	case map[string]any:
		for _, val := range t {
			if s, ok := val.(string); !ok || s != "allow" {
				return false
			}
		}
		return true
	}
	return false
}

// stripJSONC removes comments and trailing commas so OpenCode's JSONC configs
// parse as JSON. It is string-aware; whatever it cannot make valid fails the
// JSON decode, and the caller treats that as restrictive.
func stripJSONC(in []byte) []byte {
	var noComments bytes.Buffer
	inStr := false
	for i := 0; i < len(in); i++ {
		c := in[i]
		if inStr {
			noComments.WriteByte(c)
			if c == '\\' && i+1 < len(in) {
				i++
				noComments.WriteByte(in[i])
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch {
		case c == '"':
			inStr = true
			noComments.WriteByte(c)
		case c == '/' && i+1 < len(in) && in[i+1] == '/':
			for i < len(in) && in[i] != '\n' {
				i++
			}
			if i < len(in) {
				noComments.WriteByte('\n')
			}
		case c == '/' && i+1 < len(in) && in[i+1] == '*':
			i += 2
			for i+1 < len(in) && !(in[i] == '*' && in[i+1] == '/') {
				i++
			}
			i++
			noComments.WriteByte(' ')
		default:
			noComments.WriteByte(c)
		}
	}
	src := noComments.Bytes()
	var out bytes.Buffer
	inStr = false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inStr {
			out.WriteByte(c)
			if c == '\\' && i+1 < len(src) {
				i++
				out.WriteByte(src[i])
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
		}
		if c == ',' {
			j := i + 1
			for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == '\r') {
				j++
			}
			if j < len(src) && (src[j] == '}' || src[j] == ']') {
				continue
			}
		}
		out.WriteByte(c)
	}
	return out.Bytes()
}
