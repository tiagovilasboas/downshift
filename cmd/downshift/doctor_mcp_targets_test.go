// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

func mcpFixture(t *testing.T, tgt setup.Target, content *string) string {
	t.Helper()
	home := t.TempDir()
	testenv.SetHome(t, home)
	if err := os.MkdirAll(filepath.Join(home, filepath.FromSlash(tgt.ConfigDir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if content != nil {
		f := filepath.Join(home, filepath.FromSlash(tgt.Mcp.File))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(*content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func okMcpContent(m *setup.Mcp) string {
	switch {
	case m.Kind == setup.McpTOMLBlock:
		return setup.McpBlockBegin + "\n" + setup.McpTOMLBody("/x/downshift") + "\n" + setup.McpBlockEnd + "\n"
	case m.Layout == setup.McpLayoutOpenCode:
		return `{"mcp":{"servers":{"downshift":{"type":"local","command":["/x/downshift","mcp","serve"]}}}}`
	}
	return `{"mcpServers":{"downshift":{"command":"/x/downshift","args":["mcp","serve"]}}}`
}

func TestMcpEntryInFileManualTOML(t *testing.T) {
	m := &setup.Mcp{Kind: setup.McpTOMLBlock}
	for _, input := range []string{"[mcp_servers.downshift]\ncommand = 'downshift'\n", "[mcp_servers]\ndownshift = { command = 'downshift' }\n"} {
		found, err := mcpEntryInFile([]byte(input), m)
		if err != nil || !found {
			t.Fatalf("manual entry not detected: found=%v err=%v", found, err)
		}
	}
}

func TestCheckMcpTargetsEveryFileTarget(t *testing.T) {
	n := 0
	for _, tgt := range setup.Targets {
		if tgt.Mcp == nil || tgt.Mcp.Kind == setup.McpViaCLI {
			continue
		}
		n++
		malformed, empty, ok := "{not json", `{"other":{}}`, okMcpContent(tgt.Mcp)
		if tgt.Mcp.Kind == setup.McpTOMLBlock {
			malformed, empty = setup.McpBlockBegin+"\n", "[other]\nx = 1\n"
		}
		fix := "downshift setup " + tgt.Name
		cases := []struct {
			name    string
			content *string
			status  string
			fix     string
		}{
			{"registered", &ok, doctorOK, ""},
			{"missing file", nil, doctorWarn, fix},
			{"no entry", &empty, doctorWarn, fix},
			{"malformed", &malformed, doctorWarn, fix},
		}
		for _, c := range cases {
			t.Run(tgt.Name+"/"+c.name, func(t *testing.T) {
				home := mcpFixture(t, tgt, c.content)
				var rows [][4]string
				checkMcpTargets(func(id, st, d, f string) { rows = append(rows, [4]string{id, st, d, f}) }, home, []setup.Target{tgt})
				if len(rows) != 1 || rows[0][1] != c.status || rows[0][3] != c.fix {
					t.Fatalf("rows = %v, want one %s row with fix %q", rows, c.status, c.fix)
				}
				if !strings.HasPrefix(rows[0][0], tgt.Name) {
					t.Errorf("row id %q lacks target name", rows[0][0])
				}
			})
		}
		t.Run(tgt.Name+"/not installed", func(t *testing.T) {
			rows := 0
			checkMcpTargets(func(string, string, string, string) { rows++ }, t.TempDir(), []setup.Target{tgt})
			if rows != 0 {
				t.Errorf("not installed target produced %d rows, want 0", rows)
			}
		})
	}
	if n < 6 {
		t.Errorf("only %d file targets checked, setup registers at least 6", n)
	}
}
