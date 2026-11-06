// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift/internal/setup"
	"github.com/tiagovilasboas/downshift/internal/setup/configfile"
)

// The MCP registration check for every target setup writes an entry for, read
// from the same setup.Targets table setup uses (setup.Target.Mcp). Claude Code is
// registered through its CLI and has its own deeper check (doctor_mcp.go);
// every file-based target is checked here by reading its config only. No server
// is started: a config read is enough to say registered or missing.

type mcpTargetState int

const (
	mcpTargetNotInstalled mcpTargetState = iota
	mcpTargetRegistered
	mcpTargetMissing
)

// checkMcpTargets adds one row per file-registered target: registered, missing
// (fix: downshift setup <target>), or nothing when the harness is not installed.
func checkMcpTargets(add func(id, status, detail, fix string), home string, targets []setup.Target) {
	for _, t := range targets {
		if t.Mcp == nil || t.Mcp.Kind == setup.McpViaCLI {
			continue
		}
		state, detail := mcpTargetStatus(home, t)
		id := t.Name + " mcp registered"
		switch state {
		case mcpTargetRegistered:
			add(id, doctorOK, detail, "")
		case mcpTargetMissing:
			add(id, doctorWarn, detail, "downshift setup "+t.Name)
		}
	}
}

// mcpTargetStatus reads the target's MCP file and reports whether the downshift
// server is registered. A harness whose config directory is absent is not
// installed. A missing or malformed file, or a missing entry, is "missing".
func mcpTargetStatus(home string, t setup.Target) (mcpTargetState, string) {
	if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(t.ConfigDir))); err != nil {
		return mcpTargetNotInstalled, ""
	}
	if t.Mcp.File == "" {
		return mcpTargetMissing, "no MCP file declared for " + t.Name
	}
	file := filepath.Join(home, filepath.FromSlash(t.Mcp.File))
	raw, err := os.ReadFile(file)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return mcpTargetMissing, "no " + file
	case err != nil:
		return mcpTargetMissing, fmt.Sprintf("%s unreadable: %v", file, err)
	}
	found, err := mcpEntryInFile(raw, t.Mcp)
	switch {
	case err != nil:
		return mcpTargetMissing, fmt.Sprintf("%s unreadable: %v", file, err)
	case !found:
		return mcpTargetMissing, "no " + setup.McpServerName + " MCP entry in " + file
	}
	return mcpTargetRegistered, setup.McpServerName + " MCP entry present in " + file
}

// mcpEntryInFile reports whether raw holds the downshift entry, in the shape
// setup writes for m.Kind and m.Layout.
func mcpEntryInFile(raw []byte, m *setup.Mcp) (bool, error) {
	if m.Kind == setup.McpTOMLBlock {
		_, _, found, err := setup.McpBlockBounds(string(raw))
		return found, err
	}
	doc, err := configfile.ParseMembers(raw)
	if err != nil {
		return false, err
	}
	servers, err := configfile.MembersAt(doc, setup.McpJSONPath(m.Layout))
	if err != nil {
		return false, err
	}
	return configfile.MemberIndex(servers, setup.McpServerName) >= 0, nil
}
