// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"sort"
	"strings"
)

// McpJSONPath is the object path that holds the servers of a harness file.
func McpJSONPath(layout string) []string {
	if layout == McpLayoutOpenCode {
		return []string{"mcp", "servers"}
	}
	return []string{"mcpServers"}
}

// mcpJSONEntry is the entry setup writes for bin under m's layout, with m's
// extra environment beside the marker.
func mcpJSONEntry(m Mcp, bin string) any {
	layout := m.Layout
	env := map[string]string{McpMarkerKey: McpMarkerValue}
	for k, v := range m.Env {
		env[k] = v
	}
	if layout == McpLayoutOpenCode {
		return struct {
			Type        string            `json:"type"`
			Command     []string          `json:"command"`
			Environment map[string]string `json:"environment"`
		}{"local", append([]string{bin}, McpServeArgs...), env}
	}
	return struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	}{bin, McpServeArgs, env}
}

// mcpEntryCommand reads the command and its arguments from either entry shape:
// a command string with an args list, or one command list whose first item is
// the program.
func mcpEntryCommand(entry map[string]any) (string, []string) {
	if list, ok := entry["command"].([]any); ok {
		parts := stringItems(list)
		if len(parts) == 0 {
			return "", nil
		}
		return parts[0], parts[1:]
	}
	cmd, _ := entry["command"].(string)
	list, _ := entry["args"].([]any)
	return cmd, stringItems(list)
}

// mcpEntryEnv reads the environment of either entry shape ("env" or
// "environment").
func mcpEntryEnv(entry map[string]any) map[string]any {
	if env, ok := entry["env"].(map[string]any); ok {
		return env
	}
	env, _ := entry["environment"].(map[string]any)
	return env
}

func stringItems(list []any) []string {
	var out []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// mcpEntrySummary describes a server entry as command, args and env key names
// only, so the dry-run shows what setup owns and never prints env values.
// label names the entry, for example "mcp.servers.downshift".
func mcpEntrySummary(label string, entry map[string]any) string {
	var b strings.Builder
	b.WriteString(label + "\n")
	cmd, args := mcpEntryCommand(entry)
	b.WriteString("  command: " + cmd + "\n")
	b.WriteString("  args: " + strings.Join(args, " ") + "\n")
	var keys []string
	for k := range mcpEntryEnv(entry) {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteString("  env keys: " + strings.Join(keys, ", ") + "\n")
	return b.String()
}
