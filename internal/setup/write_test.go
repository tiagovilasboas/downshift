// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A dotfiles-managed file is a symlink. Setup leaves it alone: no write, no
// backup, and the link and the real file stay as they were.
func TestSetupWriteRefusesASymlinkedTarget(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dot", "settings.json")
	if err := os.MkdirAll(filepath.Dir(real), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "settings.json")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	p := Plan{Path: link, Existed: true, Before: []byte(`{"theme":"dark"}`), After: []byte(`{"new":1}`)}
	bak, err := Write(p, "20260102T000000.000000")
	if err == nil || !strings.Contains(err.Error(), link) || !strings.Contains(err.Error(), "link") {
		t.Fatalf("err = %v, want a refusal naming %s", err, link)
	}
	if bak != "" {
		t.Errorf("backup path = %q, want none", bak)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link changed: %v %v", fi, err)
	}
	if got, _ := os.ReadFile(real); string(got) != `{"theme":"dark"}` {
		t.Errorf("real file changed: %s", got)
	}
	for _, d := range []string{dir, filepath.Dir(real)} {
		entries, _ := os.ReadDir(d)
		for _, e := range entries {
			if strings.Contains(e.Name(), BackupInfix) || strings.Contains(e.Name(), ".downshift-tmp") {
				t.Errorf("leftover %s in %s", e.Name(), d)
			}
		}
	}
}

// The backup is the old file byte for byte, written with the file's mode.
func TestSetupWriteBackupHoldsTheOldContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	bak, err := Write(Plan{Path: path, Existed: true, Before: []byte("old"), After: []byte("new")}, "20260102T000000.000000")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(bak); string(got) != "old" {
		t.Errorf("backup = %q, want old", got)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Errorf("file = %q, want new", got)
	}
	// Windows reports 0666 for any writable file; the bits mean nothing there.
	if fi, _ := os.Stat(bak); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("backup mode = %v, want 0600", fi.Mode().Perm())
	}
}

// Each setup run backs a file up next to it. Undo restores only the newest
// backup, so older ones are removed and the "did not exist" marker is kept.
func TestSetupWriteKeepsOnlyTheNewestBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "register.tsx")
	marker := path + BackupInfix + "20260101T000000.000000.new"
	old := path + BackupInfix + "20260101T000000.000001"
	for _, f := range []string{marker, old} {
		if err := os.WriteFile(f, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := Plan{Path: path, Existed: true, Before: []byte("v1"), After: []byte("v2")}
	bak, err := Write(p, "20260102T000000.000000")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(bak); string(got) != "v1" {
		t.Fatalf("newest backup = %q, want v1", got)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("older backup %s still exists", filepath.Base(old))
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("did-not-exist marker removed: %v", err)
	}
	if b, isMarker := LatestBackup(path); b != bak || isMarker {
		t.Errorf("LatestBackup = %s (marker %v), want %s", b, isMarker, bak)
	}
}
