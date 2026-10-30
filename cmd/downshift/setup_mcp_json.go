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

// planMcpJSON plans the registration of downshift mcp serve inside a JSON file
// holding an "mcpServers" map (Cursor, Antigravity, Kiro).
func planMcpJSON(o setupOptions, t setupTarget) (setupPlan, bool) {
	relFile := filepath.FromSlash(t.Mcp.File)
	targetPath := filepath.Join(o.Home, relFile)
	shown := setupDisplay(o.Home, targetPath)

	realPath, err := filepath.EvalSymlinks(targetPath)
	if err == nil {
		targetPath = realPath
	}

	raw, err := os.ReadFile(targetPath)
	existed := !errors.Is(err, os.ErrNotExist)
	if existed && err != nil {
		if o.Err != nil {
			fmt.Fprintf(o.Err, "warning: %s: %s: %v\n  the MCP server was not registered; the file was left as it is\n\n", t.Name, shown, err)
		}
		return setupPlan{}, false
	}

	doc := map[string]any{}
	if existed {
		if err := json.Unmarshal(raw, &doc); err != nil {
			if o.Err != nil {
				fmt.Fprintf(o.Err, "warning: %s: %s: not valid JSON, left untouched\n\n", t.Name, shown)
			}
			return setupPlan{}, false
		}
	}

	serversAny, ok := doc["mcpServers"]
	var servers map[string]any
	if !ok || serversAny == nil {
		servers = map[string]any{}
		doc["mcpServers"] = servers
	} else if sm, ok := serversAny.(map[string]any); ok {
		servers = sm
	} else {
		if o.Err != nil {
			fmt.Fprintf(o.Err, "warning: %s: %s: `mcpServers` is not an object, left untouched\n\n", t.Name, shown)
		}
		return setupPlan{}, false
	}

	desiredEntry := map[string]any{
		"command": o.Binary,
		"args":    mcpServeArgs,
		"env": map[string]any{
			mcpMarkerKey: mcpMarkerValue,
		},
	}

	if existingAny, exists := servers[mcpServerName]; exists {
		if em, ok := existingAny.(map[string]any); ok {
			cmdMatch := em["command"] == o.Binary
			envMatch := false
			if envMap, ok := em["env"].(map[string]any); ok {
				envMatch = envMap[mcpMarkerKey] == mcpMarkerValue
			}
			argsMatch := false
			if argsList, ok := em["args"].([]any); ok {
				strArgs := make([]string, len(argsList))
				for i, a := range argsList {
					if s, ok := a.(string); ok {
						strArgs[i] = s
					}
				}
				argsMatch = mcpSameArgs(strArgs, mcpServeArgs)
			}
			if cmdMatch && envMatch && argsMatch {
				if o.Out != nil {
					fmt.Fprintf(o.Out, "%s: MCP server %q already up to date (%s)\n\n", t.Name, mcpServerName, shown)
				}
				return setupPlan{}, false
			}
			if !envMatch {
				if o.Err != nil {
					fmt.Fprintf(o.Err, "warning: %s: %s already has an MCP server named %q that setup did not add; left untouched\n\n", t.Name, shown, mcpServerName)
				}
				return setupPlan{}, false
			}
		}
	}

	servers[mcpServerName] = desiredEntry

	after, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		if o.Err != nil {
			fmt.Fprintf(o.Err, "warning: %s: %s: marshal failed: %v\n\n", t.Name, shown, err)
		}
		return setupPlan{}, false
	}
	after = append(after, '\n')

	if o.Out != nil {
		fmt.Fprintf(o.Out, "%s: will register MCP server %q in %s\n\n", t.Name, mcpServerName, shown)
	}

	mode := os.FileMode(0600)
	if existed {
		if st, err := os.Stat(targetPath); err == nil {
			mode = st.Mode().Perm()
		}
	}

	return setupPlan{
		target:  t,
		path:    targetPath,
		before:  raw,
		existed: existed,
		after:   after,
		mode:    mode,
	}, true
}

func planMcpJSONUndo(o setupOptions, t setupTarget) (setupPlan, bool, bool) {
	relFile := filepath.FromSlash(t.Mcp.File)
	targetPath := filepath.Join(o.Home, relFile)
	shown := setupDisplay(o.Home, targetPath)

	realPath, err := filepath.EvalSymlinks(targetPath)
	if err == nil {
		targetPath = realPath
	}

	raw, err := os.ReadFile(targetPath)
	if errors.Is(err, os.ErrNotExist) {
		return setupPlan{}, false, false
	}
	if err != nil {
		if o.Err != nil {
			fmt.Fprintf(o.Err, "warning: %s: %s: %v\n\n", t.Name, shown, err)
		}
		return setupPlan{}, false, true
	}

	doc := map[string]any{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return setupPlan{}, false, false
	}

	serversAny, ok := doc["mcpServers"]
	if !ok || serversAny == nil {
		return setupPlan{}, false, false
	}
	servers, ok := serversAny.(map[string]any)
	if !ok {
		return setupPlan{}, false, false
	}

	existingAny, exists := servers[mcpServerName]
	if !exists {
		return setupPlan{}, false, false
	}

	em, ok := existingAny.(map[string]any)
	if !ok {
		return setupPlan{}, false, false
	}
	envMap, ok := em["env"].(map[string]any)
	if !ok || envMap[mcpMarkerKey] != mcpMarkerValue {
		// Not added by setup
		return setupPlan{}, false, false
	}

	delete(servers, mcpServerName)

	after, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return setupPlan{}, false, true
	}
	after = append(after, '\n')

	mode := os.FileMode(0600)
	if st, err := os.Stat(targetPath); err == nil {
		mode = st.Mode().Perm()
	}

	return setupPlan{
		target:  t,
		path:    targetPath,
		before:  raw,
		existed: true,
		after:   after,
		mode:    mode,
	}, true, false
}
