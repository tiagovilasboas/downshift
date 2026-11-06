// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package configfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteAtomicModeAndNoLeftovers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.json")
	if err := WriteAtomic(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600 (no 0644 window)", fi.Mode().Perm())
	}
	if b, _ := os.ReadFile(path); string(b) != "two" {
		t.Errorf("content = %q", b)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("leftover files in %s: %v", dir, entries)
	}
	// A pre-existing predictable temp name is neither used nor followed.
	victim := filepath.Join(dir, "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, path+".downshift-tmp"); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("three"), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(victim); string(b) != "keep" {
		t.Errorf("a planted symlink at the old fixed temp name was followed: %q", b)
	}
}

func TestWriteAtomicFailureLeavesTheOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.json")
	if err := os.WriteFile(path, []byte("orig"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil { // cannot create the temp file
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o700)
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	if err := WriteAtomic(path, []byte("new"), 0o600); err == nil {
		t.Fatal("expected an error in a read-only directory")
	}
	if b, _ := os.ReadFile(path); string(b) != "orig" {
		t.Errorf("original damaged: %q", b)
	}
}

func TestRemoveEmptyDirsStopsAtALink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	root := t.TempDir()
	path := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }
	for _, d := range []string{"real/a", "target"} {
		if err := os.MkdirAll(path(d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(path("target"), path("real/a/link")); err != nil {
		t.Fatal(err)
	}
	RemoveEmptyDirs(path("real/a/link"), 3)
	if _, err := os.Lstat(path("real/a/link")); err != nil {
		t.Fatalf("the link was unlinked: %v", err)
	}
	if _, err := os.Stat(path("target")); err != nil {
		t.Fatal(err)
	}
	RemoveEmptyDirs(path("real/a"), 2)
	if _, err := os.Lstat(path("real/a")); err != nil {
		t.Fatal("a directory holding a link was removed")
	}
}
