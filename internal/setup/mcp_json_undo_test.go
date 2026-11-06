// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPlanMcpJSON_NilStreamsNeverPanic(t *testing.T) {
	home := t.TempDir()
	opts := Options{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Mcp:    true,
		Out:    nil,
		Err:    nil,
	}
	target := Target{
		Name: "cursor",
		Mcp:  &Mcp{Kind: McpJSONMap, File: ".cursor/mcp.json"},
	}

	// 1. Missing file: writes "will register" to Out
	plan, ok := PlanMcpJSON(opts, target)
	if !ok {
		t.Fatal("PlanMcpJSON returned !ok")
	}
	if _, err := Write(plan, "niltest1"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// 2. Up to date: writes "already up to date" to Out
	_, ok = PlanMcpJSON(opts, target)
	if ok {
		t.Error("PlanMcpJSON returned ok for up-to-date file")
	}

	// 3. Invalid JSON: writes warning to Err
	targetPath := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.WriteFile(targetPath, []byte("invalid json {"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, ok = PlanMcpJSON(opts, target)
	if ok {
		t.Error("PlanMcpJSON returned ok for invalid JSON")
	}

	// 4. mcpServers not an object: writes warning to Err
	if err := os.WriteFile(targetPath, []byte(`{"mcpServers": "not-an-object"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, ok = PlanMcpJSON(opts, target)
	if ok {
		t.Error("PlanMcpJSON returned ok for non-object mcpServers")
	}

	// 5. Unmanaged server: writes warning to Err
	unmanaged := `{"mcpServers": {"downshift": {"command": "/usr/local/bin/downshift", "args": ["mcp", "serve"], "env": {}}}}`
	if err := os.WriteFile(targetPath, []byte(unmanaged), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, ok = PlanMcpJSON(opts, target)
	if ok {
		t.Error("PlanMcpJSON returned ok for unmanaged server")
	}

	// 6. Undo with nil streams
	undoOpts := Options{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Undo:   true,
		Yes:    true,
		Out:    nil,
		Err:    nil,
	}
	_, _, _ = PlanMcpJSONUndo(undoOpts, target)
	_ = mcpUndo(undoOpts, []Target{target})
}

func TestPlanMcpJSON_UndoRemovesOnlyDownshift(t *testing.T) {
	home := t.TempDir()
	mcpDir := filepath.Join(home, ".cursor")
	_ = os.MkdirAll(mcpDir, 0o755)
	existing := []byte(`{
  "mcpServers": {
    "other": {
      "command": "node"
    },
    "downshift": {
      "command": "/usr/local/bin/downshift",
      "args": ["mcp", "serve"],
      "env": {
        "DOWNSHIFT_MANAGED": "setup"
      }
    }
  }
}`)
	_ = os.WriteFile(filepath.Join(mcpDir, "mcp.json"), existing, 0o644)

	opts := Options{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Undo:   true,
		Yes:    true,
	}
	target := Target{
		Name: "cursor",
		Mcp:  &Mcp{Kind: McpJSONMap, File: ".cursor/mcp.json"},
	}

	plan, ok, bad := PlanMcpJSONUndo(opts, target)
	if bad || !ok {
		t.Fatalf("undo failed: ok=%v bad=%v", ok, bad)
	}

	var doc map[string]any
	_ = json.Unmarshal(plan.After, &doc)
	servers := doc["mcpServers"].(map[string]any)
	if _, ok := servers["downshift"]; ok {
		t.Error("downshift was not removed")
	}
	if _, ok := servers["other"]; !ok {
		t.Error("other server was accidentally removed")
	}

	// Test mcpUndo execution
	code := mcpUndo(opts, []Target{target})
	if code != 0 {
		t.Errorf("mcpUndo exit code = %d, want 0", code)
	}
	raw, err := os.ReadFile(filepath.Join(mcpDir, "mcp.json"))
	if err != nil {
		t.Fatalf("ReadFile after undo: %v", err)
	}
	var docAfter map[string]any
	_ = json.Unmarshal(raw, &docAfter)
	serversAfter := docAfter["mcpServers"].(map[string]any)
	if _, ok := serversAfter["downshift"]; ok {
		t.Error("downshift still present in file after mcpUndo")
	}
	if _, ok := serversAfter["other"]; !ok {
		t.Error("other server was removed from file after mcpUndo")
	}
}
