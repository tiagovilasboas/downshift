// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package paths_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

func TestResolveEnvWins(t *testing.T) {
	t.Setenv(paths.EnvStateDir, "/tmp/downshift-test-state")
	t.Setenv("HOME", t.TempDir())
	dir, src, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/tmp/downshift-test-state" || src != paths.SourceEnv {
		t.Fatalf("got %q %q", dir, src)
	}
}

func TestResolveLegacyWhenOnlyLegacyExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(paths.EnvStateDir, "")
	legacy := filepath.Join(home, paths.LegacyName)
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, src, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if dir != legacy || src != paths.SourceLegacy {
		t.Fatalf("got %q %q", dir, src)
	}
}

func TestResolveDefaultWhenNeitherExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(paths.EnvStateDir, "")
	want := filepath.Join(home, paths.DirName)
	dir, src, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if dir != want || src != paths.SourceDefault {
		t.Fatalf("got %q %q", dir, src)
	}
}

func TestResolvePrefersNewDirWhenBothExist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(paths.EnvStateDir, "")
	newDir := filepath.Join(home, paths.DirName)
	legacy := filepath.Join(home, paths.LegacyName)
	_ = os.MkdirAll(newDir, 0o755)
	_ = os.MkdirAll(legacy, 0o755)
	dir, src, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if dir != newDir || src != paths.SourceDefault {
		t.Fatalf("got %q %q", dir, src)
	}
}
