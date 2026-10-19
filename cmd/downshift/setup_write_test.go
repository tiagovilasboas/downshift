// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A dotfiles-managed settings.json is a symlink. Setup and undo must write the
// real file and leave the link alone, or the next dotfiles sync drops the hooks.
func TestSetupKeepsASymlinkedSettingsFile(t *testing.T) {
	e := newSetupEnv(t, ".claude", "dot")
	real := e.path("dot/settings.json")
	if err := os.WriteFile(real, []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := e.path(".claude/settings.json")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	if code, out, _ := e.run("claude-code", "--yes"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("settings.json is no longer a symlink: %v %v", fi, err)
	}
	got, _ := os.ReadFile(real)
	if !strings.Contains(string(got), "downshift") || !strings.Contains(string(got), `"theme"`) {
		t.Fatalf("the real file was not updated in place:\n%s", got)
	}
	if fi, _ := os.Stat(real); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode of the real file = %v, want 0600 kept", fi.Mode().Perm())
	}

	if code, out, _ := e.run("--undo", "--yes"); code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("undo replaced the symlink: %v %v", fi, err)
	}
	if got, _ := os.ReadFile(real); string(got) != `{"theme":"dark"}` {
		t.Fatalf("undo did not restore the real file: %s", got)
	}
}

func TestAtomicWriteFileModeAndNoLeftovers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.json")
	if err := atomicWriteFile(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteFile(path, []byte("two"), 0o600); err != nil {
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
	if err := atomicWriteFile(path, []byte("three"), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(victim); string(b) != "keep" {
		t.Errorf("a planted symlink at the old fixed temp name was followed: %q", b)
	}
}

func TestAtomicWriteFileFailureLeavesTheOriginal(t *testing.T) {
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
	if err := atomicWriteFile(path, []byte("new"), 0o600); err == nil {
		t.Fatal("expected an error in a read-only directory")
	}
	if b, _ := os.ReadFile(path); string(b) != "orig" {
		t.Errorf("original damaged: %q", b)
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
