// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLinkedComponent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	home := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(home, "real", "sub"), 0o755))
	must(os.MkdirAll(filepath.Join(home, "dotfiles", "opencode"), 0o755))
	must(os.Symlink(filepath.Join(home, "dotfiles", "opencode"), filepath.Join(home, "linked")))
	must(os.WriteFile(filepath.Join(home, "real", "f"), nil, 0o600))
	must(os.Symlink(filepath.Join(home, "real", "f"), filepath.Join(home, "real", "leaf")))
	must(os.Symlink(filepath.Join(home, "dotfiles", "opencode"), filepath.Join(home, "..cache")))
	cases := []struct {
		name, path, want string
	}{
		{"real chain", filepath.Join("real", "sub", "x"), ""},
		{"existing real file", filepath.Join("real", "f"), ""},
		{"missing tail stops the walk", filepath.Join("real", "new", "deep", "x"), ""},
		{"linked parent", filepath.Join("linked", "x"), "linked"},
		{"linked parent, missing below", filepath.Join("linked", "a", "b", "x"), "linked"},
		{"linked leaf", filepath.Join("real", "leaf"), filepath.Join("real", "leaf")},
		{"the home itself", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.want
			if want != "" {
				want = filepath.Join(home, want)
			}
			if got := linkedComponent(home, filepath.Join(home, tc.path)); got != want {
				t.Fatalf("linkedComponent = %q, want %q", got, want)
			}
		})
	}
	t.Run("a dot-dot named child is inside home", func(t *testing.T) {
		path := filepath.Join(home, "..cache", "x")
		if !withinHome(home, path) {
			t.Fatal("withinHome(..cache/x) = false")
		}
		if err := ErrLinkedPath(path, home); err == nil {
			t.Fatal("ErrLinkedPath did not refuse a linked ..cache parent")
		}
		if withinHome(home, filepath.Dir(home)) {
			t.Fatal("withinHome(parent of home) = true")
		}
	})
	t.Run("a linked home is not checked", func(t *testing.T) {
		alias := filepath.Join(t.TempDir(), "home")
		must(os.Symlink(home, alias))
		if got := linkedComponent(alias, filepath.Join(alias, "real", "f")); got != "" {
			t.Fatalf("linked home refused: %q", got)
		}
	})
	t.Run("outside home is walked from the root", func(t *testing.T) {
		other, err := filepath.EvalSymlinks(t.TempDir()) // macOS: /var is itself a link
		must(err)
		must(os.Symlink(home, filepath.Join(other, "l")))
		if got := linkedComponent(filepath.Join(other, "nohome"), filepath.Join(other, "l", "x")); got != filepath.Join(other, "l") {
			t.Fatalf("linkedComponent = %q", got)
		}
	})
}
