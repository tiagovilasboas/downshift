// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/setup/configfile"
)

// PlanMcpJSON plans the registration of downshift mcp serve inside a JSON file
// holding an "mcpServers" map (Cursor, Antigravity, Kiro). Only the downshift
// entry changes: every other member keeps its text and order.
func PlanMcpJSON(o Options, t Target) (Plan, bool) {
	relFile := filepath.FromSlash(t.Mcp.File)
	targetPath := filepath.Join(o.Home, relFile)
	shown := Display(o.Home, targetPath)

	if err := ErrLinkedPath(targetPath, o.LinkBases()...); err != nil {
		if o.Err != nil {
			fmt.Fprintf(o.Err, "warning: %s: %s: %v\n  the MCP server was not registered; nothing was written\n\n", t.Name, shown, err)
		}
		return Plan{}, false
	}

	raw, err := os.ReadFile(targetPath)
	existed := !errors.Is(err, os.ErrNotExist)
	if existed && err != nil {
		if o.Err != nil {
			fmt.Fprintf(o.Err, "warning: %s: %s: %v\n  the MCP server was not registered; the file was left as it is\n\n", t.Name, shown, err)
		}
		return Plan{}, false
	}

	doc := []configfile.Member{}
	if existed {
		var perr error
		if doc, perr = configfile.ParseMembers(raw); perr != nil {
			if o.Err != nil {
				fmt.Fprint(o.Err, mcpInvalidJSONWarning(t.Name, shown, *t.Mcp, o.Binary))
			}
			return Plan{}, false
		}
	}

	path := McpJSONPath(t.Mcp.Layout)
	label := strings.Join(path, ".") + "." + McpServerName
	servers, perr := configfile.MembersAt(doc, path)
	if perr != nil {
		if o.Err != nil {
			fmt.Fprintf(o.Err, "warning: %s: %s: `%s` is not an object, left untouched\n\n", t.Name, shown, strings.Join(path, "."))
		}
		return Plan{}, false
	}

	desired, err := configfile.MarshalValue(mcpJSONEntry(*t.Mcp, o.Binary))
	if err != nil {
		return Plan{}, false
	}

	var oldSummary string
	if i := configfile.MemberIndex(servers, McpServerName); i >= 0 {
		var em map[string]any
		if json.Unmarshal(servers[i].Val, &em) == nil {
			if mcpJSONEntryManaged(em) {
				if mcpJSONEntryCurrent(em, o.Binary, t.Mcp.Env) {
					if o.Out != nil {
						fmt.Fprintf(o.Out, "%s: MCP server %q already up to date (%s)\n\n", t.Name, McpServerName, shown)
					}
					return Plan{}, false
				}
				oldSummary = mcpEntrySummary(label, em)
			} else {
				if o.Err != nil {
					fmt.Fprintf(o.Err, "warning: %s: %s already has an MCP server named %q that setup did not add; left untouched\n\n", t.Name, shown, McpServerName)
				}
				return Plan{}, false
			}
		}
	}

	if holdForOwnerApp(o, t, shown) {
		return Plan{}, false
	}

	servers = configfile.SetMember(servers, McpServerName, desired)
	after, err := configfile.RenderWithMembersAt(doc, path, servers)
	if err != nil {
		if o.Err != nil {
			fmt.Fprintf(o.Err, "warning: %s: %s: could not build the new file: %v\n\n", t.Name, shown, err)
		}
		return Plan{}, false
	}

	if o.Out != nil {
		var nm map[string]any
		_ = json.Unmarshal(desired, &nm)
		verb := "will change"
		if o.DryRun {
			verb = "would change"
		}
		fmt.Fprintf(o.Out, "%s: %s %s (only the %q entry; other servers keep their text and order)\n", t.Name, shown, verb, McpServerName)
		fmt.Fprint(o.Out, configfile.UnifiedDiff(DiffName(shown, oldSummary != "", "a"), DiffName(shown, true, "b"), oldSummary, mcpEntrySummary(label, nm)))
		fmt.Fprintln(o.Out)
	}

	// Mode 0 keeps the file's own permission (0600 when new) and leaves the
	// containing directory alone: it belongs to the harness, not to setup.
	return Plan{
		Target:  t,
		Path:    targetPath,
		Before:  raw,
		Existed: existed,
		After:   after,
	}, true
}

// mcpJSONEntryManaged is true when the entry carries the marker setup adds.
func mcpJSONEntryManaged(em map[string]any) bool {
	return mcpEntryEnv(em)[McpMarkerKey] == McpMarkerValue
}

// mcpJSONEntryCurrent is true when the entry already runs bin with the serve
// args and carries every extra variable setup sets for the harness.
func mcpJSONEntryCurrent(em map[string]any, bin string, extra map[string]string) bool {
	cmd, args := mcpEntryCommand(em)
	if cmd != bin || !McpSameArgs(args, McpServeArgs) {
		return false
	}
	env := mcpEntryEnv(em)
	for k, v := range extra {
		if env[k] != v {
			return false
		}
	}
	return true
}

func PlanMcpJSONUndo(o Options, t Target) (Plan, bool, bool) {
	relFile := filepath.FromSlash(t.Mcp.File)
	targetPath := filepath.Join(o.Home, relFile)
	shown := Display(o.Home, targetPath)

	if err := ErrLinkedPath(targetPath, o.LinkBases()...); err != nil {
		if o.Err != nil {
			fmt.Fprintf(o.Err, "warning: %s: %s: %v\n  the MCP entry was not removed\n\n", t.Name, shown, err)
		}
		return Plan{}, false, true
	}

	raw, err := os.ReadFile(targetPath)
	if errors.Is(err, os.ErrNotExist) {
		return Plan{}, false, false
	}
	if err != nil {
		if o.Err != nil {
			fmt.Fprintf(o.Err, "warning: %s: %s: %v\n\n", t.Name, shown, err)
		}
		return Plan{}, false, true
	}

	doc, err := configfile.ParseMembers(raw)
	if err != nil {
		return Plan{}, false, false
	}
	path := McpJSONPath(t.Mcp.Layout)
	servers, err := configfile.MembersAt(doc, path)
	if err != nil {
		return Plan{}, false, false
	}
	ei := configfile.MemberIndex(servers, McpServerName)
	if ei < 0 {
		return Plan{}, false, false
	}
	var em map[string]any
	if json.Unmarshal(servers[ei].Val, &em) != nil || !mcpJSONEntryManaged(em) {
		return Plan{}, false, false // not added by setup
	}

	after, err := configfile.RenderWithMembersAt(doc, path, configfile.DeleteMember(servers, McpServerName))
	if err != nil {
		return Plan{}, false, true
	}

	return Plan{
		Target:  t,
		Path:    targetPath,
		Before:  raw,
		Existed: true,
		After:   after,
	}, true, false
}
