// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanMcpJSON_CreatesNewFileWhenMissing(t *testing.T) {
	home := t.TempDir()
	opts := setupOptions{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Mcp:    true,
	}
	target := setupTarget{
		Name: "cursor",
		Mcp:  &setupMcp{Kind: mcpJSONMap, File: ".cursor/mcp.json"},
	}

	plan, ok := planMcpJSON(opts, target)
	if !ok {
		t.Fatal("planMcpJSON returned !ok")
	}
	if plan.existed {
		t.Error("plan.existed was true for missing file")
	}

	var doc map[string]any
	if err := json.Unmarshal(plan.after, &doc); err != nil {
		t.Fatalf("unmarshal after: %v", err)
	}
	servers := doc["mcpServers"].(map[string]any)
	entry := servers["downshift"].(map[string]any)
	if entry["command"] != "/usr/local/bin/downshift" {
		t.Errorf("command = %v", entry["command"])
	}
}

func TestPlanMcpJSON_MergesExistingServers(t *testing.T) {
	home := t.TempDir()
	mcpDir := filepath.Join(home, ".cursor")
	_ = os.MkdirAll(mcpDir, 0o755)
	existing := []byte(`{
  "mcpServers": {
    "other": {
      "command": "node",
      "args": ["server.js"]
    }
  }
}`)
	_ = os.WriteFile(filepath.Join(mcpDir, "mcp.json"), existing, 0o644)

	opts := setupOptions{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Mcp:    true,
	}
	target := setupTarget{
		Name: "cursor",
		Mcp:  &setupMcp{Kind: mcpJSONMap, File: ".cursor/mcp.json"},
	}

	plan, ok := planMcpJSON(opts, target)
	if !ok {
		t.Fatal("planMcpJSON returned !ok")
	}

	var doc map[string]any
	_ = json.Unmarshal(plan.after, &doc)
	servers := doc["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Error("existing 'other' server was dropped")
	}
	if _, ok := servers["downshift"]; !ok {
		t.Error("missing 'downshift' server")
	}
}

