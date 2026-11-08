// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// checkOwnerAppMcp reports, during the claude-code check, whether an MCP entry
// setup wrote into an app-owned config (setup.Mcp.OwnerApp, Claude Desktop) is
// still there. A setup backup next to the file is the proof setup wrote it; an
// entry missing after that means the app rewrote the file while it was open.
// Targets come from setup.Targets, so no harness is named here.
func checkOwnerAppMcp(add func(id, status, detail, fix string), harness string) {
	if harness != "claude-code" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	ownerAppMcpRows(add, home, setup.Targets, setup.OwnerAppRunningState)
}

// ownerAppMcpRows is checkOwnerAppMcp with its machine inputs passed in.
func ownerAppMcpRows(add func(id, status, detail, fix string), home string, targets []setup.Target, state func(string) setup.OwnerAppState) {
	for _, t := range targets {
		if t.Mcp == nil || t.Mcp.OwnerApp == "" || t.Mcp.Kind != setup.McpJSONMap {
			continue
		}
		path := filepath.Join(home, filepath.FromSlash(t.Mcp.File))
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		baks, _ := filepath.Glob(path + setup.BackupInfix + "*")
		if len(baks) == 0 {
			continue // setup never wrote this file: nothing to compare
		}
		id := t.Name + " mcp"
		if jsonMapHasServer(raw, setup.McpServerName) {
			add(id, doctorOK, fmt.Sprintf("mcpServers.%s present", setup.McpServerName), "")
			continue
		}
		detail := fmt.Sprintf("setup wrote mcpServers.%s (%d backup(s) beside the file) but it is gone: %s rewrote the file while open",
			setup.McpServerName, len(baks), t.Mcp.OwnerApp)
		if state(t.Mcp.OwnerApp) == setup.OwnerAppRunning {
			detail += fmt.Sprintf("; %s is running now", t.Mcp.OwnerApp)
		}
		add(id, doctorWarn, detail, setup.OwnerAppFix(t.Mcp.OwnerApp))
	}
}

// jsonMapHasServer reports whether raw holds mcpServers.<name>. Unparsable
// JSON counts as absent.
func jsonMapHasServer(raw []byte, name string) bool {
	var doc struct {
		McpServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return false
	}
	_, ok := doc.McpServers[name]
	return ok
}
