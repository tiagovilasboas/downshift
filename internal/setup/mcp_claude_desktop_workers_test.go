// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"testing"
)

// Claude Desktop Chat starts the MCP server from "/", so read workers there
// would see the whole disk. Setup writes DOWNSHIFT_WORKERS=off into its entry.
func TestSetupClaudeDesktopTurnsWorkersOff(t *testing.T) {
	e := newSetupEnv(t, claudeDesktopDirRel())
	e.write(claudeDesktopConfigRel, `{"mcpServers": {}}`+"\n")
	if code, out, errOut := e.desktopRun("claude-desktop", "--yes"); code != 0 {
		t.Fatalf("code %d\nout:%s\nerr:%s", code, out, errOut)
	}
	if got := desktopEntryEnv(e)["DOWNSHIFT_WORKERS"]; got != "off" {
		t.Errorf("DOWNSHIFT_WORKERS = %v, want off", got)
	}
}

// An entry an older setup wrote (same binary, no DOWNSHIFT_WORKERS) is not up
// to date: the rerun adds the variable.
func TestSetupClaudeDesktopUpgradesEntryWithoutWorkersOff(t *testing.T) {
	e := newSetupEnv(t, claudeDesktopDirRel())
	e.write(claudeDesktopConfigRel, `{"mcpServers": {"downshift": {"command": "`+testBin+
		`", "args": ["mcp", "serve"], "env": {"`+McpMarkerKey+`": "`+McpMarkerValue+`"}}}}`+"\n")
	code, out, errOut := e.desktopRun("claude-desktop", "--yes")
	if code != 0 {
		t.Fatalf("code %d\nout:%s\nerr:%s", code, out, errOut)
	}
	if got := desktopEntryEnv(e)["DOWNSHIFT_WORKERS"]; got != "off" {
		t.Errorf("DOWNSHIFT_WORKERS = %v after rerun, want off\nout:%s", got, out)
	}
}

func desktopEntryEnv(e *setupEnv) map[string]any {
	e.t.Helper()
	ds, _ := e.desktopServers()["downshift"].(map[string]any)
	env, _ := ds["env"].(map[string]any)
	return env
}
