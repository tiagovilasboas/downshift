// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestSetupUndoDoesNotFollowLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	e := newSetupEnv(t, ".config/opencode", "dotfiles")
	if code, out, errOut := e.run("--yes", "opencode"); code != 0 {
		t.Fatalf("setup: %d\n%s\n%s", code, out, errOut)
	}
	// Move the plugins folder into a dotfiles repo and link it back; link
	// AGENTS.md to a copy there.
	if err := os.Rename(e.path(".config/opencode/plugins"), e.path("dotfiles/plugins")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(e.path("dotfiles/plugins"), e.path(".config/opencode/plugins")); err != nil {
		t.Fatal(err)
	}
	e.write("dotfiles/plugins/other.txt", "keep")
	agents := e.read(".config/opencode/AGENTS.md")
	e.write("dotfiles/AGENTS.md", agents)
	if err := os.Remove(e.path(".config/opencode/AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(e.path("dotfiles/AGENTS.md"), e.path(".config/opencode/AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	bridge := e.read("dotfiles/plugins/downshift/index.ts")

	for _, args := range [][]string{{"--undo", "--dry-run", "opencode"}, {"--undo", "--yes", "opencode"}} {
		_, out, errOut := e.run(args...)
		if !strings.Contains(out+errOut, "is a link") {
			t.Errorf("%v: want the links reported\n%s\n%s", args, out, errOut)
		}
		if got := e.read("dotfiles/plugins/downshift/index.ts"); got != bridge {
			t.Fatalf("%v: undo changed a file through the link", args)
		}
		if got := e.read("dotfiles/AGENTS.md"); got != agents {
			t.Fatalf("%v: undo rewrote the linked AGENTS.md", args)
		}
		for _, l := range []string{".config/opencode/plugins", ".config/opencode/AGENTS.md"} {
			if st, err := os.Lstat(e.path(l)); err != nil || st.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("%v: %s is no longer a link (%v)", args, l, err)
			}
		}
		if e.read("dotfiles/plugins/other.txt") != "keep" {
			t.Fatalf("%v: other.txt lost", args)
		}
	}
}

// Undo must not widen a 0600 file, not even briefly.
func TestUndoRestoresTheOriginalMode(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	path := e.path(".claude/settings.json")
	if err := os.WriteFile(path, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := e.run("claude-code", "--yes"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	if code, out, _ := e.run("--undo", "--yes"); code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode after undo = %v, want 0600", fi.Mode().Perm())
	}
}
