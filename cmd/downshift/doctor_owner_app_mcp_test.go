// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

func TestOwnerAppMcpRows(t *testing.T) {
	const withEntry = `{"mcpServers":{"downshift":{"command":"x"}}}`
	const dropped = `{"mcpServers":{"weather":{"command":"y"}}}`
	cases := []struct {
		name     string
		file     string // "" = no file
		backup   bool
		state    setup.OwnerAppState
		wantRow  bool
		status   string
		inDetail string
	}{
		{name: "no file", file: "", backup: true},
		{name: "setup never wrote it", file: dropped},
		{name: "entry kept", file: withEntry, backup: true, wantRow: true, status: doctorOK, inDetail: "present"},
		{name: "entry dropped", file: dropped, backup: true, state: setup.OwnerAppStopped, wantRow: true, status: doctorWarn, inDetail: "rewrote the file while open"},
		{name: "entry dropped, app running", file: dropped, backup: true, state: setup.OwnerAppRunning, wantRow: true, status: doctorWarn, inDetail: "Claude is running now"},
	}
	target := setup.Target{Name: "claude-desktop", Mcp: &setup.Mcp{Kind: setup.McpJSONMap, File: "App/config.json", OwnerApp: "Claude"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "App", "config.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if c.file != "" {
				if err := os.WriteFile(path, []byte(c.file), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if c.backup {
				if err := os.WriteFile(path+setup.BackupInfix+"20261104T000000", []byte("{}"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var rows []string
			var status, fix string
			ownerAppMcpRows(func(id, s, detail, f string) {
				rows = append(rows, id+": "+detail)
				status, fix = s, f
			}, home, []setup.Target{target, {Name: "no-owner", Mcp: &setup.Mcp{Kind: setup.McpJSONMap, File: "App/config.json"}}},
				func(string) setup.OwnerAppState { return c.state })
			if !c.wantRow {
				if len(rows) != 0 {
					t.Fatalf("want no row, got %v", rows)
				}
				return
			}
			if len(rows) != 1 {
				t.Fatalf("want one row, got %v", rows)
			}
			if status != c.status || !strings.Contains(rows[0], c.inDetail) {
				t.Errorf("got %s %q; want %s containing %q", status, rows[0], c.status, c.inDetail)
			}
			if c.status == doctorWarn && !strings.Contains(fix, "Cmd+Q") {
				t.Errorf("fix %q lacks the quit instruction", fix)
			}
		})
	}
}