func TestPlanMcpJSON_Idempotent(t *testing.T) {
	home := t.TempDir()
	mcpDir := filepath.Join(home, ".cursor")
	_ = os.MkdirAll(mcpDir, 0o755)
	existing := []byte(`{
  "mcpServers": {
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

	var stdout strings.Builder
	opts := setupOptions{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Mcp:    true,
		Out:    &stdout,
	}
	target := setupTarget{
		Name: "cursor",
		Mcp:  &setupMcp{Kind: mcpJSONMap, File: ".cursor/mcp.json"},
	}

	_, ok := planMcpJSON(opts, target)
	if ok {
		t.Error("planMcpJSON returned ok for already up to date")
	}
	if !strings.Contains(stdout.String(), "already up to date") {
		t.Errorf("stdout = %q; want 'already up to date'", stdout.String())
	}
}

func TestPlanMcpJSON_NilStreamsNeverPanic(t *testing.T) {
	home := t.TempDir()
	opts := setupOptions{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Mcp:    true,
		Out:    nil,
		Err:    nil,
	}
	target := setupTarget{
		Name: "cursor",
		Mcp:  &setupMcp{Kind: mcpJSONMap, File: ".cursor/mcp.json"},
	}

	// 1. Missing file: writes "will register" to Out
	plan, ok := planMcpJSON(opts, target)
	if !ok {
		t.Fatal("planMcpJSON returned !ok")
	}
	if _, err := setupWrite(plan, "niltest1"); err != nil {
		t.Fatalf("setupWrite: %v", err)
	}

	// 2. Up to date: writes "already up to date" to Out
	_, ok = planMcpJSON(opts, target)
	if ok {
		t.Error("planMcpJSON returned ok for up-to-date file")
	}

	// 3. Invalid JSON: writes warning to Err
	targetPath := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.WriteFile(targetPath, []byte("invalid json {"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, ok = planMcpJSON(opts, target)
	if ok {
		t.Error("planMcpJSON returned ok for invalid JSON")
	}

	// 4. mcpServers not an object: writes warning to Err
	if err := os.WriteFile(targetPath, []byte(`{"mcpServers": "not-an-object"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, ok = planMcpJSON(opts, target)
	if ok {
		t.Error("planMcpJSON returned ok for non-object mcpServers")
	}

	// 5. Unmanaged server: writes warning to Err
	unmanaged := `{"mcpServers": {"downshift": {"command": "/usr/local/bin/downshift", "args": ["mcp", "serve"], "env": {}}}}`
	if err := os.WriteFile(targetPath, []byte(unmanaged), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, ok = planMcpJSON(opts, target)
	if ok {
		t.Error("planMcpJSON returned ok for unmanaged server")
	}

	// 6. Undo with nil streams
	undoOpts := setupOptions{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Undo:   true,
		Yes:    true,
		Out:    nil,
		Err:    nil,
	}
	_, _, _ = planMcpJSONUndo(undoOpts, target)
	_ = mcpUndo(undoOpts, []setupTarget{target})
}

func TestPlanMcpJSON_HarnessTargetsCreationAndIdempotence(t *testing.T) {
	targets := []setupTarget{
		{
			Name: "cursor",
			Mcp:  &setupMcp{Kind: mcpJSONMap, File: ".cursor/mcp.json"},
		},
		{
			Name: "antigravity",
			Mcp:  &setupMcp{Kind: mcpJSONMap, File: ".gemini/config/mcp_config.json"},
		},
		{
			Name: "kirocrew",
			Mcp:  &setupMcp{Kind: mcpJSONMap, File: ".kiro/settings/mcp.json"},
		},
	}

	for _, tgt := range targets {
		t.Run(tgt.Name, func(t *testing.T) {
			home := t.TempDir()
			var stdout strings.Builder
			opts := setupOptions{
				Home:   home,
				Binary: "/usr/local/bin/downshift",
				Mcp:    true,
				Out:    &stdout,
			}

			plan, ok := planMcpJSON(opts, tgt)
			if !ok {
				t.Fatalf("%s: planMcpJSON returned !ok", tgt.Name)
			}
			if plan.existed {
				t.Errorf("%s: plan.existed was true for missing file", tgt.Name)
			}
			if _, err := setupWrite(plan, "stamp1"); err != nil {
				t.Fatalf("%s: setupWrite: %v", tgt.Name, err)
			}

			targetPath := filepath.Join(home, filepath.FromSlash(tgt.Mcp.File))
			raw, err := os.ReadFile(targetPath)
			if err != nil {
				t.Fatalf("%s: ReadFile: %v", tgt.Name, err)
			}
			var doc map[string]any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("%s: unmarshal: %v", tgt.Name, err)
			}
			servers, ok := doc["mcpServers"].(map[string]any)
			if !ok {
				t.Fatalf("%s: mcpServers missing or not a map", tgt.Name)
			}
			entry, ok := servers["downshift"].(map[string]any)
			if !ok {
				t.Fatalf("%s: downshift entry missing", tgt.Name)
			}
			if entry["command"] != "/usr/local/bin/downshift" {
				t.Errorf("%s: command = %v, want /usr/local/bin/downshift", tgt.Name, entry["command"])
			}

			stdout.Reset()
			_, ok2 := planMcpJSON(opts, tgt)
			if ok2 {
				t.Errorf("%s: second planMcpJSON returned ok=true, want false", tgt.Name)
			}
			if !strings.Contains(stdout.String(), "already up to date") {
				t.Errorf("%s: stdout %q does not contain 'already up to date'", tgt.Name, stdout.String())
			}
		})
	}
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

	opts := setupOptions{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Undo:   true,
		Yes:    true,
	}
	target := setupTarget{
		Name: "cursor",
		Mcp:  &setupMcp{Kind: mcpJSONMap, File: ".cursor/mcp.json"},
	}

	plan, ok, bad := planMcpJSONUndo(opts, target)
	if bad || !ok {
		t.Fatalf("undo failed: ok=%v bad=%v", ok, bad)
	}

	var doc map[string]any
	_ = json.Unmarshal(plan.after, &doc)
	servers := doc["mcpServers"].(map[string]any)
	if _, ok := servers["downshift"]; ok {
		t.Error("downshift was not removed")
	}
	if _, ok := servers["other"]; !ok {
		t.Error("other server was accidentally removed")
	}

	// Test mcpUndo execution
	code := mcpUndo(opts, []setupTarget{target})
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

func TestPlanMcpJSON_SymlinkCoexistence(t *testing.T) {
	home := t.TempDir()
	geminiConfigDir := filepath.Join(home, ".gemini", "config")
	geminiAntigravityDir := filepath.Join(home, ".gemini", "antigravity")
	if err := os.MkdirAll(geminiConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(geminiAntigravityDir, 0o755); err != nil {
		t.Fatal(err)
	}

	realFile := filepath.Join(geminiConfigDir, "mcp_config.json")
	initialContent := []byte(`{
  "mcpServers": {
    "existing-server": {
      "command": "node",
      "args": ["server.js"]
    }
  }
}`)
	if err := os.WriteFile(realFile, initialContent, 0o644); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(geminiAntigravityDir, "mcp_config.json")
	if err := os.Symlink(realFile, symlinkPath); err != nil {
		t.Skipf("symlink creation skipped: %v", err)
	}

	opts := setupOptions{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Mcp:    true,
	}
	target := setupTarget{
		Name: "antigravity",
		Mcp:  &setupMcp{Kind: mcpJSONMap, File: ".gemini/config/mcp_config.json"},
	}

	plan, ok := planMcpJSON(opts, target)
	if !ok {
		t.Fatal("planMcpJSON returned !ok")
	}
	if _, err := setupWrite(plan, "symstamp"); err != nil {
		t.Fatalf("setupWrite: %v", err)
	}

	symRaw, err := os.ReadFile(symlinkPath)
	if err != nil {
		t.Fatalf("ReadFile via symlink: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(symRaw, &doc); err != nil {
		t.Fatalf("unmarshal via symlink: %v", err)
	}
	servers := doc["mcpServers"].(map[string]any)
	if _, ok := servers["existing-server"]; !ok {
		t.Error("existing-server missing via symlink")
	}
	if _, ok := servers["downshift"]; !ok {
		t.Error("downshift missing via symlink")
	}

	// Reverse symlink scenario: .gemini/config/mcp_config.json is a symlink to real file
	home2 := t.TempDir()
	dir1 := filepath.Join(home2, ".gemini", "antigravity")
	dir2 := filepath.Join(home2, ".gemini", "config")
	_ = os.MkdirAll(dir1, 0o755)
	_ = os.MkdirAll(dir2, 0o755)

	targetRealFile := filepath.Join(dir1, "mcp_config.json")
	if err := os.WriteFile(targetRealFile, initialContent, 0o644); err != nil {
		t.Fatal(err)
	}
	configSymlink := filepath.Join(dir2, "mcp_config.json")
	if err := os.Symlink(targetRealFile, configSymlink); err != nil {
		t.Skipf("symlink creation skipped: %v", err)
	}

	opts2 := setupOptions{
		Home:   home2,
		Binary: "/usr/local/bin/downshift",
		Mcp:    true,
	}
	plan2, ok2 := planMcpJSON(opts2, target)
	if !ok2 {
		t.Fatal("planMcpJSON with symlinked config returned !ok")
	}
	canonicalTarget, err := filepath.EvalSymlinks(targetRealFile)
	if err != nil {
		canonicalTarget = targetRealFile
	}
	if plan2.path != canonicalTarget {
		t.Errorf("plan.path = %q, want canonical %q", plan2.path, canonicalTarget)
	}
	if _, err := setupWrite(plan2, "symstamp2"); err != nil {
		t.Fatalf("setupWrite: %v", err)
	}

	fi, err := os.Lstat(configSymlink)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("configSymlink was replaced by a regular file, want symlink preserved")
	}
}
