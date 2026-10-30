// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Claude Desktop Chat reads this file (macOS layout). The target is MCP-only:
// it has no hook file, so every assertion here is about the mcpServers map.
const claudeDesktopConfigRel = "Library/Application Support/Claude/claude_desktop_config.json"

func claudeDesktopDirRel() string { return filepath.Dir(claudeDesktopConfigRel) }

// desktopRun drives setupRun for the claude-desktop target with an isolated
// HOME and state dir, no CLI (the JSON-map writer needs none) and a fixed clock.
func (e *setupEnv) desktopRun(args ...string) (code int, out, errOut string) {
	e.t.Helper()
	e.t.Setenv("HOME", e.home)
	e.t.Setenv("DOWNSHIFT_STATE_DIR", filepath.Join(e.home, ".downshift"))
	var o, er bytes.Buffer
	opts := setupOptions{
		Home: e.home, Binary: testBin, StateDir: filepath.Join(e.home, ".downshift"),
		SkipDiscovery: true, NoPlugin: true, Mcp: true,
		In: strings.NewReader(""), Out: &o, Err: &er,
	}
	opts.Now = func() time.Time {
		e.tick++
		return time.Date(2026, 10, 30, 12, 0, 0, e.tick*1000, time.UTC)
	}
	for _, a := range args {
		switch a {
		case "--dry-run":
			opts.DryRun = true
		case "--undo":
			opts.Undo = true
		case "--yes":
			opts.Yes = true
		case "--no-mcp":
			opts.Mcp = false
		default:
			opts.Names = append(opts.Names, a)
		}
	}
	return setupRun(opts), o.String(), er.String()
}

// desktopServers reads the mcpServers map from the written config.
func (e *setupEnv) desktopServers() map[string]any {
	e.t.Helper()
	m := parseDoc(e.t, e.read(claudeDesktopConfigRel))
	s, _ := m["mcpServers"].(map[string]any)
	return s
}

func TestSetupClaudeDesktopMergesAndPreservesEverythingElse(t *testing.T) {
	e := newSetupEnv(t, claudeDesktopDirRel())
	// Another server and an unrelated top-level key Claude Desktop owns.
	existing := `{
  "coworkUserFilesPath": "/Users/me/CoworkFiles",
  "mcpServers": {
    "weather": {
      "command": "node",
      "args": ["weather.js"]
    }
  }
}
`
	e.write(claudeDesktopConfigRel, existing)

	code, out, errOut := e.desktopRun("claude-desktop", "--yes")
	if code != 0 {
		t.Fatalf("code %d\nout:%s\nerr:%s", code, out, errOut)
	}

	after := e.read(claudeDesktopConfigRel)
	// Unrelated key and the other server survive byte-for-byte in value.
	if !strings.Contains(after, `"coworkUserFilesPath": "/Users/me/CoworkFiles"`) {
		t.Errorf("lost the unrelated top-level key:\n%s", after)
	}
	servers := e.desktopServers()
	if servers["weather"] == nil {
		t.Errorf("lost the weather server:\n%s", after)
	}
	ds, ok := servers["downshift"].(map[string]any)
	if !ok {
		t.Fatalf("no downshift server:\n%s", after)
	}
	if ds["command"] != testBin {
		t.Errorf("command = %v, want %s", ds["command"], testBin)
	}
	env, _ := ds["env"].(map[string]any)
	if env[mcpMarkerKey] != mcpMarkerValue {
		t.Errorf("missing setup marker: %v", ds["env"])
	}
	// Key order: the unrelated key stays before mcpServers.
	if strings.Index(after, "coworkUserFilesPath") > strings.Index(after, "mcpServers") {
		t.Errorf("top-level order changed:\n%s", after)
	}
	// The after-write note names the restart and the Chat capability limit.
	if !strings.Contains(out, "Restart Claude Desktop") || !strings.Contains(out, "providers/status/playbook/apply") {
		t.Errorf("missing the restart/capability note:\n%s", out)
	}
}

