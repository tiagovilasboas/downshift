// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

func TestTomlQuote(t *testing.T) {
	cases := map[string]string{
		`/opt/ds/downshift`:          `"/opt/ds/downshift"`,
		`C:\Program Files\ds\ds.exe`: `"C:\\Program Files\\ds\\ds.exe"`,
		`a"b`:                        `"a\"b"`,
		"a\tb":                       `"a\tb"`,
	}
	for in, want := range cases {
		if got := tomlQuote(in); got != want {
			t.Errorf("tomlQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestMcpBlockRoundTrip(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"comments":         "# c1\nkey = 1 # c2\n\n[table]\nx = \"y\"\n",
		"trailing blank":   "a = 1\n\n",
		"another server":   "[mcp_servers.other]\ncommand = \"o\"\n",
		"no final newline": "a = 1",
	}
	for name, orig := range cases {
		t.Run(name, func(t *testing.T) {
			with := string(mergeMarkedBlock([]byte(orig), mcpBlockBegin, mcpBlockEnd, mcpTOMLBody("/b/ds")))
			if !strings.HasPrefix(with, orig) {
				t.Fatalf("text before the block changed:\n%q", with)
			}
			if again := string(mergeMarkedBlock([]byte(with), mcpBlockBegin, mcpBlockEnd, mcpTOMLBody("/b/ds"))); again != with {
				t.Errorf("not idempotent:\n%q\n%q", with, again)
			}
			got := removeMcpBlock(with)
			want := orig
			if name == "no final newline" {
				want = orig + "\n" // the only difference: the missing final newline
			}
			if got != want {
				t.Errorf("round trip:\n got %q\nwant %q", got, want)
			}
		})
	}
}

func TestMcpBlockReplacedWhenBinaryMoves(t *testing.T) {
	first := mergeMarkedBlock([]byte("a = 1\n"), mcpBlockBegin, mcpBlockEnd, mcpTOMLBody("/old/ds"))
	second := string(mergeMarkedBlock(first, mcpBlockBegin, mcpBlockEnd, mcpTOMLBody("/new/ds")))
	if strings.Contains(second, "/old/ds") || strings.Count(second, mcpBlockBegin) != 1 || !strings.Contains(second, "/new/ds") {
		t.Errorf("block not replaced in place:\n%s", second)
	}
}

func TestMcpBlockBoundsUnbalanced(t *testing.T) {
	for _, text := range []string{mcpBlockBegin + "\nx\n", mcpBlockEnd + "\n", mcpBlockEnd + "\n" + mcpBlockBegin + "\n"} {
		if _, _, _, err := mcpBlockBounds(text); err == nil {
			t.Errorf("no error for %q", text)
		}
	}
}

func TestMcpTableOutsideDetection(t *testing.T) {
	yes := []string{"[mcp_servers.downshift]\n", "  [ mcp_servers . downshift ]\n", "[mcp_servers.\"downshift\"]\n", "[mcp_servers.downshift.env]\n",
		"mcp_servers.downshift.command = \"x\"\n", "[mcp_servers]\ndownshift = {}\n"}
	no := []string{"[mcp_servers.other]\n", "# [mcp_servers.downshift]\n", "[mcp_servers.downshift2]\n", "x = \"[mcp_servers.downshift]\"\n",
		"[mcp_servers]\nother = {}\n[x]\ndownshift = 1\n", "[mcp_servers.other]\ndownshift = 1\n"}
	for _, s := range yes {
		if !mcpDefinedOutside(s) {
			t.Errorf("missed %q", s)
		}
	}
	for _, s := range no {
		if mcpDefinedOutside(s) {
			t.Errorf("false positive on %q", s)
		}
	}
}
