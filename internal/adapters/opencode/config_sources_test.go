// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode_test

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	denyCfg  = `{"permission":{"bash":"deny"}}`
	allowCfg = `{"permission":{"bash":"allow"}}`
)

func TestShellBlocked_EnvSources(t *testing.T) {
	cases := []struct {
		name, env, value string
		want             bool
	}{
		{"content restrictive", "OPENCODE_CONFIG_CONTENT", denyCfg, true},
		{"content allow-only", "OPENCODE_CONFIG_CONTENT", allowCfg, false},
		{"content unparsable", "OPENCODE_CONFIG_CONTENT", `{"permission":`, true},
		{"permission restrictive", "OPENCODE_PERMISSION", `{"bash":"ask"}`, true},
		{"permission allow-only", "OPENCODE_PERMISSION", `{"bash":"allow"}`, false},
		{"permission unparsable", "OPENCODE_PERMISSION", `{"bash":`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useHome(t)
			t.Setenv(tc.env, tc.value)
			if got := shellCall("ls", t.TempDir()).ShellBlocked("ls"); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShellBlocked_FileSources(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, home, dir string)
		want  bool
	}{
		{"OPENCODE_CONFIG deny", func(t *testing.T, home, dir string) {
			p := filepath.Join(home, "elsewhere", "custom.json")
			writeConfig(t, p, denyCfg)
			t.Setenv("OPENCODE_CONFIG", p)
		}, true},
		{"OPENCODE_CONFIG allow", func(t *testing.T, home, dir string) {
			p := filepath.Join(home, "elsewhere", "custom.json")
			writeConfig(t, p, allowCfg)
			t.Setenv("OPENCODE_CONFIG", p)
		}, false},
		{"OPENCODE_CONFIG missing is skipped", func(t *testing.T, home, dir string) {
			t.Setenv("OPENCODE_CONFIG", filepath.Join(home, "nope.json"))
		}, false},
		{"OPENCODE_CONFIG_DIR deny", func(t *testing.T, home, dir string) {
			d := filepath.Join(home, "cfgdir")
			writeConfig(t, filepath.Join(d, "opencode.json"), denyCfg)
			t.Setenv("OPENCODE_CONFIG_DIR", d)
		}, true},
		{"XDG_CONFIG_HOME deny", func(t *testing.T, home, dir string) {
			x := filepath.Join(home, "xdg")
			writeConfig(t, filepath.Join(x, "opencode", "opencode.jsonc"), denyCfg)
			t.Setenv("XDG_CONFIG_HOME", x)
		}, true},
		{"managed dir deny", func(t *testing.T, home, dir string) {
			writeConfig(t, filepath.Join(home, "managed", "opencode.json"), denyCfg)
		}, true},
		{"managed dir agent with permission", func(t *testing.T, home, dir string) {
			writeConfig(t, filepath.Join(home, "managed", "agents", "a.md"), "---\npermission:\n  bash: deny\n---\n")
		}, true},
		{"ancestor config below git root", func(t *testing.T, home, dir string) {
			root := filepath.Dir(dir)
			writeConfig(t, filepath.Join(root, "opencode.json"), denyCfg)
			writeConfig(t, filepath.Join(root, ".git"), "gitdir: x")
		}, true},
		{"ancestor .opencode below git root", func(t *testing.T, home, dir string) {
			root := filepath.Dir(dir)
			writeConfig(t, filepath.Join(root, ".opencode", "opencode.json"), denyCfg)
			writeConfig(t, filepath.Join(root, ".git", "HEAD"), "x")
		}, true},
		{"config above the git root is read", func(t *testing.T, home, dir string) {
			root := filepath.Dir(dir)
			writeConfig(t, filepath.Join(filepath.Dir(root), "opencode.json"), denyCfg)
			writeConfig(t, filepath.Join(root, ".git"), "gitdir: x")
		}, true},
		{"ancestor config with no git root", func(t *testing.T, home, dir string) {
			writeConfig(t, filepath.Join(filepath.Dir(dir), "opencode.json"), denyCfg)
		}, true},
		{"project agent with permission", func(t *testing.T, home, dir string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "agent", "x.md"), "---\ndescription: x\npermission:\n  bash: ask\n---\nbody\n")
		}, true},
		{"project agents dir with permission", func(t *testing.T, home, dir string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "agents", "x.md"), "---\n\"permission\":\n  bash: ask\n---\n")
		}, true},
		{"global agent with permission", func(t *testing.T, home, dir string) {
			writeConfig(t, filepath.Join(home, ".config", "opencode", "agent", "x.md"), "---\npermission: {bash: deny}\n---\n")
		}, true},
		{"agent without permission", func(t *testing.T, home, dir string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "agent", "x.md"), "---\ndescription: x\n---\npermission: in body\n")
		}, false},
		{"agent with nested permission-like key only", func(t *testing.T, home, dir string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "agent", "x.md"), "---\ntools:\n  permission: x\n---\n")
		}, false},
		{"agent without frontmatter", func(t *testing.T, home, dir string) {
			writeConfig(t, filepath.Join(dir, ".opencode", "agent", "x.md"), "just text\n")
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := useHome(t)
			dir := filepath.Join(t.TempDir(), "proj")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			tc.setup(t, home, dir)
			if got := shellCall("ls", dir).ShellBlocked("ls"); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
