// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package paths

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func forceCopy(t *testing.T) {
	t.Helper()
	old := renameFn
	renameFn = func(string, string) error { return errors.New("cross-device") }
	t.Cleanup(func() { renameFn = old })
}

func skipNoChmod(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod does not restrict this user")
	}
}

func TestMigrateCopyFailureRollsBack(t *testing.T) {
	skipNoChmod(t)
	forceCopy(t)
	legacy, target := migHome(t)
	bad := filepath.Join(legacy, "sub", "x")
	if err := os.Chmod(bad, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := MigrateLegacy(false); err == nil {
		t.Fatal("expected a copy error")
	}
	for _, n := range []string{"events.jsonl", "sub"} {
		if _, err := os.Stat(filepath.Join(target, n)); err == nil {
			t.Fatalf("partial %s left in target", n)
		}
		if _, err := os.Stat(filepath.Join(legacy, n)); err != nil {
			t.Fatalf("legacy %s lost: %v", n, err)
		}
	}
	if n, _ := dirEntryCount(target); n != 0 {
		t.Fatalf("target not empty after rollback: %d", n)
	}
}

func TestMigrateCopyKeepsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	forceCopy(t)
	legacy, target := migHome(t)
	if err := os.Symlink("events.jsonl", filepath.Join(legacy, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := MigrateLegacy(false); err != nil {
		t.Fatal(err)
	}
	got, err := os.Readlink(filepath.Join(target, "link"))
	if err != nil || got != "events.jsonl" {
		t.Fatalf("symlink not recreated: %q %v", got, err)
	}
}

func TestMigrateRemoveFailureIsWarning(t *testing.T) {
	skipNoChmod(t)
	forceCopy(t)
	legacy, target := migHome(t)
	if err := os.Chmod(legacy, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(legacy, 0o700) })
	m, err := MigrateLegacy(false)
	if err != nil {
		t.Fatalf("remove failure must not be an error: %v", err)
	}
	if len(m.Warnings) == 0 || !strings.Contains(m.Describe(), "could not remove legacy") {
		t.Fatalf("no warning: %+v", m)
	}
	if _, err := os.Stat(filepath.Join(target, "events.jsonl")); err != nil {
		t.Fatal("data missing from target")
	}
}

func TestDescribeNamesCollidingEntries(t *testing.T) {
	_, target := migHome(t)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "events.jsonl"), []byte("n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := MigrateLegacy(false)
	if err != nil || !strings.Contains(m.Describe(), "present in both: events.jsonl") {
		t.Fatalf("%v %q", err, m.Describe())
	}
}
