// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func cursorTarget() setupTarget {
	return setupTarget{Name: "cursor", Mcp: &setupMcp{Kind: mcpJSONMap, File: ".cursor/mcp.json"}}
}

func TestPlanMcpJSON_KeepsNeighbourTextOrderAndNumbers(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".cursor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{
  "zeta": 1,
  "mcpServers": {
    "web": {
      "url": "https://x.test/a?b=1&c=2",
      "args": ["<tag>", "a>b", "&amp;", 12345678901234567890],
      "id": 9007199254740993
    },
    "alpha": {"command": "node"}
  },
  "alpha": true
}
`
	if err := os.WriteFile(filepath.Join(dir, "mcp.json"), []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	o := setupOptions{Home: home, Binary: "/bin/downshift", Mcp: true, DryRun: true, Out: &out, Err: &out}
	plan, ok := planMcpJSON(o, cursorTarget())
	if !ok {
		t.Fatal("planMcpJSON returned !ok")
	}
	after := string(plan.after)
	for _, want := range []string{
		`"https://x.test/a?b=1&c=2"`, `"<tag>"`, `"a>b"`, `"&amp;"`,
		`12345678901234567890`, `9007199254740993`,
	} {
		if !strings.Contains(after, want) {
			t.Errorf("after lost %s:\n%s", want, after)
		}
	}
	for _, bad := range []string{`\u0026`, `\u003c`, `\u003e`, `e+`} {
		if strings.Contains(after, bad) {
			t.Errorf("after contains %q:\n%s", bad, after)
		}
	}
	zeta, ms, alpha := strings.Index(after, `"zeta"`), strings.Index(after, `"mcpServers"`), strings.LastIndex(after, `"alpha"`)
	if !(zeta < ms && ms < alpha) {
		t.Errorf("top-level order changed:\n%s", after)
	}
	if strings.Index(after, `"web"`) > strings.Index(after, `"alpha"`) || strings.Index(after, `"alpha"`) > strings.Index(after, `"downshift"`) {
		t.Errorf("server order changed:\n%s", after)
	}

	preview := out.String()
	if !strings.Contains(preview, "+  command: /bin/downshift") || !strings.Contains(preview, "env keys: "+mcpMarkerKey) {
		t.Errorf("dry-run does not show the downshift entry:\n%s", preview)
	}
	if strings.Contains(preview, "x.test") || strings.Contains(preview, "zeta") {
		t.Errorf("dry-run shows lines outside the downshift entry:\n%s", preview)
	}
}

func TestPlanMcpJSON_DryRunShowsChangedEntry(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".cursor")
	_ = os.MkdirAll(dir, 0o755)
	old := `{"mcpServers":{"downshift":{"command":"/old/ds","args":["mcp","serve"],"env":{"` + mcpMarkerKey + `":"` + mcpMarkerValue + `"}}}}`
	_ = os.WriteFile(filepath.Join(dir, "mcp.json"), []byte(old), 0o600)
	var out bytes.Buffer
	o := setupOptions{Home: home, Binary: "/new/ds", Mcp: true, DryRun: true, Out: &out, Err: &out}
	if _, ok := planMcpJSON(o, cursorTarget()); !ok {
		t.Fatal("expected a plan")
	}
	if !strings.Contains(out.String(), "-  command: /old/ds") || !strings.Contains(out.String(), "+  command: /new/ds") {
		t.Errorf("diff missing old/new command:\n%s", out.String())
	}
}

func TestSetupWrite_McpJSONLeavesDirectoryMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	home := t.TempDir()
	dir := filepath.Join(home, ".cursor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	o := setupOptions{Home: home, Binary: "/bin/downshift", Mcp: true}
	plan, ok := planMcpJSON(o, cursorTarget())
	if !ok {
		t.Fatal("expected a plan")
	}
	if plan.mode != 0 {
		t.Fatalf("plan.mode = %v, want 0", plan.mode)
	}
	if _, err := setupWrite(plan, "t1"); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o755 {
		t.Errorf("directory mode = %v, want 0755", st.Mode().Perm())
	}
	fst, err := os.Stat(filepath.Join(dir, "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	if fst.Mode().Perm() != 0o600 {
		t.Errorf("new file mode = %v, want 0600", fst.Mode().Perm())
	}
}
