// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/opencode"
)

const agentWithPermission = "---\npermission:\n  bash: deny\n---\n"

// TestShellBlocked_Hardening covers the doubts that keep the compactor off:
// both directories of the call, ancestors, modes, links, MDM, caps.
func TestShellBlocked_Hardening(t *testing.T) {
	needsSymlink := func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need privileges on Windows")
		}
	}
	cases := []struct {
		name  string
		setup func(t *testing.T, home, dir string) (cwd, session string)
		want  bool
	}{
		{"both directories clean", func(t *testing.T, home, dir string) (string, string) { return dir, dir }, false},
		{"session directory has the deny", func(t *testing.T, home, dir string) (string, string) {
			other := filepath.Join(home, "session")
			writeConfig(t, filepath.Join(other, "opencode.json"), denyCfg)
			return dir, other
		}, true},
		{"cwd has the deny", func(t *testing.T, home, dir string) (string, string) {
			other := filepath.Join(home, "cwd")
			writeConfig(t, filepath.Join(other, ".opencode", "opencode.json"), denyCfg)
			return other, dir
		}, true},
		{"relative cwd", func(t *testing.T, home, dir string) (string, string) { return "proj", dir }, true},
		{"relative session directory", func(t *testing.T, home, dir string) (string, string) { return dir, "." }, true},
		{"frontmatter with a language tag", func(t *testing.T, home, dir string) (string, string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "agent", "f.md"), "---yaml\ndescription: x\n---\n")
			return dir, dir
		}, true},
		{"frontmatter opener with extra text", func(t *testing.T, home, dir string) (string, string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "agent", "f.md"), "--- # note\ndescription: x\n---\n")
			return dir, dir
		}, true},
		{"key with a space before the colon", func(t *testing.T, home, dir string) (string, string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "agent", "f.md"), "---\npermission : {bash: deny}\n---\n")
			return dir, dir
		}, true},
		{"quoted key with a space before the colon", func(t *testing.T, home, dir string) (string, string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "agent", "f.md"), "---\n\"permissions\" :\n  bash: deny\n---\n")
			return dir, dir
		}, true},
		{"model and description frontmatter", func(t *testing.T, home, dir string) (string, string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "agent", "f.md"), "---\nmodel: x/y\ndescription: a helper\n---\nbody\n")
			return dir, dir
		}, false},
		{"opener with trailing spaces", func(t *testing.T, home, dir string) (string, string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "agent", "f.md"), "---  \ndescription: x\n---\n")
			return dir, dir
		}, false},
		{"empty cwd", func(t *testing.T, home, dir string) (string, string) { return "", dir }, true},
		{"empty session directory", func(t *testing.T, home, dir string) (string, string) { return dir, "" }, true},
		{"deny in a grandparent far above any git root", func(t *testing.T, home, dir string) (string, string) {
			deep := filepath.Join(dir, "a", "b", "c")
			if err := os.MkdirAll(deep, 0o755); err != nil {
				t.Fatal(err)
			}
			writeConfig(t, filepath.Join(dir, "opencode.json"), denyCfg)
			return deep, deep
		}, true},
		{"mode with permission", func(t *testing.T, home, dir string) (string, string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "mode", "m.md"), agentWithPermission)
			return dir, dir
		}, true},
		{"modes with permission", func(t *testing.T, home, dir string) (string, string) {
			writeConfig(t, filepath.Join(home, ".config", "opencode", "modes", "m.md"), agentWithPermission)
			return dir, dir
		}, true},
		{"mode without permission", func(t *testing.T, home, dir string) (string, string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "mode", "m.md"), "---\ndescription: x\n---\n")
			return dir, dir
		}, false},
		{"symlinked agent directory", func(t *testing.T, home, dir string) (string, string) {
			needsSymlink(t)
			target := filepath.Join(home, "elsewhere")
			writeConfig(t, filepath.Join(target, "a.md"), "---\ndescription: x\n---\n")
			if err := os.MkdirAll(filepath.Join(dir, ".opencode"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(dir, ".opencode", "agent")); err != nil {
				t.Skip(err)
			}
			return dir, dir
		}, true},
		{"symlinked mode directory", func(t *testing.T, home, dir string) (string, string) {
			needsSymlink(t)
			target := filepath.Join(home, "elsewhere")
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(dir, ".opencode"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(dir, ".opencode", "modes")); err != nil {
				t.Skip(err)
			}
			return dir, dir
		}, true},
		{"config over the 1 MiB cap", func(t *testing.T, home, dir string) (string, string) {
			pad := strings.Repeat(" ", 1<<20)
			writeConfig(t, filepath.Join(dir, "opencode.json"), pad+allowCfg)
			return dir, dir
		}, true},
		{"config exactly at the cap is read", func(t *testing.T, home, dir string) (string, string) {
			body := allowCfg + strings.Repeat(" ", (1<<20)-len(allowCfg))
			writeConfig(t, filepath.Join(dir, "opencode.json"), body)
			return dir, dir
		}, false},
		{"agent file over the cap", func(t *testing.T, home, dir string) (string, string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "agent", "big.md"), "---\n"+strings.Repeat("a: b\n", 1<<18)+"---\n")
			return dir, dir
		}, true},
		{"config path that is a directory", func(t *testing.T, home, dir string) (string, string) {
			if err := os.MkdirAll(filepath.Join(dir, "opencode.json"), 0o755); err != nil {
				t.Fatal(err)
			}
			return dir, dir
		}, true},
		{"MDM profile at the root", func(t *testing.T, home, dir string) (string, string) {
			prefs := filepath.Join(home, "prefs")
			writeConfig(t, filepath.Join(prefs, "ai.opencode.managed.plist"), "<plist/>")
			t.Cleanup(opencode.SetManagedPrefsForTest(prefs))
			return dir, dir
		}, true},
		{"MDM profile for a user", func(t *testing.T, home, dir string) (string, string) {
			prefs := filepath.Join(home, "prefs")
			writeConfig(t, filepath.Join(prefs, "someone", "ai.opencode.managed.plist"), "<plist/>")
			t.Cleanup(opencode.SetManagedPrefsForTest(prefs))
			return dir, dir
		}, true},
		{"MDM folder without an OpenCode profile", func(t *testing.T, home, dir string) (string, string) {
			prefs := filepath.Join(home, "prefs")
			writeConfig(t, filepath.Join(prefs, "other.plist"), "<plist/>")
			t.Cleanup(opencode.SetManagedPrefsForTest(prefs))
			return dir, dir
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := useHome(t)
			dir := filepath.Join(t.TempDir(), "proj")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			req := shellCall("ls", "")
			req.Directory, req.SessionDirectory = tc.setup(t, home, dir)
			if got := req.ShellBlocked("ls"); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestShellCommand_DirectoryFieldsAreBounded: an oversized or NUL-bearing
// directory makes the call malformed, so the runner allows it unchanged.
func TestShellCommand_DirectoryFieldsAreBounded(t *testing.T) {
	for name, mutate := range map[string]func(*opencode.Request){
		"cwd too long":     func(r *opencode.Request) { r.Directory = strings.Repeat("d", 4097) },
		"session too long": func(r *opencode.Request) { r.SessionDirectory = strings.Repeat("d", 4097) },
		"cwd with NUL":     func(r *opencode.Request) { r.Directory = "/tmp/a\x00b" },
		"session with NUL": func(r *opencode.Request) { r.SessionDirectory = "/tmp/a\x00b" },
	} {
		t.Run(name, func(t *testing.T) {
			req := shellCall("ls", "/tmp")
			mutate(&req)
			if cmd, isShell := req.ShellCommand(); cmd != "" || !isShell {
				t.Fatalf("cmd=%q isShell=%v, want a malformed shell call", cmd, isShell)
			}
		})
	}
}
