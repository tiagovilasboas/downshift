// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const openCodeMcpRel = ".config/opencode/opencode.jsonc"

func openCodeMcpTarget() Target {
	return Target{
		Name: "opencode",
		Mcp:  &Mcp{Kind: McpJSONMap, File: openCodeMcpRel, Layout: McpLayoutOpenCode},
	}
}

func openCodeMcpOpts(home string, out *bytes.Buffer) Options {
	return Options{Home: home, Binary: "/usr/local/bin/downshift", Mcp: true, Out: out, Err: out}
}

func writeOpenCodeConfig(t *testing.T, home, body string) {
	t.Helper()
	path := filepath.Join(home, filepath.FromSlash(openCodeMcpRel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The entry goes under mcp.servers in OpenCode's own shape: type local, one
// command list and an environment object carrying the setup marker.
func TestPlanOpenCodeMcp_CreatesTheServerEntry(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	plan, ok := PlanMcpJSON(openCodeMcpOpts(home, &out), openCodeMcpTarget())
	if !ok {
		t.Fatalf("PlanMcpJSON returned !ok:\n%s", out.String())
	}
	var doc struct {
		MCP struct {
			Servers map[string]struct {
				Type        string            `json:"type"`
				Command     []string          `json:"command"`
				Environment map[string]string `json:"environment"`
			} `json:"servers"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(plan.After, &doc); err != nil {
		t.Fatalf("unmarshal after: %v\n%s", err, plan.After)
	}
	entry, found := doc.MCP.Servers[McpServerName]
	if !found {
		t.Fatalf("no mcp.servers.%s in %s", McpServerName, plan.After)
	}
	if entry.Type != "local" || len(entry.Command) != 3 || entry.Command[0] != "/usr/local/bin/downshift" ||
		entry.Command[1] != "mcp" || entry.Command[2] != "serve" {
		t.Errorf("entry = %+v", entry)
	}
	if entry.Environment[McpMarkerKey] != McpMarkerValue {
		t.Errorf("setup marker missing: %v", entry.Environment)
	}
}

// Other config keys and other servers keep their values and their order; the
// file is re-rendered, so only the values are compared.
func TestPlanOpenCodeMcp_KeepsTheRestOfTheFile(t *testing.T) {
	home := t.TempDir()
	writeOpenCodeConfig(t, home, `{
  "$schema": "https://opencode.ai/config.json",
  "theme": "dark",
  "mcp": {
    "timeout": {"startup": 20000},
    "servers": {
      "zeta": {"type": "local", "command": ["zeta-bin"]}
    }
  }
}`)
	var out bytes.Buffer
	plan, ok := PlanMcpJSON(openCodeMcpOpts(home, &out), openCodeMcpTarget())
	if !ok {
		t.Fatalf("PlanMcpJSON returned !ok:\n%s", out.String())
	}
	var doc struct {
		Theme string `json:"theme"`
		MCP   struct {
			Timeout struct {
				Startup int `json:"startup"`
			} `json:"timeout"`
			Servers map[string]json.RawMessage `json:"servers"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(plan.After, &doc); err != nil {
		t.Fatalf("unmarshal after: %v\n%s", err, plan.After)
	}
	if doc.Theme != "dark" || doc.MCP.Timeout.Startup != 20000 {
		t.Errorf("neighbour values changed: theme=%q timeout=%d", doc.Theme, doc.MCP.Timeout.Startup)
	}
	if _, ok := doc.MCP.Servers["zeta"]; !ok {
		t.Errorf("zeta was dropped:\n%s", plan.After)
	}
	if bytes.Index(plan.After, []byte(`"zeta"`)) > bytes.Index(plan.After, []byte(`"downshift"`)) {
		t.Errorf("zeta must keep its place before the new entry:\n%s", plan.After)
	}
}

// A second run finds its own entry and writes nothing.
func TestPlanOpenCodeMcp_IsIdempotent(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	plan, ok := PlanMcpJSON(openCodeMcpOpts(home, &out), openCodeMcpTarget())
	if !ok {
		t.Fatal("first plan !ok")
	}
	writeOpenCodeConfig(t, home, string(plan.After))
	out.Reset()
	if _, ok := PlanMcpJSON(openCodeMcpOpts(home, &out), openCodeMcpTarget()); ok {
		t.Fatal("second plan wrote the same entry again")
	}
	if !bytes.Contains(out.Bytes(), []byte("already up to date")) {
		t.Errorf("no up-to-date note:\n%s", out.String())
	}
}

// An entry with the same name that setup did not add is left alone.
func TestPlanOpenCodeMcp_LeavesAForeignEntryAlone(t *testing.T) {
	home := t.TempDir()
	writeOpenCodeConfig(t, home, `{"mcp": {"servers": {"downshift": {"type": "local", "command": ["/x/ds", "mcp", "serve"]}}}}`)
	var out bytes.Buffer
	if _, ok := PlanMcpJSON(openCodeMcpOpts(home, &out), openCodeMcpTarget()); ok {
		t.Fatal("a foreign entry was replaced")
	}
	if !bytes.Contains(out.Bytes(), []byte("did not add")) {
		t.Errorf("no warning for the foreign entry:\n%s", out.String())
	}
}

// Undo removes the entry setup added and nothing else.
func TestPlanOpenCodeMcpUndo_RemovesOnlyTheEntry(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	first, ok := PlanMcpJSON(openCodeMcpOpts(home, &out), openCodeMcpTarget())
	if !ok {
		t.Fatal("setup plan !ok")
	}
	withZeta := bytes.Replace(first.After, []byte(`"servers": {`), []byte(`"servers": {"zeta": {"type": "local", "command": ["zeta-bin"]}, `), 1)
	writeOpenCodeConfig(t, home, string(withZeta))

	undo, ok, bad := PlanMcpJSONUndo(Options{Home: home, Binary: "/usr/local/bin/downshift", Undo: true}, openCodeMcpTarget())
	if bad || !ok {
		t.Fatalf("undo: ok=%v bad=%v", ok, bad)
	}
	if bytes.Contains(undo.After, []byte(`"downshift"`)) {
		t.Errorf("undo kept the downshift entry:\n%s", undo.After)
	}
	if !bytes.Contains(undo.After, []byte(`"zeta"`)) {
		t.Errorf("undo removed zeta:\n%s", undo.After)
	}
}
