// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package paths_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

func mode(t *testing.T, p string) os.FileMode {
	t.Helper()
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Mode().Perm()
}

func skipOnWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes")
	}
}

func TestEnsureStateDirCreatesPrivately(t *testing.T) {
	skipOnWindows(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(paths.EnvStateDir, "")
	dir, err := paths.EnsureStateDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(home, paths.DirName) || mode(t, dir) != 0o700 {
		t.Fatalf("dir %q mode %v, want 0700", dir, mode(t, dir))
	}
}

func TestHardenTightensOnlyTheProductsOwnFilesInTheDefaultDir(t *testing.T) {
	skipOnWindows(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(paths.EnvStateDir, "")
	dir := filepath.Join(home, paths.DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.Chmod(dir, 0o755)
	write := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chmod(p, 0o644)
		return p
	}
	own := write("context-optimization.json")
	log := write("events.jsonl")
	notes := write("notes.txt")                             // not ours: untouched
	backup := write("settings.json.downshift-bak-20261013") // not ours either
	outside := filepath.Join(home, "outside.json")
	os.WriteFile(outside, []byte("x"), 0o644)
	os.Chmod(outside, 0o644)
	link := filepath.Join(dir, "linked.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	if err := paths.Harden(); err != nil {
		t.Fatal(err)
	}
	if mode(t, dir) != 0o700 {
		t.Errorf("dir = %v, want 0700", mode(t, dir))
	}
	for _, p := range []string{own, log} {
		if mode(t, p) != 0o600 {
			t.Errorf("%s = %v, want 0600", filepath.Base(p), mode(t, p))
		}
	}
	for _, p := range []string{notes, backup, outside} {
		if mode(t, p) != 0o644 {
			t.Errorf("%s = %v, want it left at 0644", filepath.Base(p), mode(t, p))
		}
	}
	// Idempotent and never widens.
	os.Chmod(own, 0o400)
	if err := paths.Harden(); err != nil || mode(t, own) != 0o400 {
		t.Errorf("Harden widened or failed: %v %v", err, mode(t, own))
	}
}

// A state directory the user pointed at with DOWNSHIFT_STATE_DIR may be shared
// or be something else entirely: it is created, never chmodded.
func TestHardenLeavesAnExplicitStateDirAlone(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	os.Chmod(dir, 0o755)
	f := filepath.Join(dir, "events.jsonl")
	os.WriteFile(f, []byte("x"), 0o644)
	os.Chmod(f, 0o644)
	t.Setenv(paths.EnvStateDir, dir)
	if err := paths.Harden(); err != nil {
		t.Fatal(err)
	}
	if mode(t, dir) != 0o755 || mode(t, f) != 0o644 {
		t.Errorf("explicit dir was changed: dir %v file %v", mode(t, dir), mode(t, f))
	}
}
