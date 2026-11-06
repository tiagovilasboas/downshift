// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/setup/configfile"
)

func TestDropModDirs(t *testing.T) {
	sep := string(os.PathListSeparator)
	mod := filepath.FromSlash("/src/downshift/mods/downshift-agents")
	other := filepath.FromSlash("/src/other-mod")
	cases := map[string]struct{ in, want string }{
		"only the mod, with our comment": {
			in:   "alias ll=ls\n# downshift-agents mod from the worktree\nexport CLAUDE_CODE_PLUGIN_DIRS=\"" + mod + "\"\n",
			want: "alias ll=ls\n",
		},
		"a foreign comment stays": {
			in:   "# my plugins\nexport CLAUDE_CODE_PLUGIN_DIRS='" + mod + "'\n",
			want: "# my plugins\n",
		},
		"another folder is kept": {
			in:   "export CLAUDE_CODE_PLUGIN_DIRS=\"" + other + sep + mod + "\"\n",
			want: "export CLAUDE_CODE_PLUGIN_DIRS=\"" + other + "\"\n",
		},
		"no export keyword, trailing slash": {
			in:   "CLAUDE_CODE_PLUGIN_DIRS=" + mod + string(filepath.Separator) + "\n",
			want: "",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, changed := dropModDirs(c.in, []string{"downshift-agents"})
			if !changed || got != c.want {
				t.Fatalf("changed=%v\ngot:  %q\nwant: %q", changed, got, c.want)
			}
		})
	}
	if _, changed := dropModDirs("export CLAUDE_CODE_PLUGIN_DIRS=\""+other+"\"\n", []string{"downshift-agents"}); changed {
		t.Fatal("a line without the mod must stay")
	}
}

func TestSetupRemovesTheModFromAShellProfile(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	line := "export CLAUDE_CODE_PLUGIN_DIRS=\"" + filepath.FromSlash("/src/mods/downshift-agents") + "\"\n"
	e.write(".zshrc", "alias ll=ls\n"+line)
	code, out, errOut := e.run("--yes", "claude-code")
	if code != 0 || !strings.Contains(out, "would run it twice") {
		t.Fatalf("exit %d, want the profile change explained\n%s\n%s", code, out, errOut)
	}
	if got := e.read(".zshrc"); got != "alias ll=ls\n" {
		t.Fatalf("profile not cleaned: %q", got)
	}
	if code, out, errOut := e.run("--undo", "--yes", "claude-code"); code != 0 {
		t.Fatalf("undo exit %d\n%s\n%s", code, out, errOut)
	}
	if got := e.read(".zshrc"); got != "alias ll=ls\n"+line {
		t.Fatalf("undo did not restore the profile: %q", got)
	}
}

func TestSetupNoModsLeavesTheShellProfile(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	text := "export CLAUDE_CODE_PLUGIN_DIRS=\"" + filepath.FromSlash("/src/mods/downshift-agents") + "\"\n"
	e.write(".zshrc", text)
	if code, out, errOut := e.run("--yes", "--no-mods", "claude-code"); code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if got := e.read(".zshrc"); got != text {
		t.Fatalf("--no-mods changed the profile: %q", got)
	}
}

func TestSetupDoesNotWriteThroughALinkedProfile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	e := newSetupEnv(t, ".claude")
	text := "export CLAUDE_CODE_PLUGIN_DIRS=\"/src/mods/downshift-agents\"\n"
	e.write("dotfiles/zshrc", text)
	if err := os.Symlink(e.path("dotfiles/zshrc"), e.path(".zshrc")); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := e.run("--yes", "claude-code")
	if code != 0 || !strings.Contains(out, "is a link") {
		t.Fatalf("exit %d, want a skip for the link\n%s\n%s", code, out, errOut)
	}
	if got := e.read("dotfiles/zshrc"); got != text {
		t.Fatalf("setup wrote through the link: %q", got)
	}
}

// A bridge file linked into a dotfiles repository is never written through,
// even when the linked file carries setup's ownership marker: the dry run and
// the real run both name the link, and the linked file stays as it was.
func TestSetupDoesNotWriteThroughALinkedHarnessFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	e := newSetupEnv(t, ".config/opencode/plugins/downshift")
	text := "// " + configfile.OwnedPhrase + "\nold bridge\n"
	e.write("dotfiles/index.ts", text)
	if err := os.Symlink(e.path("dotfiles/index.ts"), e.path(".config/opencode/plugins/downshift/index.ts")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--dry-run", "opencode"}, {"--yes", "opencode"}} {
		_, out, errOut := e.run(args...)
		if !strings.Contains(out+errOut, "is a link") || strings.Contains(out+errOut, "dotfiles") {
			t.Errorf("%v: want the link named, not its target\n%s\n%s", args, out, errOut)
		}
		if got := e.read("dotfiles/index.ts"); got != text {
			t.Fatalf("%v: setup wrote through the link: %q", args, got)
		}
	}
}
