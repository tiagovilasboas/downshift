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
// holding an "mcpServers" map (Cursor, Antigravity, Kiro). Only the downshift
// entry changes: every other member keeps its text and order.
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

	doc := []jsonMember{}
	if existed {
		var perr error
		if doc, perr = parseJSONObject(raw); perr != nil {
			if o.Err != nil {
				fmt.Fprintf(o.Err, "warning: %s: %s: not a valid JSON object, left untouched\n\n", t.Name, shown)
			}
			return setupPlan{}, false
		}
	}

	servers := []jsonMember{}
	if i := jsonMemberIndex(doc, "mcpServers"); i >= 0 && string(doc[i].val) != "null" {
		var perr error
		if servers, perr = parseJSONObject(doc[i].val); perr != nil {
			if o.Err != nil {
				fmt.Fprintf(o.Err, "warning: %s: %s: `mcpServers` is not an object, left untouched\n\n", t.Name, shown)
			}
			return setupPlan{}, false
		}
	}

	desired, err := marshalJSONValue(struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	}{o.Binary, mcpServeArgs, map[string]string{mcpMarkerKey: mcpMarkerValue}})
	if err != nil {
		return setupPlan{}, false
	}

	var oldSummary string
	if i := jsonMemberIndex(servers, mcpServerName); i >= 0 {
		var em map[string]any
		if json.Unmarshal(servers[i].val, &em) == nil {
			if mcpJSONEntryManaged(em) {
				if mcpJSONEntryCurrent(em, o.Binary) {
					if o.Out != nil {
						fmt.Fprintf(o.Out, "%s: MCP server %q already up to date (%s)\n\n", t.Name, mcpServerName, shown)
					}
					return setupPlan{}, false
				}
				oldSummary = mcpEntrySummary(em)
			} else {
				if o.Err != nil {
					fmt.Fprintf(o.Err, "warning: %s: %s already has an MCP server named %q that setup did not add; left untouched\n\n", t.Name, shown, mcpServerName)
				}
				return setupPlan{}, false
			}
		}
	}

	servers = setJSONMember(servers, mcpServerName, desired)
	after, err := withMcpServers(doc, servers)
	if err != nil {
		if o.Err != nil {
			fmt.Fprintf(o.Err, "warning: %s: %s: could not build the new file: %v\n\n", t.Name, shown, err)
		}
		return setupPlan{}, false
	}

	if o.Out != nil {
		var nm map[string]any
		_ = json.Unmarshal(desired, &nm)
		verb := "will change"
		if o.DryRun {
			verb = "would change"
		}
		fmt.Fprintf(o.Out, "%s: %s %s (only the %q entry; other servers keep their text and order)\n", t.Name, shown, verb, mcpServerName)
		fmt.Fprint(o.Out, unifiedDiff(diffName(shown, oldSummary != "", "a"), diffName(shown, true, "b"), oldSummary, mcpEntrySummary(nm)))
		fmt.Fprintln(o.Out)
	}

	// Mode 0 keeps the file's own permission (0600 when new) and leaves the
	// containing directory alone: it belongs to the harness, not to setup.
	return setupPlan{
		target:  t,
		path:    targetPath,
		before:  raw,
		existed: existed,
		after:   after,
	}, true
}

// mcpJSONEntryManaged is true when the entry carries the marker setup adds.
func mcpJSONEntryManaged(em map[string]any) bool {
	env, ok := em["env"].(map[string]any)
	return ok && env[mcpMarkerKey] == mcpMarkerValue
}

// mcpJSONEntryCurrent is true when the entry already runs bin with the serve args.
func mcpJSONEntryCurrent(em map[string]any, bin string) bool {
	if em["command"] != bin {
		return false
	}
	list, ok := em["args"].([]any)
	if !ok {
		return false
	}
	args := make([]string, len(list))
	for i, a := range list {
		if s, ok := a.(string); ok {
			args[i] = s
		}
	}
	return mcpSameArgs(args, mcpServeArgs)
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

	doc, err := parseJSONObject(raw)
	if err != nil {
		return setupPlan{}, false, false
	}
	si := jsonMemberIndex(doc, "mcpServers")
	if si < 0 {
		return setupPlan{}, false, false
	}
	servers, err := parseJSONObject(doc[si].val)
	if err != nil {
		return setupPlan{}, false, false
	}
	ei := jsonMemberIndex(servers, mcpServerName)
	if ei < 0 {
		return setupPlan{}, false, false
	}
	var em map[string]any
	if json.Unmarshal(servers[ei].val, &em) != nil || !mcpJSONEntryManaged(em) {
		return setupPlan{}, false, false // not added by setup
	}

	after, err := withMcpServers(doc, deleteJSONMember(servers, mcpServerName))
	if err != nil {
		return setupPlan{}, false, true
	}

	return setupPlan{
		target:  t,
		path:    targetPath,
		before:  raw,
		existed: true,
		after:   after,
	}, true, false
}
