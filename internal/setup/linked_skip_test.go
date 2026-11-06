// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestSetupLinkedPluginFolderSkipsOnlyItsFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	e := newSetupEnv(t, ".config/opencode", "dotfiles/plugins")
	if err := os.Symlink(e.path("dotfiles/plugins"), e.path(".config/opencode/plugins")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--dry-run", "--mcp", "opencode"}, {"--yes", "--mcp", "opencode"}} {
		code, out, errOut := e.run(args...)
		all := out + errOut
		if code != 0 {
			t.Errorf("%v: a skipped linked file is not a failure, exit %d\n%s", args, code, all)
		}
		if n := strings.Count(all, "~/.config/opencode/plugins is a link"); n != 1 {
			t.Errorf("%v: want the linked folder named once with its fix, got %d\n%s", args, n, all)
		}
		if strings.Contains(all, "nothing else of opencode is changed") {
			t.Errorf("%v: the rest of the harness must not be held back\n%s", args, all)
		}
		if entries, _ := os.ReadDir(e.path("dotfiles/plugins")); len(entries) != 0 {
			t.Fatalf("%v: setup wrote through the link: %v", args, entries)
		}
	}
	for _, f := range []string{".config/opencode/AGENTS.md", ".config/opencode/opencode.jsonc"} {
		if _, err := os.Lstat(e.path(f)); err != nil {
			t.Errorf("%s was not written although only the plugin folder is linked: %v", f, err)
		}
	}
}

func TestSetupLinkedInstructionsFileStillRegistersMCP(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	e := newSetupEnv(t, ".gemini/config")
	e.write(".gemini/config/AGENTS.md", "mine\n")
	if err := os.Symlink(e.path(".gemini/config/AGENTS.md"), e.path(".gemini/AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := e.run("--yes", "--mcp", "antigravity")
	all := out + errOut
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, all)
	}
	if n := strings.Count(all, "~/.gemini/AGENTS.md is a link"); n != 1 || !strings.Contains(all, "edit the real file by hand") {
		t.Errorf("want the skipped file named once with how to fix it, got %d\n%s", n, all)
	}
	if strings.Contains(all, "MCP server not registered") || strings.Contains(all, "nothing else") {
		t.Errorf("the MCP registration must not be held back\n%s", all)
	}
	if got := e.read(".gemini/config/AGENTS.md"); got != "mine\n" {
		t.Errorf("setup wrote through the link: %q", got)
	}
	if !strings.Contains(e.read(".gemini/config/mcp_config.json"), "downshift") {
		t.Error("the MCP entry was not registered")
	}
	if !strings.Contains(out, "~/.gemini/config/mcp_config.json written") {
		t.Errorf("the written file must be named\n%s", out)
	}
}
