// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// TestMcpTargetsTableInvariants keeps the doctor rows unique: every
// file-registered target names a file and yields its own row id, distinct from
// the Claude Code CLI check's "mcp registered".
func TestMcpTargetsTableInvariants(t *testing.T) {
	seen := map[string]bool{"mcp registered": true}
	for _, tgt := range setup.Targets {
		if tgt.Mcp == nil || tgt.Mcp.Kind == setup.McpViaCLI {
			continue
		}
		if tgt.Name == "" || tgt.Mcp.File == "" {
			t.Fatalf("target %+v: file-registered MCP needs a name and a file", tgt)
		}
		id := tgt.Name + " mcp registered"
		if seen[id] {
			t.Fatalf("duplicate doctor row id %q", id)
		}
		seen[id] = true
	}
}

func TestMcpTargetStatusEmptyFileIsMissing(t *testing.T) {
	home := t.TempDir()
	tgt := setup.Target{Name: "x", ConfigDir: ".", Mcp: &setup.Mcp{}}
	if st, _ := mcpTargetStatus(home, tgt); st != mcpTargetMissing {
		t.Fatalf("state = %v, want missing for an empty MCP file path", st)
	}
}
