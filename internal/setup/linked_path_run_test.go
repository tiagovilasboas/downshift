// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSetupRefusesALinkedParentDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	e := newSetupEnv(t, "dotfiles/opencode", ".config")
	if err := os.Symlink(e.path("dotfiles/opencode"), e.path(".config/opencode")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--dry-run", "--mcp", "opencode"}, {"--yes", "--mcp", "opencode"}} {
		_, out, errOut := e.run(args...)
		all := out + errOut
		if !strings.Contains(all, "is a link") || strings.Contains(all, "dotfiles") {
			t.Errorf("%v: want the link named, not its target\n%s", args, all)
		}
		if !strings.Contains(all, "~/.config/opencode is a link") {
			t.Errorf("%v: want the linked component named\n%s", args, all)
		}
		if !strings.Contains(all, "the MCP server was not registered") {
			t.Errorf("%v: want the MCP file refused too\n%s", args, all)
		}
		entries, _ := os.ReadDir(e.path("dotfiles/opencode"))
		if len(entries) != 0 {
			t.Fatalf("%v: setup wrote through the linked parent: %v", args, entries)
		}
	}
}

func TestSetupWritesARealDirectoryChain(t *testing.T) {
	e := newSetupEnv(t, ".config/opencode")
	if code, out, errOut := e.run("--yes", "--mcp", "opencode"); code != 0 {
		t.Fatalf("code %d\n%s\n%s", code, out, errOut)
	}
	if _, err := os.Stat(e.path(".config/opencode/plugins/downshift/index.ts")); err != nil {
		t.Fatalf("the bridge was not written, and its missing directories were not created: %v", err)
	}
	if !strings.Contains(e.read(".config/opencode/opencode.jsonc"), "downshift") {
		t.Fatal("the MCP entry was not written")
	}
}

func TestSetupRefusesALinkedProjectPolicyDirectoryOutsideHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	env := newSetupEnv(t, ".claude")
	project, elsewhere := t.TempDir(), t.TempDir()
	if err := os.Symlink(elsewhere, filepath.Join(project, ".downshift")); err != nil {
		t.Fatal(err)
	}
	var out, errw bytes.Buffer
	opts := Options{Home: env.home, Binary: testBin, SkipDiscovery: true, ProjectDir: project,
		Yes: true, In: strings.NewReader(""), Out: &out, Err: &errw,
		Now: func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) }}
	Run(opts)
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Fatalf("setup wrote into the link target: %v", entries)
	}
	if !strings.Contains(out.String()+errw.String(), "is a link") {
		t.Errorf("want the link reported\n%s\n%s", out.String(), errw.String())
	}
}
