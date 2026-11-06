// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanMcpJSON_CreatesNewFileWhenMissing(t *testing.T) {
	home := t.TempDir()
	opts := Options{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Mcp:    true,
	}
	target := Target{
		Name: "cursor",
		Mcp:  &Mcp{Kind: McpJSONMap, File: ".cursor/mcp.json"},
	}

	plan, ok := PlanMcpJSON(opts, target)
	if !ok {
		t.Fatal("PlanMcpJSON returned !ok")
	}
	if plan.Existed {
		t.Error("plan.existed was true for missing file")
	}

	var doc map[string]any
	if err := json.Unmarshal(plan.After, &doc); err != nil {
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

	opts := Options{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Mcp:    true,
	}
	target := Target{
		Name: "cursor",
		Mcp:  &Mcp{Kind: McpJSONMap, File: ".cursor/mcp.json"},
	}

	plan, ok := PlanMcpJSON(opts, target)
	if !ok {
		t.Fatal("PlanMcpJSON returned !ok")
	}

	var doc map[string]any
	_ = json.Unmarshal(plan.After, &doc)
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
	opts := Options{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Mcp:    true,
		Out:    &stdout,
	}
	target := Target{
		Name: "cursor",
		Mcp:  &Mcp{Kind: McpJSONMap, File: ".cursor/mcp.json"},
	}

	_, ok := PlanMcpJSON(opts, target)
	if ok {
		t.Error("PlanMcpJSON returned ok for already up to date")
	}
	if !strings.Contains(stdout.String(), "already up to date") {
		t.Errorf("stdout = %q; want 'already up to date'", stdout.String())
	}
}

func TestPlanMcpJSON_HarnessTargetsCreationAndIdempotence(t *testing.T) {
	targets := []Target{
		{
			Name: "cursor",
			Mcp:  &Mcp{Kind: McpJSONMap, File: ".cursor/mcp.json"},
		},
		{
			Name: "antigravity",
			Mcp:  &Mcp{Kind: McpJSONMap, File: ".gemini/config/mcp_config.json"},
		},
		{
			Name: "kirocrew",
			Mcp:  &Mcp{Kind: McpJSONMap, File: ".kiro/settings/mcp.json"},
		},
	}

	for _, tgt := range targets {
		t.Run(tgt.Name, func(t *testing.T) {
			home := t.TempDir()
			var stdout strings.Builder
			opts := Options{
				Home:   home,
				Binary: "/usr/local/bin/downshift",
				Mcp:    true,
				Out:    &stdout,
			}

			plan, ok := PlanMcpJSON(opts, tgt)
			if !ok {
				t.Fatalf("%s: PlanMcpJSON returned !ok", tgt.Name)
			}
			if plan.Existed {
				t.Errorf("%s: plan.existed was true for missing file", tgt.Name)
			}
			if _, err := Write(plan, "stamp1"); err != nil {
				t.Fatalf("%s: Write: %v", tgt.Name, err)
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
			_, ok2 := PlanMcpJSON(opts, tgt)
			if ok2 {
				t.Errorf("%s: second PlanMcpJSON returned ok=true, want false", tgt.Name)
			}
			if !strings.Contains(stdout.String(), "already up to date") {
				t.Errorf("%s: stdout %q does not contain 'already up to date'", tgt.Name, stdout.String())
			}
		})
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

	opts := Options{
		Home:   home,
		Binary: "/usr/local/bin/downshift",
		Mcp:    true,
	}
	target := Target{
		Name: "antigravity",
		Mcp:  &Mcp{Kind: McpJSONMap, File: ".gemini/config/mcp_config.json"},
	}

	plan, ok := PlanMcpJSON(opts, target)
	if !ok {
		t.Fatal("PlanMcpJSON returned !ok")
	}
	if _, err := Write(plan, "symstamp"); err != nil {
		t.Fatalf("Write: %v", err)
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

	// Linked leaf: .gemini/config/mcp_config.json is a symlink; setup refuses it
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

	opts2 := Options{
		Home:   home2,
		Binary: "/usr/local/bin/downshift",
		Mcp:    true,
	}
	var errOut bytes.Buffer
	opts2.Err = &errOut
	if _, ok2 := PlanMcpJSON(opts2, target); ok2 {
		t.Fatal("PlanMcpJSON planned a write through a linked config file")
	}
	if !strings.Contains(errOut.String(), "is a link") {
		t.Errorf("want the link reported:\n%s", errOut.String())
	}
	if got, _ := os.ReadFile(targetRealFile); string(got) != string(initialContent) {
		t.Errorf("the link target changed: %q", got)
	}
}

func TestPlanMcpJSON_CommentedFileGetsAManualHint(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "opencode.jsonc")
	body := "{\n  // my comment\n  \"model\": \"x\"\n}\n"
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var errOut bytes.Buffer
	opts := Options{Home: home, Binary: "/usr/local/bin/downshift", Mcp: true, Err: &errOut}
	target := Target{Name: "opencode", Mcp: &Mcp{Kind: McpJSONMap, File: ".config/opencode/opencode.jsonc", Layout: McpLayoutOpenCode}}
	if _, ok := PlanMcpJSON(opts, target); ok {
		t.Fatal("a commented file must not be planned for a write")
	}
	got, err := os.ReadFile(file)
	if err != nil || string(got) != body {
		t.Fatalf("file changed: %q, %v", got, err)
	}
	for _, want := range []string{"comments", "left untouched", "mcp.servers.downshift", `"downshift": {"type":"local","command":["/usr/local/bin/downshift","mcp","serve"]`} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("warning lacks %q:\n%s", want, errOut.String())
		}
	}
}