func TestSetupClaudeDesktopGoldenMerge(t *testing.T) {
	e := newSetupEnv(t, claudeDesktopDirRel())
	e.write(claudeDesktopConfigRel, `{
  "coworkUserFilesPath": "/data",
  "mcpServers": {
    "weather": {"command": "node", "args": ["w.js"]}
  }
}
`)
	if code, out, errOut := e.desktopRun("claude-desktop", "--yes"); code != 0 {
		t.Fatalf("code %d\nout:%s\nerr:%s", code, out, errOut)
	}
	want := `{
  "coworkUserFilesPath": "/data",
  "mcpServers": {
    "weather": {
      "command": "node",
      "args": [
        "w.js"
      ]
    },
    "downshift": {
      "command": "` + testBin + `",
      "args": [
        "mcp",
        "serve"
      ],
      "env": {
        "` + mcpMarkerKey + `": "` + mcpMarkerValue + `"
      }
    }
  }
}
`
	if got := e.read(claudeDesktopConfigRel); got != want {
		t.Errorf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSetupClaudeDesktopIdempotent(t *testing.T) {
	e := newSetupEnv(t, claudeDesktopDirRel())
	e.write(claudeDesktopConfigRel, `{"mcpServers": {}}`+"\n")
	if code, _, errOut := e.desktopRun("claude-desktop", "--yes"); code != 0 {
		t.Fatalf("first run code: %s", errOut)
	}
	first := e.read(claudeDesktopConfigRel)
	code, out, errOut := e.desktopRun("claude-desktop", "--yes")
	if code != 0 {
		t.Fatalf("second run code %d: %s", code, errOut)
	}
	if got := e.read(claudeDesktopConfigRel); got != first {
		t.Errorf("second run changed the file:\n--- first ---\n%s\n--- second ---\n%s", first, got)
	}
	if !strings.Contains(out, "already up to date") {
		t.Errorf("second run did not report up to date:\n%s", out)
	}
}

func TestSetupClaudeDesktopNoOpWhenDirAbsent(t *testing.T) {
	// HOME without the Claude directory. Naming the target explicitly must fail
	// with "not found" and write nothing.
	e := newSetupEnv(t)
	code, _, errOut := e.desktopRun("claude-desktop", "--yes")
	if code == 0 {
		t.Fatalf("expected a non-zero code when the Claude dir is absent")
	}
	if !strings.Contains(errOut, "claude-desktop not found") {
		t.Errorf("missing not-found message:\n%s", errOut)
	}
	if _, err := os.Stat(e.path(claudeDesktopConfigRel)); !os.IsNotExist(err) {
		t.Errorf("the config file was created when the Claude dir was absent: %v", err)
	}
	if _, err := os.Stat(e.path(claudeDesktopDirRel())); !os.IsNotExist(err) {
		t.Errorf("the Claude dir was created: %v", err)
	}
}

func TestSetupClaudeDesktopDryRunWritesNothing(t *testing.T) {
	e := newSetupEnv(t, claudeDesktopDirRel())
	before := `{"mcpServers": {"weather": {"command": "node", "args": []}}}` + "\n"
	e.write(claudeDesktopConfigRel, before)
	code, out, errOut := e.desktopRun("claude-desktop", "--dry-run")
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	if got := e.read(claudeDesktopConfigRel); got != before {
		t.Errorf("dry run wrote to the file:\n%s", got)
	}
	for _, want := range []string{"would change", "+  command: " + testBin, "Dry run: no MCP registration was written."} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output lacks %q:\n%s", want, out)
		}
	}
}

func TestSetupClaudeDesktopUndoRemovesOnlyTheMember(t *testing.T) {
	e := newSetupEnv(t, claudeDesktopDirRel())
	e.write(claudeDesktopConfigRel, `{
  "coworkUserFilesPath": "/keep",
  "mcpServers": {"weather": {"command": "node", "args": []}}
}
`)
	if code, _, errOut := e.desktopRun("claude-desktop", "--yes"); code != 0 {
		t.Fatalf("setup code: %s", errOut)
	}
	if e.desktopServers()["downshift"] == nil {
		t.Fatal("downshift was not registered before undo")
	}
	if code, out, errOut := e.desktopRun("claude-desktop", "--undo", "--yes"); code != 0 {
		t.Fatalf("undo code %d\nout:%s\nerr:%s", code, out, errOut)
	}
	servers := e.desktopServers()
	if servers["downshift"] != nil {
		t.Errorf("downshift survived undo: %v", servers)
	}
	if servers["weather"] == nil {
		t.Errorf("undo removed the other server: %v", servers)
	}
	if !strings.Contains(e.read(claudeDesktopConfigRel), `"coworkUserFilesPath": "/keep"`) {
		t.Errorf("undo dropped the unrelated key")
	}
}
